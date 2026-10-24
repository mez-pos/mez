package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/mez-pos/mez/internal/relay/wire"
)

const (
	// agentPath is where the relay takes venues, under the configured URL.
	agentPath = "/agent"

	// keyPrefix is how a venue key starts. Checked so that a pairing code or
	// a password pasted into relay_key fails at startup, with a person
	// watching, and not as a 401 every five minutes for ever.
	keyPrefix = "mzr_"

	// keyScopeRemote is the third of four parts in a key the relay issued
	// for a venue's back office as well as its guests (ADR-306). The word is
	// the relay's; internal/relay is not imported for it, as it is not for
	// keyPrefix, so the venue's binary carries none of the relay's server.
	keyScopeRemote = "remote"

	dialTimeout = 15 * time.Second

	// sendTimeout bounds one write to the relay. A socket that cannot take a
	// frame in this long is dead whatever TCP still believes.
	sendTimeout = 10 * time.Second

	// A connection that lasted this long was a working one, and the next
	// failure starts the backoff again from the bottom. A shorter one was
	// part of the same outage.
	stableAfter = 30 * time.Second

	defaultMinBackoff = time.Second
	defaultMaxBackoff = time.Minute

	// defaultLongBackoff is the wait after the relay says something that
	// retrying will not change: the key is refused, or another machine holds
	// this venue's place.
	defaultLongBackoff = 5 * time.Minute

	// defaultEntitledBackoff is the longest wait after the relay says it is
	// not carrying this venue (ADR-316). The key is good and the answer can
	// change at any moment, when a payment lands or a file is edited, so it
	// is asked again within the minute and never left for the five a
	// refused key waits. Half of this is the shortest: sixty venues turned
	// off together do not all come back in the same second.
	defaultEntitledBackoff = time.Minute

	defaultHeartbeat = 20 * time.Second
	heartbeatTimeout = 10 * time.Second

	// defaultHealthEvery is how often the venue's health is sent to a relay
	// that asked for it whether or not it changed, and defaultHealthPoll
	// how often it is read to see whether it has (ADR-361). The cloud's
	// alerts have an hour of patience, so a change is news within the
	// minute and a summary unchanged for five says the venue is still
	// there.
	defaultHealthEvery = 5 * time.Minute
	defaultHealthPoll  = time.Minute
	healthTimeout      = 10 * time.Second
)

// State is where the tunnel is. It is a vocabulary the back office shows.
type State string

// The states.
const (
	StateConnecting State = "connecting" // dialling
	StateConnected  State = "connected"  // guests can reach this venue
	StateWaiting    State = "waiting"    // the last attempt failed; another is coming
	StateRefused    State = "refused"    // the relay does not accept this venue's key
	// StateNotEntitled: the key is good and the relay is not carrying this
	// venue: its date has passed, or it was never given one (ADR-316).
	StateNotEntitled State = "not_entitled"
	StateReplaced    State = "replaced" // another machine connected with the same key
	StateStopped     State = "stopped"  // closed, or never started
)

// Status is the tunnel as of now.
type Status struct {
	State State
	// Since is when it entered this state.
	Since time.Time
	// LastError is why the last attempt ended, or empty.
	LastError string
	// RetryAt is when the next attempt is due. Zero unless waiting, refused,
	// not entitled or replaced.
	RetryAt time.Time
	// Standing is where the relay last said this venue stands with it. Nil
	// when it has said nothing: a relay from before there was anything to
	// say, or one not yet reached.
	Standing *Standing
}

// Standing is where a venue stands with its relay, as the relay tells it
// (ADR-313, ADR-316). The words are the relay's; nothing here acts on them.
type Standing struct {
	// State is "trial", "active", "grace" and the like from a relay that
	// keeps accounts, and empty from one that keeps a file.
	State string
	// PaidUntil is the end of what has been paid for. Zero when nobody is
	// counting.
	PaidUntil time.Time
	// Until is when the relay stops carrying the venue. Zero with no date
	// set, and when it is not carried at all.
	Until time.Time
	// BackOffice says the relay carries the back office for this venue.
	BackOffice bool
	// Reason is the relay's short code for why.
	Reason string
	// LastReport is when the venue's last close-of-day message went to its
	// owner, and MessagesURL where they change what is sent (ADR-364).
	// Zero and empty when the relay said nothing of either.
	LastReport  time.Time
	MessagesURL string
}

func standingOf(e wire.Entitlement) *Standing {
	s := &Standing{State: e.State, BackOffice: e.Remote, Reason: e.Reason, MessagesURL: e.MessagesURL}
	if e.LastReport != nil {
		s.LastReport = e.LastReport.UTC()
	}
	if e.PaidUntil != nil {
		s.PaidUntil = e.PaidUntil.UTC()
	}
	if e.Until != nil {
		s.Until = e.Until.UTC()
	}
	return s
}

// standingFrom reads what the relay said on its answer to the dial. Nothing
// said, or something unreadable, is nothing known.
func standingFrom(resp *http.Response) *Standing {
	if resp == nil {
		return nil
	}
	raw := resp.Header.Get(wire.HeaderEntitlement)
	if raw == "" {
		return nil
	}
	var e wire.Entitlement
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return nil
	}
	return standingOf(e)
}

// Options configures a Client.
type Options struct {
	// RelayURL is the relay, as relay_url gives it: ws, wss, http or https.
	RelayURL string

	// Key is this venue's key. It is sent to the relay and written nowhere.
	Key string

	// Handler is the server. Guests reach Mount on it and nothing else.
	Handler http.Handler

	// Mount is where the guest app lives on Handler, "/order/".
	Mount string

	// BackOffice is the venue's remote_backoffice setting: the back office
	// and its API are carried too (ADR-306). It needs a key the relay
	// issued for it, and New refuses one that was not: the relay would
	// answer 404 for every page, and the owner would believe it was on.
	BackOffice bool

	// Log is required.
	Log *slog.Logger

	// Now is the clock. Nil means time.Now.
	Now func() time.Time

	// Health, when set, is what the venue says of its own health (ADR-361),
	// read on connecting and every HealthPoll after, and sent to a relay
	// that said on the dial that it reads it: at once, when it has changed,
	// and every HealthEvery whether or not it has. A relay that did not
	// ask is sent nothing and Health is not called. Nil sends nothing.
	Health func(ctx context.Context) (wire.Health, error)

	// The rest are for tests; zero means the default.
	MinBackoff      time.Duration
	MaxBackoff      time.Duration
	LongBackoff     time.Duration
	EntitledBackoff time.Duration
	Heartbeat       time.Duration
	HealthEvery     time.Duration
	HealthPoll      time.Duration
}

// Client holds the tunnel open. It owns its goroutines: Start launches
// them, Close stops them and waits.
type Client struct {
	url     string
	key     string
	handler http.Handler
	mount   string
	// backOffice is Options.BackOffice. It never changes after New.
	backOffice bool
	log        *slog.Logger
	now        func() time.Time
	health     func(ctx context.Context) (wire.Health, error)

	minBackoff, maxBackoff, longBackoff, entitledBackoff, heartbeat time.Duration
	healthEvery, healthPoll                                         time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	status  Status
	ws      *websocket.Conn
	started bool
	// standing is what the relay last said of this venue. It outlives a
	// state: it is still what is known while the line is down.
	standing *Standing
}

// New checks the options and builds a client that is not yet running.
func New(opts Options) (*Client, error) {
	if opts.Handler == nil || opts.Log == nil {
		return nil, errors.New("tunnel: Options.Handler and Options.Log are required")
	}
	if !strings.HasPrefix(opts.Mount, "/") || !strings.HasSuffix(opts.Mount, "/") || len(opts.Mount) < 3 {
		return nil, fmt.Errorf("tunnel: mount %q must be a directory like /order/, and not the root", opts.Mount)
	}
	if opts.Key == "" {
		return nil, errors.New("tunnel: relay_url is set but relay_key is not; the relay's operator issues one")
	}
	if !strings.HasPrefix(opts.Key, keyPrefix) {
		// Not the key itself: this message goes to a log.
		return nil, fmt.Errorf("tunnel: relay_key does not look like a venue key, which starts %q", keyPrefix)
	}
	if opts.BackOffice && !keyAllowsBackOffice(opts.Key) {
		// Not the key itself: this message goes to a log.
		return nil, errors.New("tunnel: remote_backoffice is on, but relay_key is a guests-only key; the relay's operator issues one for the back office with `relay issue-key VENUE --remote`")
	}
	target, err := agentURL(opts.RelayURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		url:             target,
		key:             opts.Key,
		handler:         opts.Handler,
		mount:           opts.Mount,
		backOffice:      opts.BackOffice,
		log:             opts.Log,
		now:             opts.Now,
		health:          opts.Health,
		minBackoff:      orDefault(opts.MinBackoff, defaultMinBackoff),
		maxBackoff:      orDefault(opts.MaxBackoff, defaultMaxBackoff),
		longBackoff:     orDefault(opts.LongBackoff, defaultLongBackoff),
		entitledBackoff: orDefault(opts.EntitledBackoff, defaultEntitledBackoff),
		heartbeat:       orDefault(opts.Heartbeat, defaultHeartbeat),
		healthEvery:     orDefault(opts.HealthEvery, defaultHealthEvery),
		healthPoll:      orDefault(opts.HealthPoll, defaultHealthPoll),
	}
	if c.now == nil {
		c.now = time.Now
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.status = Status{State: StateStopped, Since: c.now()}
	return c, nil
}

// BackOffice reports whether this client carries the back office as well as
// the guest app. It is the setting, not the state of the line.
func (c *Client) BackOffice() bool { return c.backOffice }

// keyAllowsBackOffice reads the scope out of a venue key:
// "mzr_<venue>.<key id>.remote.<mac>". It is what the key says, not a check
// of it: only the relay, which holds the secret, can tell a real key from a
// written one, and it refuses the second when the venue dials.
func keyAllowsBackOffice(key string) bool {
	parts := strings.Split(strings.TrimPrefix(key, keyPrefix), ".")
	return len(parts) == 4 && parts[2] == keyScopeRemote
}

func orDefault(d, fallback time.Duration) time.Duration {
	if d <= 0 {
		return fallback
	}
	return d
}

// agentURL is the address to dial: the relay's URL with the agent path on
// the end of whatever path it already has.
//
// The venue's key is sent on the dial, so the dial must be TLS. Plain ws
// and http are accepted only for a relay on this machine; the config
// package refuses the same thing earlier, and this check stands on its own
// so that a Client built any other way cannot leak the key either.
func agentURL(relayURL string) (string, error) {
	u, err := url.Parse(relayURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("tunnel: relay url %q is not a url with a host", relayURL)
	}
	switch u.Scheme {
	case "wss", "https":
	case "ws", "http":
		if !loopbackHost(u.Hostname()) {
			return "", fmt.Errorf("tunnel: relay url %q: the venue's key travels on this connection, so the scheme must be wss or https", relayURL)
		}
	default:
		return "", fmt.Errorf("tunnel: relay url %q: scheme must be wss or https", relayURL)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + agentPath
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

// loopbackHost reports whether a hostname can only be this machine.
func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Start begins connecting, and returns at once. Nothing waits on the tunnel:
// a venue with no internet starts exactly as fast as one with.
func (c *Client) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started || c.ctx.Err() != nil {
		return
	}
	c.started = true
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run()
	}()
}

// Close disconnects and waits for the client's goroutines, or for ctx.
func (c *Client) Close(ctx context.Context) error {
	c.cancel()
	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws != nil {
		// A goodbye the relay can tell from a dropped line. It unblocks the
		// read, which is how the session ends. On a line that is half dead
		// the handshake waits up to ten seconds for a frame that never
		// comes, which is the whole shutdown; past ctx the socket is torn
		// down instead, and the relay sees a drop, as it would have anyway.
		closed := make(chan struct{})
		go func() {
			defer close(closed)
			_ = ws.Close(websocket.StatusNormalClosure, "venue shutting down")
		}()
		select {
		case <-closed:
		case <-ctx.Done():
			_ = ws.CloseNow()
		}
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	// Stopped either way: Close was called, and whatever is still winding
	// down is no longer serving anyone.
	defer c.setStatus(Status{State: StateStopped})
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tunnel: closing: %w", ctx.Err())
	}
}

// Status is the tunnel as of now.
func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.status
	if c.standing != nil {
		standing := *c.standing
		st.Standing = &standing
	}
	return st
}

// setStanding keeps what the relay said of this venue. Nil forgets it: the
// relay now reached says nothing.
func (c *Client) setStanding(s *Standing) {
	c.mu.Lock()
	c.standing = s
	c.mu.Unlock()
}

func (c *Client) setStatus(s Status) {
	s.Since = c.now()
	c.mu.Lock()
	c.status = s
	c.mu.Unlock()
}

// run connects, serves, and connects again, until Close.
func (c *Client) run() {
	failures := 0
	for c.ctx.Err() == nil {
		c.setStatus(Status{State: StateConnecting})
		connectedFor, err := c.session()
		if c.ctx.Err() != nil {
			return
		}

		state, wait := StateWaiting, time.Duration(0)
		switch {
		case errors.Is(err, errRefused), websocket.CloseStatus(err) == wire.CloseRevoked:
			// Refused at the dial, or turned off while connected (ADR-359),
			// whose dial back would be refused: either way it waits long.
			state, wait = StateRefused, c.longBackoff
		case notEntitled(err):
			// Not counted as a failure of the line, which it is not: the
			// next real one starts its backoff from the bottom.
			state = StateNotEntitled
			wait = c.entitledBackoff/2 + time.Duration(float64(c.entitledBackoff/2)*rand.Float64())
		case websocket.CloseStatus(err) == wire.CloseReplaced:
			state, wait = StateReplaced, c.longBackoff
		default:
			if connectedFor >= stableAfter {
				failures = 0
			}
			wait = backoff(failures, c.minBackoff, c.maxBackoff, rand.Float64())
			failures++
		}

		// A venue with no internet is the usual state, not news (ADR-372):
		// the first failure after a connection is a warning, the redials
		// after it are information, so the event log is not a line a
		// minute for every offline day.
		level := slog.LevelInfo
		if failures <= 1 || state != StateWaiting {
			level = slog.LevelWarn
		}
		c.log.Log(c.ctx, level, "tunnel: not connected",
			"state", string(state), "error", err.Error(), "retry_in_ms", wait.Milliseconds())
		c.setStatus(Status{State: state, LastError: err.Error(), RetryAt: c.now().Add(wait)})

		select {
		case <-c.ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// backoff is how long to wait after the nth failure in a row: doubling from
// low to high, then anywhere in the upper half of that, by jitter in [0,1).
// The spread is what stops every venue on a relay that restarted from
// arriving back in the same second.
func backoff(failures int, low, high time.Duration, jitter float64) time.Duration {
	d := high
	// Past 30 doublings the shift would overflow, and it is long past high.
	if failures < 30 {
		d = min(low<<failures, high)
	}
	if d <= 0 {
		d = high
	}
	half := d / 2
	return half + time.Duration(float64(d-half)*jitter)
}

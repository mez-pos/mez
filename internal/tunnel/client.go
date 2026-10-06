package tunnel

import (
	"context"
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

	"github.com/byjoellobo/mez/internal/relay/wire"
)

const (
	// agentPath is where the relay takes venues, under the configured URL.
	agentPath = "/agent"

	// keyPrefix is how a venue key starts. Checked so that a pairing code or
	// a password pasted into relay_key fails at startup, with a person
	// watching, and not as a 401 every five minutes for ever.
	keyPrefix = "mzr_"

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

	defaultHeartbeat = 20 * time.Second
	heartbeatTimeout = 10 * time.Second
)

// State is where the tunnel is. It is a vocabulary the back office shows.
type State string

// The states.
const (
	StateConnecting State = "connecting" // dialling
	StateConnected  State = "connected"  // guests can reach this venue
	StateWaiting    State = "waiting"    // the last attempt failed; another is coming
	StateRefused    State = "refused"    // the relay does not accept this venue's key
	StateReplaced   State = "replaced"   // another machine connected with the same key
	StateStopped    State = "stopped"    // closed, or never started
)

// Status is the tunnel as of now.
type Status struct {
	State State
	// Since is when it entered this state.
	Since time.Time
	// LastError is why the last attempt ended, or empty.
	LastError string
	// RetryAt is when the next attempt is due. Zero unless waiting, refused
	// or replaced.
	RetryAt time.Time
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

	// Log is required.
	Log *slog.Logger

	// Now is the clock. Nil means time.Now.
	Now func() time.Time

	// The rest are for tests; zero means the default.
	MinBackoff  time.Duration
	MaxBackoff  time.Duration
	LongBackoff time.Duration
	Heartbeat   time.Duration
}

// Client holds the tunnel open. It owns its goroutines: Start launches
// them, Close stops them and waits.
type Client struct {
	url     string
	key     string
	handler http.Handler
	mount   string
	log     *slog.Logger
	now     func() time.Time

	minBackoff, maxBackoff, longBackoff, heartbeat time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	status  Status
	ws      *websocket.Conn
	started bool
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
	target, err := agentURL(opts.RelayURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		url:         target,
		key:         opts.Key,
		handler:     opts.Handler,
		mount:       opts.Mount,
		log:         opts.Log,
		now:         opts.Now,
		minBackoff:  orDefault(opts.MinBackoff, defaultMinBackoff),
		maxBackoff:  orDefault(opts.MaxBackoff, defaultMaxBackoff),
		longBackoff: orDefault(opts.LongBackoff, defaultLongBackoff),
		heartbeat:   orDefault(opts.Heartbeat, defaultHeartbeat),
	}
	if c.now == nil {
		c.now = time.Now
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.status = Status{State: StateStopped, Since: c.now()}
	return c, nil
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
		// read, which is how the session ends.
		_ = ws.Close(websocket.StatusNormalClosure, "venue shutting down")
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
	return c.status
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
		case errors.Is(err, errRefused):
			state, wait = StateRefused, c.longBackoff
		case websocket.CloseStatus(err) == wire.CloseReplaced:
			state, wait = StateReplaced, c.longBackoff
		default:
			if connectedFor >= stableAfter {
				failures = 0
			}
			wait = backoff(failures, c.minBackoff, c.maxBackoff, rand.Float64())
			failures++
		}

		c.log.Warn("tunnel: not connected",
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

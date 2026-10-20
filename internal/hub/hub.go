package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/db"
)

// Subprotocol is the WebSocket subprotocol a client must ask for. A browser
// cannot set headers on a WebSocket, so this is also where its device token
// travels: the second protocol offered is "token.<device token>".
const Subprotocol = "mez.v1"

// The defaults, all overridable for a test.
const (
	// defaultPingEvery is how often a quiet connection is pinged. A kitchen
	// tablet's wifi dies silently; without this the socket looks open for
	// minutes while tickets pile up unseen.
	defaultPingEvery = 20 * time.Second

	// defaultWriteTimeout bounds one write. A device that has stopped
	// reading is dropped rather than allowed to hold the hub.
	defaultWriteTimeout = 10 * time.Second

	// defaultBatch is how many events one read of the stream takes.
	defaultBatch = 200

	// readLimit bounds a client's messages. Everything a client sends is a
	// short control frame.
	readLimit = 4 << 10

	// maxConnsPerDevice is how many streams one device may hold open. A
	// tablet has one screen, and a reconnecting one leaves its dead socket
	// behind for a moment, so a handful is plenty; a token that is being
	// used to open hundreds is being used by something other than a
	// register, and is refused at the door rather than allowed to hold the
	// hub.
	maxConnsPerDevice = 8
)

// errTooManyConns is add refusing a device that already holds
// maxConnsPerDevice streams.
var errTooManyConns = errors.New("hub: too many connections for this device")

// Topics a client can subscribe to. Everything is published to the venue
// topic as well, so a client that wants the lot subscribes to that alone.
const (
	TopicVenue    = "venue"
	prefixOrder   = "order:"
	prefixTable   = "table:"
	prefixStation = "station:"
)

// Source is where the hub reads events, and the versions it announces.
// *db.DB implements it.
type Source interface {
	VenueEventsAfter(ctx context.Context, venueID, afterID int64, limit int) ([]db.StreamEvent, error)
	VenueLastEventID(ctx context.Context, venueID int64) (int64, error)
	MenuVersion(ctx context.Context, venueID int64) (int64, error)
	PrintVersion(ctx context.Context, venueID int64) (int64, error)
	FloorVersion(ctx context.Context, venueID int64) (int64, error)
}

// Client is who is connecting, as authentication established them.
type Client struct {
	VenueID  int64
	DeviceID int64
	Name     string
	Kind     auth.DeviceKind
}

// Options configure a Hub. Source is required.
type Options struct {
	Source Source
	Log    *slog.Logger

	// Origins are browser origins allowed to connect besides the one the
	// server is served from. Empty is the shipped configuration.
	Origins []string

	PingEvery    time.Duration
	WriteTimeout time.Duration
	Batch        int
}

// Hub fans a venue's events out to its devices.
//
// Every connection reads the stream from SQLite for itself, starting at the
// position the client says it has reached. A publish is only a nudge: it
// carries no events, it wakes the connections of that venue, and they read
// what is there. Replay and live streaming are therefore the same code, and
// a device that was off for an hour catches up through the same path as one
// that missed a second.
//
// That also settles backpressure. A client that reads slowly falls behind in
// the database rather than in memory, and the only thing the hub holds for
// it is one boolean saying there is more to read.
type Hub struct {
	source       Source
	log          *slog.Logger
	origins      []string
	pingEvery    time.Duration
	writeTimeout time.Duration
	batch        int

	mu      sync.Mutex
	venues  map[int64]map[*conn]struct{}
	closed  bool
	closing chan struct{}
}

// New returns a Hub.
func New(opts Options) (*Hub, error) {
	if opts.Source == nil {
		return nil, errors.New("hub: no source")
	}
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if opts.PingEvery <= 0 {
		opts.PingEvery = defaultPingEvery
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = defaultWriteTimeout
	}
	if opts.Batch <= 0 {
		opts.Batch = defaultBatch
	}
	return &Hub{
		source: opts.Source, log: opts.Log, origins: opts.Origins, pingEvery: opts.PingEvery,
		writeTimeout: opts.WriteTimeout, batch: opts.Batch,
		venues: map[int64]map[*conn]struct{}{}, closing: make(chan struct{}),
	}, nil
}

// Notify tells the hub that a venue has new events, or a new menu. It never blocks and
// never fails: the events are already on disk, and a connection that misses
// the nudge catches up on its next ping or its next reconnect.
func (h *Hub) Notify(venueID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.venues[venueID] {
		c.wake()
	}
}

// Disconnect cuts every connection a device holds. Revoking a device stops
// its requests at once, since each one is authenticated afresh; a stream
// is authenticated when it opens and would otherwise go on carrying every
// order, payment and refund of the venue to a device the owner has just
// turned away. The device may dial again, and is refused at the door.
func (h *Hub) Disconnect(venueID, deviceID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.venues[venueID] {
		if c.client.DeviceID == deviceID {
			c.cut()
		}
	}
}

// Connections is how many devices are connected, for the health endpoint
// and for tests.
func (h *Hub) Connections() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, conns := range h.venues {
		n += len(conns)
	}
	return n
}

// Close drops every connection. It is the shutdown path, and is safe to
// call twice.
func (h *Hub) Close(context.Context) error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	close(h.closing)
	h.mu.Unlock()
	return nil
}

func (h *Hub) add(c *conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("hub: closed")
	}
	held := 0
	for other := range h.venues[c.client.VenueID] {
		if other.client.DeviceID == c.client.DeviceID {
			held++
		}
	}
	if held >= maxConnsPerDevice {
		return fmt.Errorf("%w: device %d holds %d", errTooManyConns, c.client.DeviceID, held)
	}
	if h.venues[c.client.VenueID] == nil {
		h.venues[c.client.VenueID] = map[*conn]struct{}{}
	}
	h.venues[c.client.VenueID][c] = struct{}{}
	return nil
}

func (h *Hub) remove(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns := h.venues[c.client.VenueID]
	delete(conns, c)
	if len(conns) == 0 {
		delete(h.venues, c.client.VenueID)
	}
}

// Serve upgrades a request to a WebSocket and streams to it until the
// client goes away, the server shuts down, or the connection stops
// answering.
//
// The caller has already authenticated the device; this trusts Client.
func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, client Client) {
	// The default origin check is what is wanted: a browser page from
	// another origin is refused, and a client that sends no Origin at all,
	// which is every script and every native app, is allowed. Origins is
	// the configured dev origin, if there is one, for the same reason CORS
	// has one. See ADR-023.
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:   []string{Subprotocol},
		OriginPatterns: h.origins,
	})
	if err != nil {
		// Accept has already answered the client.
		h.log.Debug("websocket handshake failed", "error", err, "device_id", client.DeviceID)
		return
	}
	ws.SetReadLimit(readLimit)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	c := &conn{hub: h, ws: ws, client: client, cut: cancel, woken: make(chan struct{}, 1), topics: map[string]struct{}{TopicVenue: {}}}
	if err := h.add(c); err != nil {
		if errors.Is(err, errTooManyConns) {
			h.log.Warn("device refused: too many connections", "venue_id", client.VenueID, "device_id", client.DeviceID, "error", err)
			_ = ws.Close(websocket.StatusPolicyViolation, "too many connections for this device")
			return
		}
		_ = ws.Close(websocket.StatusGoingAway, "server is shutting down")
		return
	}
	go func() {
		select {
		case <-h.closing:
			cancel()
		case <-ctx.Done():
		}
	}()

	h.log.Info("device connected", "venue_id", client.VenueID, "device_id", client.DeviceID, "device_kind", client.Kind)
	err = c.run(ctx)
	// Out of the venue map before the close handshake, not after it: Close
	// waits up to five seconds for the peer's close frame, and a revoked or
	// stalled device must not count as connected for that long.
	h.remove(c)
	status, reason := websocket.StatusNormalClosure, ""
	if err != nil && !isGone(err) {
		h.log.Warn("device disconnected", "venue_id", client.VenueID, "device_id", client.DeviceID, "error", err)
		status, reason = websocket.StatusInternalError, "stream failed"
	}
	_ = ws.Close(status, reason)
}

// isGone reports whether an error is just the other end leaving.
func isGone(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	switch websocket.CloseStatus(err) {
	case websocket.StatusNormalClosure, websocket.StatusGoingAway, websocket.StatusNoStatusRcvd, websocket.StatusAbnormalClosure:
		return true
	}
	return false
}

// writeJSON sends one message, bounded by the write timeout.
func (h *Hub) writeJSON(ctx context.Context, ws *websocket.Conn, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("hub: encoding a message: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, h.writeTimeout)
	defer cancel()
	return ws.Write(ctx, websocket.MessageText, raw)
}

package relay

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

const (
	// maxStreams is how many guest requests one venue may have in flight.
	// Past it a cafe is not busy, something is holding requests open.
	maxStreams = 256

	// streamBuffer is how many response frames wait for one guest. A phone
	// that falls further behind than this loses its request, so that one
	// slow download can never stop the socket being read for everyone else.
	streamBuffer = 32

	// sendTimeout bounds one write to a venue. A socket that cannot take a
	// frame in this long is dead whatever TCP still believes.
	sendTimeout = 10 * time.Second

	// pingEvery and pingTimeout find a venue whose line dropped without a
	// close, so its guests are told it is offline at once and not after
	// each of them has waited out a request.
	pingEvery   = 20 * time.Second
	pingTimeout = 10 * time.Second
)

// Why a stream could not be opened or did not finish.
var (
	errAgentGone = errors.New("relay: the venue disconnected")
	errBusy      = errors.New("relay: the venue has too many requests in flight")
)

// agent is one venue's open socket and the guest requests riding on it.
type agent struct {
	venueID string
	keyID   string
	// keyRemote says the key allows the back office across the relay
	// (ADR-306), and remote that the relay carries it now: the key's word
	// and the entitlement's together (ADR-316). The second can change while
	// the venue stays connected.
	keyRemote bool
	remote    atomic.Bool
	// notices says the venue reads Notice frames.
	notices bool
	ws      *websocket.Conn
	bucket  *bucket
	log     *slog.Logger

	// ctx ends when the socket does. Every guest handler on this agent
	// watches it.
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	streams map[uint64]*stream
	nextID  uint64

	closeOnce sync.Once
}

// stream is the relay's half of one guest request.
type stream struct {
	id uint64

	// frames is what the venue has answered so far, in order.
	frames chan wire.Frame

	// overrun is closed when the guest fell too far behind and the stream
	// was cut.
	overrun chan struct{}
}

func newAgent(key Key, ent Entitlement, notices bool, ws *websocket.Conn, lim Limit, now time.Time, log *slog.Logger) *agent {
	ctx, cancel := context.WithCancel(context.Background())
	a := &agent{
		venueID:   key.VenueID,
		keyID:     key.KeyID,
		keyRemote: key.Remote,
		notices:   notices,
		ws:        ws,
		bucket:    newBucket(lim, now),
		log:       log.With("venue_id", key.VenueID, "key_id", key.KeyID),
		ctx:       ctx,
		cancel:    cancel,
		streams:   map[uint64]*stream{},
	}
	a.remote.Store(key.Remote && ent.Remote)
	return a
}

// tell sends the venue where it stands, if it reads such a thing.
func (a *agent) tell(e Entitlement) {
	if !a.notices {
		return
	}
	msg, err := wire.EncodeNotice(e.told(a.keyRemote && e.Remote))
	if err != nil {
		return
	}
	_ = a.send(msg)
}

// open starts a stream for one guest request.
func (a *agent) open() (*stream, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx.Err() != nil {
		return nil, errAgentGone
	}
	if len(a.streams) >= maxStreams {
		return nil, errBusy
	}
	a.nextID++
	st := &stream{
		id:      a.nextID,
		frames:  make(chan wire.Frame, streamBuffer),
		overrun: make(chan struct{}),
	}
	a.streams[st.id] = st
	return st, nil
}

// forget drops a stream. Frames that still arrive for it are discarded.
func (a *agent) forget(id uint64) {
	a.mu.Lock()
	delete(a.streams, id)
	a.mu.Unlock()
}

// inFlight is how many guest requests are open on this venue.
func (a *agent) inFlight() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.streams)
}

// send writes one encoded frame to the venue. A failed write ends the agent:
// a frame lost from the middle of a stream cannot be made up.
func (a *agent) send(msg []byte) error {
	ctx, cancel := context.WithTimeout(a.ctx, sendTimeout)
	defer cancel()
	if err := a.ws.Write(ctx, websocket.MessageBinary, msg); err != nil {
		a.close(websocket.StatusGoingAway, "write failed")
		return errAgentGone
	}
	return nil
}

// reset tells the venue to stop working on a stream nobody is waiting for.
func (a *agent) reset(id uint64, why string) {
	msg, err := wire.Encode(wire.Frame{Type: wire.TypeReset, Stream: id, Payload: []byte(why)})
	if err != nil {
		return
	}
	// The venue being gone is the one way this fails, and then there is
	// nothing left to tell.
	_ = a.send(msg)
}

// read hands the venue's frames to the guests waiting on them until the
// socket ends, and returns why it did.
func (a *agent) read() error {
	for {
		// Not a.ctx: cancelling a read tears the socket down on the spot,
		// and close has a reason to deliver first.
		typ, msg, err := a.ws.Read(context.Background())
		if err != nil {
			return err
		}
		if typ != websocket.MessageBinary {
			a.close(wire.CloseProtocol, "frames are binary messages")
			return wire.ErrMalformed
		}
		f, err := wire.Decode(msg)
		if err != nil || f.Type == wire.TypeRequest {
			// A venue does not open streams. One that tries is not a
			// tunnel client.
			a.close(wire.CloseProtocol, "unreadable frame")
			return wire.ErrMalformed
		}
		a.deliver(f)
	}
}

// deliver queues a frame for its guest without ever waiting on one.
func (a *agent) deliver(f wire.Frame) {
	a.mu.Lock()
	st := a.streams[f.Stream]
	last := f.Type == wire.TypeEnd || f.Type == wire.TypeReset
	if st != nil && last {
		delete(a.streams, f.Stream)
	}
	a.mu.Unlock()

	if st == nil {
		// The guest left, or timed out, and the venue had not heard yet.
		return
	}
	select {
	case st.frames <- f:
	default:
		a.forget(st.id)
		close(st.overrun)
		if !last {
			a.reset(st.id, "guest too slow")
		}
	}
}

// ping probes the socket until it ends.
func (a *agent) ping() {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(a.ctx, pingTimeout)
			err := a.ws.Ping(ctx)
			cancel()
			if err != nil {
				a.close(websocket.StatusGoingAway, "no answer to ping")
				return
			}
		}
	}
}

// close ends the socket and with it every stream on it. It is safe to call
// from anywhere, any number of times; the first reason wins.
func (a *agent) close(code websocket.StatusCode, reason string) {
	a.closeOnce.Do(func() {
		a.cancel()
		// Close waits a few seconds for the other end to agree. A venue
		// that is already gone will not, which is fine either way.
		_ = a.ws.Close(code, reason)
	})
}

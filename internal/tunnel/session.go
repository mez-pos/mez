package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/mez/internal/relay/wire"
)

const (
	// maxStreams and maxRequestBody are the relay's own limits, held here
	// too: this end must not depend on the other being well behaved.
	maxStreams     = 256
	maxRequestBody = 1 << 20
)

// errRefused is the relay turning the venue's key away.
var errRefused = errors.New("the relay refused this venue's key")

// inbound is a guest request still arriving.
type inbound struct {
	head wire.RequestHead
	body bytes.Buffer
}

// session is one connection to the relay and the requests on it.
type session struct {
	c  *Client
	ws *websocket.Conn

	// ctx ends with the connection. Every request served on it has a
	// context under this one.
	ctx    context.Context
	cancel context.CancelFunc

	// pending is touched only by the reading goroutine.
	pending map[uint64]*inbound

	mu     sync.Mutex
	active map[uint64]context.CancelFunc
}

// session dials the relay and serves guests until the connection ends. It
// returns how long it was connected, and why it stopped; the error is never
// nil.
func (c *Client) session() (time.Duration, error) {
	dialCtx, cancelDial := context.WithTimeout(c.ctx, dialTimeout)
	defer cancelDial()

	ws, resp, err := websocket.Dial(dialCtx, c.url, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + c.key}},
		HTTPClient: &http.Client{CheckRedirect: refuseRedirect},
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return 0, errRefused
		}
		return 0, fmt.Errorf("dialling the relay: %w", err)
	}
	ws.SetReadLimit(wire.MaxFrame)

	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()
	}()
	if c.ctx.Err() != nil {
		// Close ran while this was dialling, and found no socket to close.
		_ = ws.Close(websocket.StatusNormalClosure, "venue shutting down")
		return 0, c.ctx.Err()
	}

	ctx, cancel := context.WithCancel(c.ctx)
	s := &session{
		c: c, ws: ws, ctx: ctx, cancel: cancel,
		pending: map[uint64]*inbound{},
		active:  map[uint64]context.CancelFunc{},
	}

	connected := c.now()
	c.setStatus(Status{State: StateConnected})
	c.log.Info("tunnel: connected")

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		s.heartbeat()
	}()

	err = s.read()
	// Whatever was being served has nobody to answer now.
	cancel()
	_ = ws.CloseNow()
	return c.now().Sub(connected), err
}

// read takes frames off the socket until it ends, and returns why.
func (s *session) read() error {
	for {
		// Not s.ctx: cancelling a read tears the socket down before Close
		// has said goodbye. The socket closing is what ends this.
		kind, msg, err := s.ws.Read(context.Background())
		if err != nil {
			return err
		}
		if kind != websocket.MessageBinary {
			// The wire is binary frames and nothing else, as the relay
			// holds the venue to; this end holds the relay to the same.
			return fmt.Errorf("%w: the relay sent a text message", wire.ErrMalformed)
		}
		f, err := wire.Decode(msg)
		if err != nil {
			return err
		}
		switch f.Type {
		case wire.TypeRequest:
			s.begin(f)
		case wire.TypeBody:
			s.more(f)
		case wire.TypeEnd:
			s.dispatch(f.Stream)
		case wire.TypeReset:
			s.abandon(f.Stream)
		default:
			return fmt.Errorf("%w: the relay sent a frame only a venue sends", wire.ErrMalformed)
		}
	}
}

// begin notes a new request.
func (s *session) begin(f wire.Frame) {
	head, err := wire.DecodeRequest(f)
	if err != nil {
		s.reset(f.Stream, "unreadable request")
		return
	}
	s.mu.Lock()
	inFlight := len(s.active)
	_, serving := s.active[f.Stream]
	s.mu.Unlock()
	if serving || s.pending[f.Stream] != nil {
		// The relay numbers streams and never reuses one on a connection.
		// A repeat would otherwise replace a request half arrived, or
		// leave two goroutines sharing one cancel. Refused, and the
		// earlier one goes on.
		s.reset(f.Stream, "stream id in use")
		return
	}
	if len(s.pending)+inFlight >= maxStreams {
		s.reset(f.Stream, "too many requests")
		return
	}
	s.pending[f.Stream] = &inbound{head: head}
}

// more adds a piece of a request's body.
func (s *session) more(f wire.Frame) {
	in := s.pending[f.Stream]
	if in == nil {
		return
	}
	if in.body.Len()+len(f.Payload) > maxRequestBody {
		delete(s.pending, f.Stream)
		s.reset(f.Stream, "request too large")
		return
	}
	in.body.Write(f.Payload)
}

// dispatch serves a request that has fully arrived, on a goroutine of its
// own so that a slow page never holds up the socket.
func (s *session) dispatch(stream uint64) {
	in := s.pending[stream]
	if in == nil {
		return
	}
	delete(s.pending, stream)

	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.active[stream] = cancel
	s.mu.Unlock()

	s.c.wg.Add(1)
	go func() {
		defer s.c.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.active, stream)
			s.mu.Unlock()
			cancel()
		}()
		s.serve(ctx, stream, in)
	}()
}

// abandon drops a request the relay no longer wants answered: the guest
// left, or gave up waiting.
func (s *session) abandon(stream uint64) {
	delete(s.pending, stream)
	s.mu.Lock()
	cancel := s.active[stream]
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// serve runs the server's handler for one guest request and sends back
// what it writes.
func (s *session) serve(ctx context.Context, stream uint64, in *inbound) {
	w := &responseWriter{s: s, ctx: ctx, stream: stream, header: http.Header{}}

	// The server recovers its own panics. This is for the one it does not,
	// so that a guest's request can never take down the process that is
	// also the counter.
	defer func() {
		if p := recover(); p != nil {
			if p != http.ErrAbortHandler {
				s.c.log.Error("tunnel: a guest request panicked", "panic", fmt.Sprint(p), "path", in.head.Path)
			}
			s.reset(stream, "handler failed")
		}
	}()

	req, err := guestRequest(ctx, in.head, in.body.Bytes(), s.c.mount)
	if err != nil {
		// In the server's own error envelope, so a guest app reading the
		// answer sees the shape it expects everywhere else.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"no such page"}}`))
		w.finish()
		return
	}
	s.c.handler.ServeHTTP(w, req)
	w.finish()
}

// send writes one encoded frame. A failed write ends the connection: a
// frame lost from the middle of a response cannot be made up.
func (s *session) send(msg []byte) error {
	ctx, cancel := context.WithTimeout(s.ctx, sendTimeout)
	defer cancel()
	if err := s.ws.Write(ctx, websocket.MessageBinary, msg); err != nil {
		_ = s.ws.CloseNow()
		return err
	}
	return nil
}

func (s *session) reset(stream uint64, why string) {
	msg, err := wire.Encode(wire.Frame{Type: wire.TypeReset, Stream: stream, Payload: []byte(why)})
	if err != nil {
		return
	}
	// If this fails the connection is gone, and so is the need to say it.
	_ = s.send(msg)
}

// heartbeat probes the relay, so a line that died without a close is
// noticed from this end and redialled, not left looking connected.
func (s *session) heartbeat() {
	t := time.NewTicker(s.c.heartbeat)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(s.ctx, heartbeatTimeout)
			err := s.ws.Ping(ctx)
			cancel()
			if err != nil {
				_ = s.ws.CloseNow()
				return
			}
		}
	}
}

// responseWriter turns what a handler writes into frames.
type responseWriter struct {
	s      *session
	ctx    context.Context
	stream uint64
	header http.Header

	wroteHeader bool
	// failed means the response cannot be completed and the stream has
	// been, or need not be, reset.
	failed bool
}

func (w *responseWriter) Header() http.Header { return w.header }

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader || w.failed {
		return
	}
	if status < 200 {
		// An interim response. The frames have one head, the final one.
		return
	}
	w.wroteHeader = true
	msg, err := wire.EncodeHead(wire.TypeResponse, w.stream, wire.ResponseHead{Status: status, Header: w.header.Clone()})
	if err != nil {
		// Headers too large for a frame.
		w.failed = true
		w.s.reset(w.stream, "response head too large")
		return
	}
	if w.s.send(msg) != nil {
		w.failed = true
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	w.WriteHeader(http.StatusOK)
	written := 0
	for len(p) > 0 {
		if w.failed {
			return written, errors.New("tunnel: the response was abandoned")
		}
		if err := w.ctx.Err(); err != nil {
			return written, err
		}
		n := min(len(p), wire.MaxChunk)
		msg, err := wire.Encode(wire.Frame{Type: wire.TypeBody, Stream: w.stream, Payload: p[:n]})
		if err != nil {
			return written, err
		}
		if err := w.s.send(msg); err != nil {
			w.failed = true
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

// Flush is here so a handler streaming events sees a flusher. Every write
// is already a frame on its way.
func (w *responseWriter) Flush() {}

// finish closes the venue's half of the stream.
func (w *responseWriter) finish() {
	w.WriteHeader(http.StatusOK)
	if w.failed || w.ctx.Err() != nil {
		return
	}
	msg, err := wire.Encode(wire.Frame{Type: wire.TypeEnd, Stream: w.stream})
	if err == nil {
		_ = w.s.send(msg)
	}
}

// refuseRedirect stops the dial following a redirect. The venue's key has
// already gone out with the first request; following to another scheme or
// host would send it again to wherever the answer pointed, and a relay
// reached only through a redirect is a misconfigured relay_url, not a
// relay. The dial fails and the log says where it was pointed.
func refuseRedirect(req *http.Request, _ []*http.Request) error {
	return fmt.Errorf("the relay answered with a redirect to %s; relay_url must be the relay's own address", req.URL.Redacted())
}

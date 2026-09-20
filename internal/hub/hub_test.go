package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

var at = time.Date(2026, 9, 18, 4, 30, 0, 0, time.UTC)

// source is a stream in memory, standing in for the database. The hub's
// contract with it is small enough that a real one would only make these
// tests slower and less deliberate; internal/db tests the query itself.
type source struct {
	mu     sync.Mutex
	events []db.StreamEvent
	fail   error
}

func (s *source) VenueEventsAfter(_ context.Context, venueID, afterID int64, limit int) ([]db.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return nil, s.fail
	}
	var out []db.StreamEvent
	for _, e := range s.events {
		if e.ID > afterID && len(out) < limit {
			out = append(out, e)
		}
	}
	_ = venueID
	return out, nil
}

func (s *source) VenueLastEventID(context.Context, int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail != nil {
		return 0, s.fail
	}
	if len(s.events) == 0 {
		return 0, nil
	}
	return s.events[len(s.events)-1].ID, nil
}

// append adds an event to the stream and returns its id.
func (s *source) append(orderID string, table int64, payload order.Payload) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := int64(len(s.events) + 1)
	s.events = append(s.events, db.StreamEvent{
		ID: id, OrderID: 1, OrderPublicID: orderID, OrderStatus: "open", TableID: table,
		Event: order.NewEvent(id, at, 7, 3, payload),
	})
	return id
}

func (s *source) breaks(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = err
}

func line(station string) order.LineAdded {
	return order.LineAdded{
		ItemID: 1, Name: "Flat white", Qty: 1, Station: station,
		UnitPrice: money.Amount{Minor: 22000, Currency: "INR"},
	}
}

// world is a hub behind an HTTP server, with a stream to feed it.
type world struct {
	hub    *Hub
	src    *source
	server *httptest.Server
}

func newWorld(t *testing.T, opts ...func(*Options)) *world {
	t.Helper()
	src := &source{}
	o := Options{Source: src, PingEvery: 50 * time.Millisecond, WriteTimeout: 2 * time.Second, Batch: 3}
	for _, fn := range opts {
		fn(&o)
	}
	h, err := New(o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.Serve(w, r, Client{VenueID: 1, DeviceID: 3, Name: "Kitchen", Kind: "kitchen"})
	}))
	t.Cleanup(func() {
		_ = h.Close(context.Background())
		srv.Close()
	})
	return &world{hub: h, src: src, server: srv}
}

// client is a connected device in a test.
type client struct {
	ws *websocket.Conn
	t  *testing.T
}

func (w *world) dial(t *testing.T) *client {
	t.Helper()
	ws, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(w.server.URL, "http")+"/api/v1/stream",
		&websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	return &client{ws: ws, t: t}
}

func (c *client) send(v any) {
	c.t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		c.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.ws.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

// next is the next message, as a map so a test can read any of them.
func (c *client) next() map[string]any {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := c.ws.Read(ctx)
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.t.Fatalf("decoding %s: %v", raw, err)
	}
	return msg
}

// hello sends the opening message and returns the welcome.
func (c *client) hello(msg clientMessage) map[string]any {
	c.t.Helper()
	msg.Type = msgHello
	c.send(msg)
	welcome := c.next()
	if welcome["type"] != msgWelcome {
		c.t.Fatalf("first message back = %v", welcome)
	}
	return welcome
}

// eventIDs collects event ids until a caught_up arrives.
func (c *client) eventIDs() []int64 {
	c.t.Helper()
	var ids []int64
	for {
		msg := c.next()
		switch msg["type"] {
		case msgEvent:
			ids = append(ids, int64(msg["id"].(float64)))
		case msgCaughtUp:
			return ids
		default:
			c.t.Fatalf("unexpected message %v", msg)
		}
	}
}

// TestAReconnectLosesNothing is the phase's test: a device is cut off mid
// stream, comes back with the last id it saw, and is given the gap.
func TestAReconnectLosesNothing(t *testing.T) {
	w := newWorld(t)
	for i := range 5 {
		w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: fmt.Sprint("before ", i)})
	}

	first := w.dial(t)
	welcome := first.hello(clientMessage{FromStart: true})
	if welcome["from_event_id"] != float64(0) || welcome["latest_event_id"] != float64(5) {
		t.Fatalf("welcome = %v", welcome)
	}
	got := first.eventIDs()
	// The batch is three, so the replay arrives in two goes.
	if len(got) != 3 {
		t.Fatalf("first batch = %v", got)
	}
	got = append(got, first.eventIDs()...)
	if len(got) != 5 || got[4] != 5 {
		t.Fatalf("replayed %v, want 1 to 5", got)
	}

	// The kitchen tablet's wifi dies. Three more events happen while it is
	// gone, and one more arrives as it is coming back.
	_ = first.ws.CloseNow()
	for i := range 3 {
		w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: fmt.Sprint("during ", i)})
	}
	waitFor(t, func() bool { return w.hub.Connections() == 0 })

	second := w.dial(t)
	welcome = second.hello(clientMessage{LastEventID: 5})
	if welcome["from_event_id"] != float64(5) {
		t.Fatalf("welcome = %v", welcome)
	}
	if got := second.eventIDs(); len(got) != 3 || got[0] != 6 || got[2] != 8 {
		t.Fatalf("after reconnecting: %v, want 6 to 8", got)
	}

	// And it is live again.
	w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: "after"})
	w.hub.Notify(1)
	if got := second.eventIDs(); len(got) != 1 || got[0] != 9 {
		t.Fatalf("live: %v", got)
	}
}

func TestAFreshScreenStartsAtNow(t *testing.T) {
	w := newWorld(t)
	for range 4 {
		w.src.append("ord_AAAA1111", 0, order.NoteAdded{Scope: order.ScopeOrder, Text: "old"})
	}

	c := w.dial(t)
	welcome := c.hello(clientMessage{})
	if welcome["from_event_id"] != float64(4) {
		t.Fatalf("a fresh screen was given history: %v", welcome)
	}

	w.src.append("ord_AAAA1111", 0, order.NoteAdded{Scope: order.ScopeOrder, Text: "new"})
	w.hub.Notify(1)
	if got := c.eventIDs(); len(got) != 1 || got[0] != 5 {
		t.Fatalf("live = %v, want only the new event", got)
	}
}

// TestAClientAheadOfTheStreamStartsAgain covers a restored backup: the
// device remembers further than the database goes.
func TestAClientAheadOfTheStreamStartsAgain(t *testing.T) {
	w := newWorld(t)
	w.src.append("ord_AAAA1111", 0, order.NoteAdded{Scope: order.ScopeOrder, Text: "only"})

	c := w.dial(t)
	welcome := c.hello(clientMessage{LastEventID: 900})
	if welcome["from_event_id"] != float64(0) {
		t.Fatalf("welcome = %v", welcome)
	}
	if got := c.eventIDs(); len(got) != 1 {
		t.Fatalf("replay = %v, want the whole stream", got)
	}
}

func TestTopicsDecideWhatIsSent(t *testing.T) {
	w := newWorld(t)
	c := w.dial(t)
	c.hello(clientMessage{Topics: []string{prefixStation + "bar"}})

	// A bar line and a bar ticket are the station's; the kitchen's are not,
	// and neither is a note on the same order.
	w.src.append("ord_AAAA1111", 4, line("kitchen"))
	w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: "hi"})
	wanted := w.src.append("ord_AAAA1111", 4, line("bar"))
	w.hub.Notify(1)
	if got := c.eventIDs(); len(got) != 1 || got[0] != wanted {
		t.Fatalf("station topic gave %v, want only %d", got, wanted)
	}

	// Subscribing again replaces the topics, and a table topic follows the
	// order rather than the station.
	c.send(clientMessage{Type: msgSubscribe, Topics: []string{prefixTable + "4"}})
	if welcome := c.next(); welcome["type"] != msgWelcome {
		t.Fatalf("subscribe was not acknowledged: %v", welcome)
	}
	onFour := w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: "table four"})
	w.src.append("ord_BBBB2222", 9, order.NoteAdded{Scope: order.ScopeOrder, Text: "table nine"})
	w.hub.Notify(1)
	if got := c.eventIDs(); len(got) != 1 || got[0] != onFour {
		t.Fatalf("table topic gave %v, want only %d", got, onFour)
	}

	// An order topic, and then the venue topic, which is everything.
	c.send(clientMessage{Type: msgSubscribe, Topics: []string{prefixOrder + "ord_BBBB2222"}})
	c.next()
	w.src.append("ord_AAAA1111", 4, order.NoteAdded{Scope: order.ScopeOrder, Text: "not mine"})
	mine := w.src.append("ord_BBBB2222", 9, order.NoteAdded{Scope: order.ScopeOrder, Text: "mine"})
	w.hub.Notify(1)
	if got := c.eventIDs(); len(got) != 1 || got[0] != mine {
		t.Fatalf("order topic gave %v, want only %d", got, mine)
	}
}

func TestTheProtocolRefusesWhatItDoesNotUnderstand(t *testing.T) {
	t.Run("a first message that is not a hello", func(t *testing.T) {
		w := newWorld(t)
		c := w.dial(t)
		c.send(clientMessage{Type: msgSubscribe})
		if msg := c.next(); msg["type"] != msgErrorType || msg["code"] != "expected_hello" {
			t.Fatalf("message = %v", msg)
		}
		// The hub closes it, which the client learns on its next read. That
		// read is also what completes the close handshake.
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if _, _, err := c.ws.Read(ctx); err == nil {
			t.Fatal("the connection survived a first message that was not a hello")
		}
		waitFor(t, func() bool { return w.hub.Connections() == 0 })
	})

	t.Run("a message that is not JSON", func(t *testing.T) {
		w := newWorld(t)
		c := w.dial(t)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := c.ws.Write(ctx, websocket.MessageText, []byte("not json")); err != nil {
			t.Fatal(err)
		}
		if msg := c.next(); msg["code"] != "invalid_message" {
			t.Fatalf("message = %v", msg)
		}
	})

	t.Run("a message this build does not know", func(t *testing.T) {
		w := newWorld(t)
		c := w.dial(t)
		c.hello(clientMessage{})
		c.send(map[string]string{"type": "loyalty.redeem"})
		if msg := c.next(); msg["code"] != "unknown_message" {
			t.Fatalf("message = %v", msg)
		}
	})
}

// TestASourceThatFailsClosesTheConnection: the database has gone away, so
// the socket ends rather than sitting there silently out of date.
func TestASourceThatFailsClosesTheConnection(t *testing.T) {
	w := newWorld(t)
	c := w.dial(t)
	c.hello(clientMessage{})

	w.src.breaks(errors.New("disk I/O error"))
	w.src.append("ord_AAAA1111", 0, order.NoteAdded{Scope: order.ScopeOrder, Text: "never seen"})
	w.hub.Notify(1)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, _, err := c.ws.Read(ctx); err == nil {
		t.Fatal("the connection survived the source failing")
	}
	waitFor(t, func() bool { return w.hub.Connections() == 0 })
}

func TestShutdownDropsEveryone(t *testing.T) {
	w := newWorld(t)
	first, second := w.dial(t), w.dial(t)
	first.hello(clientMessage{})
	second.hello(clientMessage{})
	if w.hub.Connections() != 2 {
		t.Fatalf("%d connections, want 2", w.hub.Connections())
	}

	if err := w.hub.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitFor(t, func() bool { return w.hub.Connections() == 0 })
	if err := w.hub.Close(context.Background()); err != nil {
		t.Errorf("closing twice: %v", err)
	}

	// And a device connecting afterwards is turned away.
	ws, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(w.server.URL, "http"),
		&websocket.DialOptions{Subprotocols: []string{Subprotocol}})
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, _, err := ws.Read(ctx); err == nil {
			t.Error("a device connected to a closed hub")
		}
		_ = ws.CloseNow()
	}
}

func TestHeartbeat(t *testing.T) {
	w := newWorld(t)
	c := w.dial(t)
	c.hello(clientMessage{})

	// coder/websocket answers pings underneath, so a connection that is
	// still here after several ping intervals is one the heartbeat has
	// been passing.
	time.Sleep(250 * time.Millisecond)
	if w.hub.Connections() != 1 {
		t.Fatalf("the heartbeat closed a healthy connection")
	}
	w.src.append("ord_AAAA1111", 0, order.NoteAdded{Scope: order.ScopeOrder, Text: "still working"})
	w.hub.Notify(1)
	if got := c.eventIDs(); len(got) != 1 {
		t.Errorf("after the heartbeat: %v", got)
	}
}

func TestNewRefusesNoSource(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Error("New with no source succeeded")
	}
}

// waitFor polls until cond is true, or fails the test.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting")
}

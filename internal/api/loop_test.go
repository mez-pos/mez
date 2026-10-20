package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/mez-pos/mez/internal/auth"
	"github.com/mez-pos/mez/internal/hub"
	"github.com/mez-pos/mez/internal/service/floor"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// The dine-in loop across devices, on the real stack (phase 08, Task 5,
// ADR-092): the database, the order service publishing to the hub, the
// API and the stream behind one HTTP server, and two real WebSocket
// clients, a till and a kitchen display. What a browser does on each is
// what these clients do: say hello with the last id seen, and read.

// within is how fast a change must reach the other device.
const within = time.Second

// streamed is one message off a socket.
type streamed struct {
	Type    string          `json:"type"`
	ID      int64           `json:"id"`
	OrderID string          `json:"order_id"`
	Station string          `json:"station"`
	Event   json.RawMessage `json:"event"`
}

func (m streamed) eventType() string {
	var e struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(m.Event, &e)
	return e.Type
}

// socket is a device on the stream, reading into a channel.
type socket struct {
	t      *testing.T
	conn   *websocket.Conn
	cancel context.CancelFunc
	in     chan streamed
	mu     sync.Mutex
	seen   map[int64]int // event id -> times delivered, across the socket's life
	last   int64
	// history is every event this socket has read, in order.
	history []streamed
}

func dialStream(t *testing.T, base, token string, hello hub.ClientMessage, seen map[int64]int) *socket {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	url := "ws" + strings.TrimPrefix(base, "http") + "/api/v1/stream"
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader:   http.Header{"Authorization": {"Bearer " + token}},
		Subprotocols: []string{"mez.v1"},
	})
	if err != nil {
		cancel()
		t.Fatalf("dialling the stream: %v", err)
	}
	s := &socket{t: t, conn: conn, cancel: cancel, in: make(chan streamed, 256), seen: seen}
	hello.Type = hub.MsgHello
	b, _ := json.Marshal(hello)
	if err := conn.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("hello: %v", err)
	}
	go func() {
		defer close(s.in)
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var m streamed
			if json.Unmarshal(data, &m) == nil {
				s.in <- m
			}
		}
	}()
	t.Cleanup(s.close)
	return s
}

// close is the display unplugged: the socket goes, with no goodbye.
func (s *socket) close() {
	s.cancel()
	_ = s.conn.CloseNow()
}

// next is the next message within the time given, or false.
func (s *socket) next(wait time.Duration) (streamed, bool) {
	select {
	case m, ok := <-s.in:
		if ok && m.Type == string(hub.MsgEvent) {
			s.mu.Lock()
			s.seen[m.ID]++
			if m.ID > s.last {
				s.last = m.ID
			}
			s.history = append(s.history, m)
			s.mu.Unlock()
		}
		return m, ok
	case <-time.After(wait):
		return streamed{}, false
	}
}

// welcomed reads the server's welcome: the socket is subscribed.
func (s *socket) welcomed() {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for {
		m, ok := s.next(time.Until(deadline))
		if !ok {
			s.t.Fatal("no welcome")
		}
		if m.Type == string(hub.MsgWelcome) {
			return
		}
	}
}

// nextOf reads until a message of the type, which need not be an event.
func (s *socket) nextOf(typ string, wait time.Duration) (streamed, bool) {
	deadline := time.Now().Add(wait)
	for {
		m, ok := s.next(time.Until(deadline))
		if !ok || m.Type == typ {
			return m, ok
		}
	}
}

// drain reads whatever is still coming, until the socket is quiet.
func (s *socket) drain() {
	for {
		if _, ok := s.next(200 * time.Millisecond); !ok {
			return
		}
	}
}

// await reads until an event of the type arrives, and fails if it does not
// within the time given. It returns the events read on the way, the one
// awaited last.
func (s *socket) await(typ string, wait time.Duration) []streamed {
	s.t.Helper()
	deadline := time.Now().Add(wait)
	var got []streamed
	for {
		m, ok := s.next(time.Until(deadline))
		if !ok {
			s.t.Fatalf("no %s within %v; had %v", typ, wait, types(got))
		}
		if m.Type != string(hub.MsgEvent) {
			continue
		}
		got = append(got, m)
		if m.eventType() == typ {
			return got
		}
	}
}

// caughtUp reads the replay through to caught_up, and returns its events.
func (s *socket) caughtUp(wait time.Duration) []streamed {
	s.t.Helper()
	deadline := time.Now().Add(wait)
	var got []streamed
	for {
		m, ok := s.next(time.Until(deadline))
		if !ok {
			s.t.Fatalf("no caught_up within %v; had %v", wait, types(got))
		}
		switch m.Type {
		case string(hub.MsgEvent):
			got = append(got, m)
		case string(hub.MsgCaughtUp):
			return got
		}
	}
}

func types(ms []streamed) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = fmt.Sprintf("%d:%s@%s", m.ID, m.eventType(), m.Station)
	}
	return out
}

// TestTheDineInLoopAcrossDevices: fired on the till, on the kitchen display
// within a second; bumped there, on the till within a second. Then the
// display is unplugged mid-service, the till carries on, and the display
// reconnects from the last id it saw: it gets exactly what it missed, in
// order, nothing lost and nothing twice, and the live stream after.
func TestTheDineInLoopAcrossDevices(t *testing.T) {
	r := newRegister(t)
	events, err := hub.New(hub.Options{Source: r.db, PingEvery: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = events.Close(context.Background()) })
	orders, err := svcorder.New(svcorder.Options{DB: r.db, Publisher: svcorder.PublisherFunc(func(_ context.Context, c svcorder.Change) {
		events.Notify(c.VenueID)
	})})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := floor.New(floor.Options{DB: r.db, Orders: orders, OnChange: events.Notify})
	if err != nil {
		t.Fatal(err)
	}
	r.rebuild(t, WithOrders(orders), WithStream(events), WithFloor(plan))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { r.handler.ServeHTTP(w, req) }))
	t.Cleanup(server.Close)

	code, err := r.devices.CreatePairingCode(context.Background(), auth.NewCode{VenueID: 1, DeviceName: "Pass", DeviceKind: auth.KindKitchen})
	if err != nil {
		t.Fatal(err)
	}
	kdsToken := pairOK(t, r.handler, code.Code).Token
	kdsHeaders := bearerOf(kdsToken)

	tillSeen, kdsSeen := map[int64]int{}, map[int64]int{}
	till := dialStream(t, server.URL, r.deviceToken, hub.ClientMessage{Topics: []string{hub.TopicVenue}}, tillSeen)
	kds := dialStream(t, server.URL, kdsToken, hub.ClientMessage{Topics: []string{"station:kitchen"}}, kdsSeen)
	till.welcomed()
	kds.welcomed()

	// Seated at table 4, a dosa for the kitchen and a coffee for the bar,
	// and Send: the kitchen's ticket is on the display within a second.
	id := r.openOrder(t)
	lineIn := func(name, station string, course int) {
		r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", fmt.Sprintf(
			`{"item_id":1,"name":%q,"qty":1,"unit_price":{"minor":10000,"currency":"INR"},"station":%q,"course":%d,"tax_rate":5000,"hsn":"996331"}`, name, station, course))
	}
	line := func(name, station string) { lineIn(name, station, 1) }
	line("Masala dosa", "kitchen")
	line("Filter coffee", "bar")
	sentAt := time.Now()
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/send", ``)
	onKDS := kds.await("ticket.sent", within)
	t.Logf("the ticket reached the display in %v", time.Since(sentAt))
	for _, m := range onKDS {
		if m.Station != "kitchen" {
			t.Errorf("the display was sent %s for %q", m.eventType(), m.Station)
		}
	}
	var sent struct {
		Seq     int64 `json:"seq"`
		Payload struct {
			LineIDs []int64 `json:"line_ids"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(onKDS[len(onKDS)-1].Event, &sent); err != nil || len(sent.Payload.LineIDs) != 1 {
		t.Fatalf("the kitchen's ticket: %s (%v)", onKDS[len(onKDS)-1].Event, err)
	}

	// Bumped on the display: on the till within a second.
	bumpAt := time.Now()
	bump := func(seq int64, verb string) {
		t.Helper()
		rec := send(r.handler, http.MethodPost, fmt.Sprintf("/api/v1/kitchen/orders/%s/tickets/%d/%s", id, seq, verb), "", kdsHeaders)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", verb, rec.Code, rec.Body)
		}
	}
	bump(sent.Seq, "bump")
	till.await("ticket.bumped", within)
	t.Logf("the bump reached the till in %v", time.Since(bumpAt))
	kds.await("ticket.bumped", within) // its own bump comes back on its stream too

	// The display is unplugged. Service goes on: a second round is sent,
	// the first ticket is recalled from the till's side of the pass (by the
	// display's API, as a second display would), and the bar is sent.
	kds.close()
	lastSeen := kds.last
	line("Upma", "kitchen")
	line("Lime soda", "bar")
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/send", ``)
	bump(sent.Seq, "recall")
	till.await("ticket.recalled", within)
	till.drain()
	missed := []streamed{}
	for eventID, n := range tillSeen {
		if n != 1 {
			t.Errorf("the till had event %d %d times", eventID, n)
		}
	}
	// What the display missed is what the till saw at the kitchen after
	// the display's last id.
	for _, m := range till.history {
		if m.ID > lastSeen && m.Station == "kitchen" {
			missed = append(missed, m)
		}
	}
	if len(missed) < 3 {
		t.Fatalf("the display should have missed a line, a ticket and a recall; the till saw %v", types(missed))
	}

	// Plugged back in, it says the last id it saw.
	kds = dialStream(t, server.URL, kdsToken, hub.ClientMessage{LastEventID: lastSeen, Topics: []string{"station:kitchen"}}, kdsSeen)
	replayed := kds.caughtUp(within)
	if got, want := types(replayed), types(missed); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the replay was\n  %v\nwant\n  %v", got, want)
	}
	// And the live stream carries on from there, once.
	bump(sent.Seq, "bump")
	kds.await("ticket.bumped", within)
	for eventID, n := range kdsSeen {
		if n != 1 {
			t.Errorf("the display had event %d %d times", eventID, n)
		}
	}

	// The display's board agrees with the stream: the dosa ready, the upma
	// still to make.
	rec := send(r.handler, http.MethodGet, "/api/v1/kitchen/kitchen/tickets", "", kdsHeaders)
	var board kitchenBoardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &board); err != nil || len(board.Tickets) != 2 {
		t.Fatalf("board: %s (%v)", rec.Body, err)
	}
	if board.Tickets[0].Status != "ready" || board.Tickets[1].Status != "fired" || board.Tickets[1].Lines[0].Name != "Upma" {
		t.Errorf("board = %+v", board.Tickets)
	}

	// The phase's done-when, to the end: course 2 fired to two stations,
	// the kitchen's on the display and the bar's not; the bill settled;
	// the table dirty, then cleared, then free.
	lineIn("Kulfi", "kitchen", 2)
	lineIn("Masala chai", "bar", 2)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/courses/2/fire", ``)
	fired := kds.await("ticket.sent", within)
	if last := fired[len(fired)-1]; last.Station != "kitchen" || !strings.Contains(string(last.Event), `"station":"kitchen"`) {
		t.Errorf("course 2 on the display: %s", last.Event)
	}
	// The till hears both tickets; the bar's comes after the kitchen's.
	for bar := false; !bar; {
		got := till.await("ticket.sent", within)
		bar = got[len(got)-1].Station == "bar"
	}

	var bill orderResponse
	rec = send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", r.asStaff(r.staffToken))
	if err := json.Unmarshal(rec.Body.Bytes(), &bill); err != nil {
		t.Fatal(err)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", fmt.Sprintf(
		`{"method":"cash","amount":{"minor":%d,"currency":"INR"},"tip":{"minor":0,"currency":"INR"}}`, bill.Totals.BalanceDue.Minor))
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/close", ``)
	till.await("order.closed", within)

	tableAt := func() liveTableResponse {
		t.Helper()
		rec := send(r.handler, http.MethodGet, "/api/v1/floor/live", "", r.asStaff(r.staffToken))
		var live liveFloorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &live); err != nil {
			t.Fatalf("live floor: %s (%v)", rec.Body, err)
		}
		for _, tb := range live.Tables {
			if tb.ID == 4 {
				return tb
			}
		}
		t.Fatal("table 4 is not on the floor")
		return liveTableResponse{}
	}
	if tb := tableAt(); tb.Status != "dirty" {
		t.Errorf("after the bill, table 4 is %s", tb.Status)
	}
	if rec := send(r.handler, http.MethodPost, "/api/v1/floor/tables/4/clear", "", r.asStaff(r.staffToken)); rec.Code != http.StatusOK {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body)
	}
	if m, ok := till.nextOf(string(hub.MsgFloorChanged), within); !ok {
		t.Errorf("the till was not told the floor changed: %+v", m)
	}
	if tb := tableAt(); tb.Status != "free" {
		t.Errorf("after clearing, table 4 is %s", tb.Status)
	}

	// Since it reconnected, the display heard its station and nothing else
	// (before, its first socket's events were checked as they came).
	for _, m := range kds.history {
		if m.Station != "kitchen" {
			t.Errorf("the display heard %d %s for %q", m.ID, m.eventType(), m.Station)
		}
	}
}

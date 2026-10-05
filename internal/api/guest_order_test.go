package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/byjoellobo/mez/internal/domain/order"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// orderingWorld is guestWorld with the venue open and taking guests' orders:
// a flat white that needs a milk, dairy or oat.
type orderingWorld struct {
	*guestWorld
	orders     *svcorder.Service
	white, oat int64
}

func newOrderingWorld(t *testing.T) *orderingWorld {
	t.Helper()
	g := newGuestWorld(t)
	g.openTestShift(t)
	orders, err := svcorder.New(svcorder.Options{DB: g.db})
	if err != nil {
		t.Fatal(err)
	}
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	g.rebuild(t, WithMenus(g.db), WithGuests(g.codes, "/order/"), WithGuestOrders(orders), WithStatic(map[string]fs.FS{"/order/": app}))

	_, _, body := g.guest(http.MethodGet, "/order/api/t/"+g.code+"/menu")
	var m guestMenuResponse
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("reading the menu: %v", err)
	}
	return &orderingWorld{guestWorld: g, orders: orders, white: m.Items[0].ID, oat: m.ModifierGroups[0].Modifiers[1].ID}
}

// place sends a cart of n oat flat whites from table 4.
func (o *orderingWorld) place(key string, n int) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":%d}]}`, o.white, o.oat, n)
	return o.post(key, body)
}

func (o *orderingWorld) post(key, body string) *httptest.ResponseRecorder {
	header := map[string]string{"Content-Type": "application/json"}
	if key != "" {
		header[HeaderIdempotencyKey] = key
	}
	return send(o.handler, http.MethodPost, "/order/api/t/"+o.code+"/orders", body, header)
}

// seat opens a ticket at table 4 as staff would, behind the API's back.
func (o *orderingWorld) seat(t *testing.T) svcorder.Result {
	t.Helper()
	res, err := o.orders.Open(context.Background(), svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1},
		svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2})
	if err != nil {
		t.Fatalf("seating table 4: %v", err)
	}
	return res
}

func decodeRound(t *testing.T, rec *httptest.ResponseRecorder) guestRoundResponse {
	t.Helper()
	var got guestRoundResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got
}

// TestGuestOrderWaits: nobody has seated the table, so the order is a
// request. The answer is priced by the server and says it is waiting.
func TestGuestOrderWaits(t *testing.T) {
	o := newOrderingWorld(t)
	rec := o.place("k1", 2)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	got := decodeRound(t, rec)
	if got.Status != svcorder.RoundWaiting || !strings.HasPrefix(got.ID, "rnd_") {
		t.Errorf("round = %+v", got)
	}
	// 2 x (180.00 + 40.00), the menu's prices and not anything the phone said.
	if got.Subtotal.Minor != 44000 || got.Subtotal.Currency != "INR" {
		t.Errorf("subtotal = %+v, want 440.00", got.Subtotal)
	}
	if len(got.Lines) != 1 || got.Lines[0].Name != "Flat white" || got.Lines[0].Qty != 2 ||
		len(got.Lines[0].Choices) != 1 || got.Lines[0].Choices[0] != "Oat" || got.Lines[0].Total.Minor != 44000 {
		t.Errorf("lines = %+v", got.Lines)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("an order's answer may be cached")
	}
	// Nothing of the venue's insides, and nothing only a till needs.
	for _, word := range []string{"order_id", "table_id", "venue_id", "tax_rate", "hsn", "station", "course"} {
		if strings.Contains(rec.Body.String(), `"`+word) {
			t.Errorf("the answer carries %s: %s", word, rec.Body)
		}
	}
}

// TestGuestOrderAtASeatedTableIsSent: staff opened the ticket, so the order
// goes to the kitchen, on that ticket.
func TestGuestOrderAtASeatedTableIsSent(t *testing.T) {
	o := newOrderingWorld(t)
	seated := o.seat(t)

	rec := o.place("k1", 1)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if got := decodeRound(t, rec); got.Status != svcorder.RoundSent {
		t.Errorf("round = %+v, want it sent", got)
	}
	res, err := o.orders.Order(context.Background(), 1, seated.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 1 || !res.State.Lines[0].Sent || len(res.State.Tickets) != 1 || res.State.Tickets[0].Station != "bar" {
		t.Errorf("the ticket has lines %+v and tickets %+v", res.State.Lines, res.State.Tickets)
	}
}

// TestGuestOrderSentTwiceIsOneOrder: the phone retries with the same key
// and gets the same answer, with 200 and not 201 the second time.
func TestGuestOrderSentTwiceIsOneOrder(t *testing.T) {
	o := newOrderingWorld(t)
	seated := o.seat(t)

	first := o.place("k1", 1)
	again := o.place("k1", 1)
	if first.Code != http.StatusCreated || again.Code != http.StatusOK {
		t.Fatalf("statuses %d then %d, want 201 then 200", first.Code, again.Code)
	}
	if a, b := decodeRound(t, first), decodeRound(t, again); a.ID != b.ID || b.Status != svcorder.RoundSent {
		t.Errorf("first %+v, again %+v", a, b)
	}
	res, err := o.orders.Order(context.Background(), 1, seated.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 1 {
		t.Errorf("%d lines on the ticket, want 1", len(res.State.Lines))
	}

	other := o.place("k1", 3)
	if other.Code != http.StatusConflict || errorOf(t, other).Code != CodeIdempotencyKeyReused {
		t.Errorf("the key with another cart: %d %s", other.Code, other.Body)
	}
}

func TestGuestOrderRefusals(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, o *orderingWorld)
		send    func(o *orderingWorld) *httptest.ResponseRecorder
		status  int
		code    ErrorCode
		says    string
	}{
		{"no key", nil, func(o *orderingWorld) *httptest.ResponseRecorder { return o.place("", 1) },
			http.StatusBadRequest, CodeInvalidRequest, "idempotency key"},
		{"an empty cart", nil, func(o *orderingWorld) *httptest.ResponseRecorder { return o.post("k", `{"lines":[]}`) },
			http.StatusBadRequest, CodeInvalidRequest, ""},
		{"not JSON", nil, func(o *orderingWorld) *httptest.ResponseRecorder { return o.post("k", `lines`) },
			http.StatusBadRequest, CodeInvalidRequest, ""},
		{"a price from the phone", nil, func(o *orderingWorld) *httptest.ResponseRecorder {
			return o.post("k", fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":1,"unit_price":{"minor":1,"currency":"INR"}}]}`, o.white, o.oat))
		}, http.StatusBadRequest, CodeInvalidRequest, ""},
		{"a dish that is gone", nil, func(o *orderingWorld) *httptest.ResponseRecorder {
			return o.post("k", `{"lines":[{"item_id":999,"modifier_ids":[],"qty":1}]}`)
		}, http.StatusConflict, CodeDishUnavailable, "A dish in your cart is no longer available"},
		{"a milk left out", nil, func(o *orderingWorld) *httptest.ResponseRecorder {
			return o.post("k", fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[],"qty":1}]}`, o.white))
		}, http.StatusConflict, CodeChoicesInvalid, "The choices for Flat white have changed"},
		{"too many of one dish", nil, func(o *orderingWorld) *httptest.ResponseRecorder { return o.place("k", 21) },
			http.StatusBadRequest, CodeInvalidRequest, "Flat white"},
		{"a ticket on hold", func(t *testing.T, o *orderingWorld) {
			if _, err := o.orders.Hold(context.Background(), svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}, o.seat(t).OrderID); err != nil {
				t.Fatal(err)
			}
		}, func(o *orderingWorld) *httptest.ResponseRecorder { return o.place("k", 1) },
			http.StatusConflict, CodeTableNotOrdering, "Ask a member of staff"},
		{"a code that leads nowhere", nil, func(o *orderingWorld) *httptest.ResponseRecorder {
			return send(o.handler, http.MethodPost, "/order/api/t/abcdefghjkmn/orders", `{"lines":[]}`, map[string]string{HeaderIdempotencyKey: "k"})
		}, http.StatusNotFound, CodeTableCodeUnknown, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newOrderingWorld(t)
			if tt.prepare != nil {
				tt.prepare(t, o)
			}
			rec := tt.send(o)
			got := errorOf(t, rec)
			if rec.Code != tt.status || got.Code != tt.code {
				t.Fatalf("%d %s: %s, want %d %s", rec.Code, got.Code, got.Message, tt.status, tt.code)
			}
			if !strings.Contains(got.Message, tt.says) {
				t.Errorf("message %q does not say %q", got.Message, tt.says)
			}
		})
	}
}

// TestGuestOrderWithTheVenueShut: no shift, no orders, and the guest is told
// in words that are theirs to act on.
func TestGuestOrderWithTheVenueShut(t *testing.T) {
	g := newGuestWorld(t)
	orders, err := svcorder.New(svcorder.Options{DB: g.db})
	if err != nil {
		t.Fatal(err)
	}
	g.rebuild(t, WithMenus(g.db), WithGuests(g.codes, "/order/"), WithGuestOrders(orders))
	o := &orderingWorld{guestWorld: g, orders: orders, white: 1, oat: 2}

	rec := o.place("k", 1)
	got := errorOf(t, rec)
	if rec.Code != http.StatusConflict || got.Code != CodeNoOpenShift || !strings.Contains(got.Message, "Ordering is closed") {
		t.Errorf("%d %+v", rec.Code, got)
	}
}

// TestGuestOrderPastTheWaitingLimit: a table can only have so many requests
// nobody has answered.
func TestGuestOrderPastTheWaitingLimit(t *testing.T) {
	o := newOrderingWorld(t)
	for i := range 5 {
		if rec := o.place(fmt.Sprint("w", i), 1); rec.Code != http.StatusCreated {
			t.Fatalf("request %d: %d %s", i, rec.Code, rec.Body)
		}
	}
	rec := o.place("one-more", 1)
	if got := errorOf(t, rec); rec.Code != http.StatusConflict || got.Code != CodeRequestsWaiting {
		t.Errorf("%d %+v", rec.Code, got)
	}
}

// TestGuestOrdersNeedTheOption: with no order service the route is not
// there, and the answer is the API's own 404 and not the app's page.
func TestGuestOrdersNeedTheOption(t *testing.T) {
	g := newGuestWorld(t)
	rec := send(g.handler, http.MethodPost, "/order/api/t/"+g.code+"/orders", `{"lines":[]}`, nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "<title>") {
		t.Error("the app's page answered an API call")
	}
}

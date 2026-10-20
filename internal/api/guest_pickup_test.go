package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/gateway"
	"github.com/mez-pos/mez/internal/service/charges"
	svcorder "github.com/mez-pos/mez/internal/service/order"
	"github.com/mez-pos/mez/internal/service/tablecodes"
)

// pickupWorld is prepayWorld with a pickup code for the venue: a gateway is
// there, so orders to collect are taken.
type pickupWorld struct {
	*prepayWorld
	// pickup is the venue's pickup code, and base the API under it.
	pickup, base string
}

func newPickupWorld(t *testing.T, publicURL string) *pickupWorld {
	t.Helper()
	p := newPrepayWorld(t, publicURL)
	pickup, err := p.codes.RenewPickup(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return &pickupWorld{prepayWorld: p, pickup: pickup, base: "/order/api/p/" + pickup}
}

// sendFrom is send from an address of the caller's choosing: a phone on its
// own mobile data, and not the one every other test asks from.
func sendFrom(h http.Handler, addr, method, path, body string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = addr + ":4000"
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// cart is n oat flat whites, as a phone sends them.
func (p *pickupWorld) cart(n int) string {
	return fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":%d}]}`, p.white, p.oat, n)
}

// collect places an order to collect of n oat flat whites.
func (p *pickupWorld) collect(key string, n int) *httptest.ResponseRecorder {
	header := map[string]string{"Content-Type": "application/json"}
	if key != "" {
		header[HeaderIdempotencyKey] = key
	}
	return send(p.handler, http.MethodPost, p.base+"/orders", p.cart(n), header)
}

// held places an order to collect, which is held unpaid, and answers its id.
func (p *pickupWorld) held(t *testing.T, key string) string {
	t.Helper()
	rec := p.collect(key, 2)
	got := decodeRound(t, rec)
	if rec.Code != http.StatusCreated || got.Status != svcorder.RoundUnpaid || got.Due == nil || got.Due.Minor != 44000 {
		t.Fatalf("holding an order to collect: %d %s", rec.Code, rec.Body)
	}
	return got.ID
}

func (p *pickupWorld) get(path string) *httptest.ResponseRecorder {
	return send(p.handler, http.MethodGet, path, "", nil)
}

func (p *pickupWorld) post(path string) *httptest.ResponseRecorder {
	return send(p.handler, http.MethodPost, path, "", nil)
}

// TestAGuestOrdersToCollect: the whole of it as a phone with no table sees
// it. The venue's code leads to the venue and its menu; an order is held with
// what it asks; its charge gives the gateway's page; until the gateway says
// paid it has no code and is not with the kitchen; then it is sent, and the
// order says the code it will be called out by.
func TestAGuestOrdersToCollect(t *testing.T) {
	p := newPickupWorld(t, "https://cafe.example")

	rec := p.get(p.base)
	var venue guestPickupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &venue); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("the venue: %d %s", rec.Code, rec.Body)
	}
	if venue.Venue.Name == "" || venue.Venue.Currency != "INR" || !venue.Ordering {
		t.Errorf("venue = %+v", venue)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("where a pickup code leads may be cached, and it can be turned off at any moment")
	}
	for _, word := range []string{"venue_id", `"id"`, "table"} {
		if strings.Contains(rec.Body.String(), word) {
			t.Errorf("the venue's answer mentions %s: %s", word, rec.Body)
		}
	}

	rec = p.get(p.base + "/menu")
	var m guestMenuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil || rec.Code != http.StatusOK || len(m.Items) != 1 || m.Items[0].Name != "Flat white" {
		t.Fatalf("the menu: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("the menu may be cached, and sold out turns on the clock")
	}

	round := p.held(t, "c1")
	if !strings.HasPrefix(round, "rnd_") {
		t.Errorf("round = %q", round)
	}
	// Sent again with its key: the same order, and 200.
	if again := p.collect("c1", 2); again.Code != http.StatusOK || decodeRound(t, again).ID != round {
		t.Errorf("placed again: %d %s", again.Code, again.Body)
	}
	if got := decodeRound(t, p.get(p.base+"/orders/"+round)); got.Status != svcorder.RoundUnpaid || got.PickupCode != "" || got.Progress != "" || got.Due == nil {
		t.Errorf("before paying: %+v", got)
	}

	rec = send(p.handler, http.MethodPost, p.base+"/orders/"+round+"/charge", "",
		map[string]string{"Host": "evil.example", "X-Forwarded-Host": "evil.example", "Origin": "https://evil.example"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("starting: %d %s", rec.Code, rec.Body)
	}
	started := decodeCharge(t, rec)
	if started.Status != charges.Open || started.Amount.Minor != 44000 || started.URL != "https://pay.test/fake_0001" || started.Round != round {
		t.Errorf("charge = %+v", started)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a charge's answer may be cached")
	}
	// Sent back to the venue's way in with no table, at the venue's own
	// address, whatever host the request claimed.
	asked := p.gw.Started[0]
	if asked.ReturnURL != "https://cafe.example/order/p/"+p.pickup || asked.Amount.Minor != 44000 || !strings.HasPrefix(asked.Description, "Order to collect, ") {
		t.Errorf("the gateway was asked for %+v", asked)
	}
	if again := p.post(p.base + "/orders/" + round + "/charge"); again.Code != http.StatusOK || decodeCharge(t, again).ID != started.ID || len(p.gw.Started) != 1 {
		t.Errorf("tapped again: %d %s", again.Code, again.Body)
	}
	if got := decodeCharge(t, p.get(p.base+"/charge/"+started.ID)); got.Status != charges.Open || got.URL == "" || got.Round != round {
		t.Errorf("the charge before paying: %+v", got)
	}

	p.gw.Pay("fake_0001", gateway.UPI)
	rec = p.get(p.base + "/charge/" + started.ID)
	if got := decodeCharge(t, rec); got.Status != charges.Paid || got.URL != "" || got.Round != round {
		t.Fatalf("the charge after paying: %+v", got)
	}
	rec = p.get(p.base + "/orders/" + round)
	sent := decodeRound(t, rec)
	if sent.Status != svcorder.RoundSent || !order.PickupCodeOK(sent.PickupCode) || sent.Progress != order.ProgressSent || sent.Due != nil {
		t.Errorf("after paying: %+v", sent)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("how an order stands may be cached, and it moves when the kitchen does")
	}
	// No bill to ask a receipt by: a receipt is asked by a table (ADR-240).
	for _, word := range []string{"order_id", "table_id", "venue_id", `"bill"`} {
		if strings.Contains(rec.Body.String(), word) {
			t.Errorf("the order's answer carries %s: %s", word, rec.Body)
		}
	}
	if done := p.post(p.base + "/orders/" + round + "/charge"); done.Code != http.StatusConflict || errorOf(t, done).Code != CodeNothingToPay {
		t.Errorf("paying for it again: %d %s", done.Code, done.Body)
	}
	// It is no table's: table 4 has no bill for it.
	if bill, _ := p.bill(t); bill.Open {
		t.Error("an order to collect is on table 4's bill")
	}
}

// TestOrderingToCollectIsRefused: a code that is not the venue's, and under
// the venue's code, an order or a payment that is not one to collect. A
// table's code and the venue's say nothing of each other's.
func TestOrderingToCollectIsRefused(t *testing.T) {
	tests := []struct {
		name   string
		ask    func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder
		status int
		code   ErrorCode
	}{
		{"a code nobody made", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get("/order/api/p/" + strings.Repeat("2", tablecodes.Length))
		}, http.StatusNotFound, CodePickupCodeUnknown},
		{"a table's code where the venue's goes", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get("/order/api/p/" + p.code)
		}, http.StatusNotFound, CodePickupCodeUnknown},
		{"a table's code, for the menu", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get("/order/api/p/" + p.code + "/menu")
		}, http.StatusNotFound, CodePickupCodeUnknown},
		{"a code turned off", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			if err := p.codes.RevokePickup(context.Background(), 1); err != nil {
				t.Fatal(err)
			}
			return p.collect("c1", 1)
		}, http.StatusNotFound, CodePickupCodeUnknown},
		{"the venue's code where a table's goes", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get("/order/api/t/" + p.pickup)
		}, http.StatusNotFound, CodeTableCodeUnknown},
		{"an order with no key", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.collect("", 1)
		}, http.StatusBadRequest, CodeInvalidRequest},
		{"an empty cart", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return send(p.handler, http.MethodPost, p.base+"/orders", `{"lines":[]}`, map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "c1"})
		}, http.StatusBadRequest, CodeInvalidRequest},
		{"a dish that is not on the menu", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return send(p.handler, http.MethodPost, p.base+"/orders", `{"lines":[{"item_id":9999,"modifier_ids":[],"qty":1}]}`, map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "c1"})
		}, http.StatusConflict, CodeDishUnavailable},
		{"with the venue shut", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			ctx := context.Background()
			err := p.db.WriteTx(ctx, func(q *gen.Queries) error {
				open, err := q.OpenShiftIDForVenue(ctx, 1)
				if err != nil {
					return err
				}
				_, err = q.CloseShift(ctx, gen.CloseShiftParams{ID: open, ClosedBy: 1, ClosedAt: sql.NullTime{Time: seededAt, Valid: true}})
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			return p.collect("c1", 1)
		}, http.StatusConflict, CodeNoOpenShift},
		{"an order that is not there", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get(p.base + "/orders/rnd_NOTTHERE")
		}, http.StatusNotFound, CodeRequestNotFound},
		{"paying for an order that is not there", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.post(p.base + "/orders/rnd_NOTTHERE/charge")
		}, http.StatusNotFound, CodeRequestNotFound},
		{"a table's order, by the venue's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get(p.base + "/orders/" + p.hold(t, "k1"))
		}, http.StatusNotFound, CodeRequestNotFound},
		{"paying for a table's order by the venue's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.post(p.base + "/orders/" + p.hold(t, "k1") + "/charge")
		}, http.StatusNotFound, CodeRequestNotFound},
		{"an order to collect, by a table's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.track(p.code, p.held(t, "c1"))
		}, http.StatusNotFound, CodeRequestNotFound},
		{"paying for an order to collect by a table's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.pay(p.code, p.held(t, "c1"), nil)
		}, http.StatusNotFound, CodeRequestNotFound},
		{"a payment that is not there", func(_ *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			return p.get(p.base + "/charge/chg_NOTTHERE")
		}, http.StatusNotFound, CodeChargeNotFound},
		{"a table's payment, by the venue's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			started := decodeCharge(t, p.pay(p.code, p.hold(t, "k1"), nil))
			return p.get(p.base + "/charge/" + started.ID)
		}, http.StatusNotFound, CodeChargeNotFound},
		{"an order to collect's payment, by a table's code", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			started := decodeCharge(t, p.post(p.base+"/orders/"+p.held(t, "c1")+"/charge"))
			return p.get("/order/api/t/" + p.code + "/bill/charge/" + started.ID)
		}, http.StatusNotFound, CodeChargeNotFound},
		{"with the gateway down", func(t *testing.T, p *pickupWorld) *httptest.ResponseRecorder {
			round := p.held(t, "c1")
			p.gw.Down(true)
			return p.post(p.base + "/orders/" + round + "/charge")
		}, http.StatusServiceUnavailable, CodeGatewayUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPickupWorld(t, "")
			rec := tt.ask(t, p)
			got := errorOf(t, rec)
			if rec.Code != tt.status || got.Code != tt.code {
				t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			// Nobody ordering to collect is at a table, and is not told of one.
			// What a table's own routes answer is in a table's words.
			if strings.Contains(tt.name, "by a table's code") || strings.Contains(tt.name, "where a table's goes") {
				return
			}
			if strings.Contains(strings.ToLower(got.Message), "table") {
				t.Errorf("a guest with no table is told of one: %q", got.Message)
			}
		})
	}
}

// TestWithNoGatewayNothingIsOrderedToCollect: an order to collect is paid
// first, so where guests cannot pay online there is none. The code still
// leads to the venue, which says so, and everything else under it says so
// in a guest's words and is not a code that leads nowhere.
func TestWithNoGatewayNothingIsOrderedToCollect(t *testing.T) {
	o := newOrderingWorld(t)
	pickup, err := o.codes.RenewPickup(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	base := "/order/api/p/" + pickup

	rec := send(o.handler, http.MethodGet, base, "", nil)
	var venue guestPickupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &venue); err != nil || rec.Code != http.StatusOK || venue.Venue.Name == "" || venue.Ordering {
		t.Fatalf("the venue: %d %s", rec.Code, rec.Body)
	}

	cart := fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":1}]}`, o.white, o.oat)
	asks := []struct{ method, path, body string }{
		{http.MethodGet, base + "/menu", ""},
		{http.MethodPost, base + "/orders", cart},
		{http.MethodGet, base + "/orders/rnd_ANYTHING", ""},
		{http.MethodPost, base + "/orders/rnd_ANYTHING/charge", ""},
		{http.MethodGet, base + "/charge/chg_ANYTHING", ""},
	}
	for _, a := range asks {
		rec := send(o.handler, a.method, a.path, a.body, map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "c1"})
		if got := errorOf(t, rec); rec.Code != http.StatusConflict || got.Code != CodePickupOff || !strings.Contains(got.Message, "counter") {
			t.Errorf("%s %s: %d %s", a.method, a.path, rec.Code, rec.Body)
		}
	}
	// A wrong code is still a wrong code.
	wrong := send(o.handler, http.MethodGet, "/order/api/p/"+strings.Repeat("2", tablecodes.Length)+"/menu", "", nil)
	if wrong.Code != http.StatusNotFound || errorOf(t, wrong).Code != CodePickupCodeUnknown {
		t.Errorf("a wrong code with no gateway: %d %s", wrong.Code, wrong.Body)
	}
}

// TestOrdersToCollectAreHeldToAFew: the venue's code is on a door, so what it
// may order is counted by who is asking and then by the venue. One address
// is stopped well short of the venue's count; past the venue's, nobody
// orders to collect, and a table orders as it did.
func TestOrdersToCollectAreHeldToAFew(t *testing.T) {
	lower(t, &pickupOrderByAddr, limit{requests: 2, per: 10 * time.Minute})
	lower(t, &pickupOrderLimit, limit{requests: 3, per: 10 * time.Minute})
	p := newPickupWorld(t, "")
	from := func(addr, key string) *httptest.ResponseRecorder {
		return sendFrom(p.handler, addr, http.MethodPost, p.base+"/orders", p.cart(1), map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: key})
	}
	for i := range 2 {
		if rec := from("198.51.100.1", fmt.Sprintf("a%d", i)); rec.Code != http.StatusCreated {
			t.Fatalf("order %d from the first phone: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := from("198.51.100.1", "a-over")
	if got := errorOf(t, rec); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || got.Code != CodeRateLimited ||
		!strings.Contains(got.Message, "counter") || strings.Contains(strings.ToLower(got.Message), "table") {
		t.Fatalf("the first phone past its limit: %d %s", rec.Code, rec.Body)
	}
	// What the first phone was refused is not counted against the venue: a
	// second phone has the venue's third, and then the venue is at its limit.
	if rec := from("198.51.100.2", "b0"); rec.Code != http.StatusCreated {
		t.Fatalf("the second phone: %d %s", rec.Code, rec.Body)
	}
	if rec := from("198.51.100.2", "b1"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("the venue's fourth order: %d %s", rec.Code, rec.Body)
	}
	if rec := from("198.51.100.3", "c0"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a third phone with the venue at its limit: %d %s", rec.Code, rec.Body)
	}
	// Reading is counted apart, and a table is not held back by the door.
	if rec := sendFrom(p.handler, "198.51.100.1", http.MethodGet, p.base, "", nil); rec.Code != http.StatusOK {
		t.Errorf("reading with orders at their limit: %d %s", rec.Code, rec.Body)
	}
	if rec := p.place("k1", 1); rec.Code != http.StatusCreated {
		t.Errorf("table 4 with the venue's orders to collect at their limit: %d %s", rec.Code, rec.Body)
	}
}

// TestOnePhoneCannotReadTheVenuesCountAway: an address reading under the
// venue's code past its own limit waits, and another phone reads on.
func TestOnePhoneCannotReadTheVenuesCountAway(t *testing.T) {
	lower(t, &pickupReadByAddr, limit{requests: 3, per: time.Minute})
	lower(t, &pickupReadLimit, limit{requests: 5, per: time.Minute})
	p := newPickupWorld(t, "")
	read := func(addr string) int { return sendFrom(p.handler, addr, http.MethodGet, p.base, "", nil).Code }
	for i := range 3 {
		if code := read("198.51.100.1"); code != http.StatusOK {
			t.Fatalf("read %d: %d", i+1, code)
		}
	}
	for range 10 {
		if code := read("198.51.100.1"); code != http.StatusTooManyRequests {
			t.Fatalf("past its limit: %d", code)
		}
	}
	// Three of the venue's five are used, and not thirteen.
	for i := range 2 {
		if code := read("198.51.100.2"); code != http.StatusOK {
			t.Fatalf("another phone's read %d: %d", i+1, code)
		}
	}
	if code := read("198.51.100.3"); code != http.StatusTooManyRequests {
		t.Errorf("the venue's sixth read: %d", code)
	}
	// It can still be changed under, and a table's reads are its own.
	if rec := sendFrom(p.handler, "198.51.100.3", http.MethodPost, p.base+"/orders", p.cart(1), map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "c1"}); rec.Code != http.StatusCreated {
		t.Errorf("ordering with reads at their limit: %d %s", rec.Code, rec.Body)
	}
	if status, _, _ := p.guest(http.MethodGet, "/order/api/t/"+p.code); status != http.StatusOK {
		t.Errorf("table 4 with the venue's code read out: %d", status)
	}
}

// TestGuessingPickupCodesIsGuessing: a wrong pickup code is a wrong code,
// counted with the wrong table codes from the same address.
func TestGuessingPickupCodesIsGuessing(t *testing.T) {
	p := newPickupWorld(t, "")
	wrong := "/order/api/p/" + strings.Repeat("2", tablecodes.Length)
	for i := range guestBadCodeLimit.requests {
		if rec := p.get(wrong); rec.Code != http.StatusNotFound {
			t.Fatalf("guess %d: %d", i+1, rec.Code)
		}
	}
	if rec := p.get(p.base); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("the right pickup code from an address that has been guessing: %d", rec.Code)
	}
	if rec := p.get("/order/api/t/" + p.code); rec.Code != http.StatusTooManyRequests {
		t.Errorf("a table's code from an address that has been guessing pickup codes: %d", rec.Code)
	}
	// A good code is no guess.
	q := newPickupWorld(t, "")
	for i := range guestBadCodeLimit.requests * 2 {
		if rec := q.get(q.base); rec.Code != http.StatusOK {
			t.Fatalf("read %d of a good code: %d %s", i+1, rec.Code, rec.Body)
		}
	}
}

// TestTheLimitsFitALunchQueue: thirty phones ordering to collect at once,
// each tracking its order every ten seconds and asking about its payment
// every four, with the menu once. The venue's limit is twice that; one
// address may read for several phones behind it; everyone in the queue can
// order and pay with a second thought; and one address cannot fill what may
// wait unpaid at the venue, in the half hour an unpaid order counts for.
func TestTheLimitsFitALunchQueue(t *testing.T) {
	const perPhone = 60/10 + 60/4 + 1
	const queue = svcorder.MaxUnpaidPickups
	if asks := queue * perPhone; pickupReadLimit.requests < 2*asks {
		t.Errorf("a queue of %d asks %d a minute by itself, and the venue's code may read %d: less than twice it", queue, asks, pickupReadLimit.requests)
	}
	if five := 5 * perPhone; pickupReadByAddr.requests < 2*five || pickupReadByAddr.requests >= pickupReadLimit.requests {
		t.Errorf("one address may read %d a minute: five phones behind it ask %d, and the venue's whole count is %d", pickupReadByAddr.requests, five, pickupReadLimit.requests)
	}
	if pickupOrderLimit.requests < queue || pickupWriteLimit.requests < 3*queue {
		t.Errorf("a queue of %d cannot each order (%d) or order and pay with a second thought (%d)", queue, pickupOrderLimit.requests, pickupWriteLimit.requests)
	}
	if pickupOrderByAddr.requests < 3 || 3*pickupOrderByAddr.requests >= queue {
		t.Errorf("one address may order %d in ten minutes: under 3, or enough in half an hour to fill the %d that may wait unpaid", pickupOrderByAddr.requests, queue)
	}
	if pickupWriteByAddr.requests < 2*pickupOrderByAddr.requests || pickupWriteByAddr.requests >= pickupWriteLimit.requests {
		t.Errorf("one address may change %d a minute, against %d orders and the venue's %d", pickupWriteByAddr.requests, pickupOrderByAddr.requests, pickupWriteLimit.requests)
	}
}

// TestTheTillsListSaysWhatIsToCollect: a paid order to collect is on the
// day's orders with the code it is called by and how far the kitchen has
// come with it. A table's order has neither.
func TestTheTillsListSaysWhatIsToCollect(t *testing.T) {
	p := newPickupWorld(t, "")
	ctx := context.Background()
	round := p.held(t, "c1")
	started := decodeCharge(t, p.post(p.base+"/orders/"+round+"/charge"))
	p.gw.Pay("fake_0001", gateway.UPI)
	if got := decodeCharge(t, p.get(p.base+"/charge/"+started.ID)); got.Status != charges.Paid {
		t.Fatalf("the charge: %+v", got)
	}
	code := decodeRound(t, p.get(p.base+"/orders/"+round)).PickupCode
	seated := p.seat(t)

	day, err := p.orders.DayOrders(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	list := toDayOrdersResponse(day)
	if len(list.Orders) != 2 {
		t.Fatalf("%d orders today", len(list.Orders))
	}
	for _, o := range list.Orders {
		if o.ID == seated.PublicID {
			if o.PickupCode != "" || o.Progress != "" {
				t.Errorf("table 4's order = %+v", o)
			}
			continue
		}
		// Paid, and not handed over: its ticket is still open.
		if o.PickupCode != code || code == "" || o.Progress != order.ProgressSent || o.Status != svcorder.DayOpen || o.BalanceDue.Minor != 0 || o.TableID != 0 {
			t.Errorf("the order to collect = %+v, want code %q", o, code)
		}
	}
	body, err := json.Marshal(list)
	if err != nil || !strings.Contains(string(body), `"pickup_code":"`+code+`"`) || strings.Count(string(body), "pickup_code") != 1 {
		t.Errorf("the list as sent: %s, %v", body, err)
	}
}

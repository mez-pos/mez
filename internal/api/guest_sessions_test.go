package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// A guest's phone and its session for the sitting (ADR-262). The phones in
// these tests send what they are given and nothing else: send, not
// phoneSend.

func landingOf(t *testing.T, rec *httptest.ResponseRecorder) guestTableResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("landing: %d %s", rec.Code, rec.Body)
	}
	var got guestTableResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got
}

func withSession(token string, header map[string]string) map[string]string {
	out := map[string]string{HeaderGuestSession: token}
	for k, v := range header {
		out[k] = v
	}
	return out
}

func (o *orderingWorld) whites(n int) string {
	return fmt.Sprintf(`{"lines":[{"item_id":%d,"modifier_ids":[%d],"qty":%d}]}`, o.white, o.oat, n)
}

// orderAs places an order from table 4 as a phone holding token, or none.
func (o *orderingWorld) orderAs(token, key string) *httptest.ResponseRecorder {
	header := map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: key}
	if token != "" {
		header[HeaderGuestSession] = token
	}
	return send(o.handler, http.MethodPost, "/order/api/t/"+o.code+"/orders", o.whites(1), header)
}

func wantSessionRequired(t *testing.T, what string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("%s: status %d, body %s; want 401", what, rec.Code, rec.Body)
		return
	}
	got := errorOf(t, rec)
	if got.Code != CodeGuestSessionRequired || got.Message != "Scan the code on your table again to continue." {
		t.Errorf("%s: %+v", what, got)
	}
}

// TestGuestLandingHandsOutASession: at a table with no ticket, once for a
// phone that has none good; at a table with a ticket, never.
func TestGuestLandingHandsOutASession(t *testing.T) {
	o := newOrderingWorld(t)
	path := "/order/api/t/" + o.code

	first := landingOf(t, send(o.handler, http.MethodGet, path, "", nil))
	if !strings.HasPrefix(first.Session, "gst_") || len(first.Session) != 47 {
		t.Fatalf("session = %q", first.Session)
	}
	if again := landingOf(t, send(o.handler, http.MethodGet, path, "", withSession(first.Session, nil))); again.Session != "" {
		t.Errorf("a phone with a good session was handed another: %q", again.Session)
	}
	if fresh := landingOf(t, send(o.handler, http.MethodGet, path, "", withSession("gst_nonsense", nil))); fresh.Session == "" || fresh.Session == first.Session {
		t.Errorf("a phone with a bad session was handed %q", fresh.Session)
	}

	o.seat(t)
	rec := send(o.handler, http.MethodGet, path, "", nil)
	if got := landingOf(t, rec); got.Session != "" {
		t.Errorf("a seated table handed out %q", got.Session)
	}
	if strings.Contains(rec.Body.String(), `"session"`) {
		t.Errorf("the answer names a session it does not give: %s", rec.Body)
	}
}

// TestGuestPrivateRoutesNeedASession: the bill, its QR, saying it is paid,
// paying it and letting a payment go are the sitting's. The landing, the
// menu, tracking an order and a receipt are not.
func TestGuestPrivateRoutesNeedASession(t *testing.T) {
	c := newChargeWorld(t, "")
	started := decodeCharge(t, c.start())
	base := "/order/api/t/" + c.code
	stranger := landingOf(t, send(c.handler, http.MethodGet, "/order/api/t/"+c.tableCode(t, 5), "", nil)).Session
	if stranger == "" {
		t.Fatal("table 5 handed out no session")
	}
	protected := []struct{ method, path string }{
		{http.MethodGet, base + "/bill?phone=abcdefgh"},
		{http.MethodGet, base + "/bill/upi.svg"},
		{http.MethodPost, base + "/bill/paid"},
		{http.MethodPost, base + "/bill/charge"},
		{http.MethodGet, base + "/bill/charge/" + started.ID},
		{http.MethodPost, base + "/bill/charge/" + started.ID + "/let-go"},
	}
	for _, p := range protected {
		for name, header := range map[string]map[string]string{
			"no session":        nil,
			"a made-up session": withSession("gst_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", nil),
			"table 5's session": withSession(stranger, nil),
		} {
			wantSessionRequired(t, p.method+" "+p.path+" with "+name, send(c.handler, p.method, p.path, "", header))
		}
	}
	// Refused, nothing was done: the bill is still owed in full and the
	// payment page still holds it.
	if bill, _ := c.bill(t); bill.BalanceDue.Minor != 44000 || bill.PaidTold {
		t.Errorf("bill after the refusals: %+v", bill)
	}
	if got := decodeCharge(t, c.ask(started.ID)); got.LetGo {
		t.Error("a phone with no session let the payment go")
	}

	placed := decodeRound(t, c.place("k2", 1))
	open := []struct{ method, path string }{
		{http.MethodGet, base},
		{http.MethodGet, base + "/menu"},
		{http.MethodGet, base + "/orders/" + placed.ID},
	}
	for _, p := range open {
		if rec := send(c.handler, p.method, p.path, "", nil); rec.Code != http.StatusOK {
			t.Errorf("%s %s with no session: %d %s", p.method, p.path, rec.Code, rec.Body)
		}
	}
}

// TestGuestJoiningAnOpenTicket: a phone with no session at a table staff
// have seated asks to join. Staff are told so, and once they accept, the
// phone is handed its session by tracking its order, until it uses it.
func TestGuestJoiningAnOpenTicket(t *testing.T) {
	w := newRequestsWorld(t)
	w.seat(t)

	rec := w.orderAs("", "j1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	placed := decodeRound(t, rec)
	if placed.Status != svcorder.RoundWaiting || placed.Session != "" {
		t.Fatalf("round = %+v, want a request with no session yet", placed)
	}
	if got := w.waiting(t); len(got) != 1 || !got[0].JoinsOpenTicket {
		t.Fatalf("waiting = %+v, want one asking to join", got)
	}
	trackPath := "/order/api/t/" + w.code + "/orders/" + placed.ID
	if before := decodeRound(t, send(w.handler, http.MethodGet, trackPath, "", nil)); before.Session != "" {
		t.Error("a session was handed over before anyone accepted")
	}

	if accepted := w.staffCall(http.MethodPost, "/api/v1/guest-requests/"+placed.ID+"/accept"); accepted.Code != http.StatusOK {
		t.Fatalf("accepting: %d %s", accepted.Code, accepted.Body)
	}
	handed := decodeRound(t, send(w.handler, http.MethodGet, trackPath, "", nil))
	if handed.Status != svcorder.RoundSent || !strings.HasPrefix(handed.Session, "gst_") {
		t.Fatalf("tracked = %+v, want it sent with a session", handed)
	}
	bill := send(w.handler, http.MethodGet, "/order/api/t/"+w.code+"/bill?phone=abcdefgh", "", withSession(handed.Session, nil))
	if bill.Code != http.StatusOK {
		t.Fatalf("the bill with the handed session: %d %s", bill.Code, bill.Body)
	}
	if after := decodeRound(t, send(w.handler, http.MethodGet, trackPath, "", nil)); after.Session != "" {
		t.Error("the session is still handed over after the phone used it")
	}
	// Of the party now: straight to the kitchen.
	if next := decodeRound(t, w.orderAs(handed.Session, "j2")); next.Status != svcorder.RoundSent {
		t.Errorf("the phone's next order = %+v, want it sent", next)
	}
}

// TestGuestStaleSessions: a session from before the ticket opened, or from
// another table, is refused when ordering too, and the phone is told to
// scan again; with none at an empty table it is told the same.
func TestGuestStaleSessions(t *testing.T) {
	o := newOrderingWorld(t)
	early := landingOf(t, send(o.handler, http.MethodGet, "/order/api/t/"+o.code, "", nil)).Session
	elsewhere := landingOf(t, send(o.handler, http.MethodGet, "/order/api/t/"+o.tableCode(t, 5), "", nil)).Session

	wantSessionRequired(t, "no session at an empty table", o.orderAs("", "s0"))
	wantSessionRequired(t, "table 5's session", o.orderAs(elsewhere, "s1"))
	o.seat(t)
	wantSessionRequired(t, "a session from before the ticket", o.orderAs(early, "s2"))
	// Nothing was asked of staff or sent for any of them.
	if res, open, _ := o.orders.TableTicket(context.Background(), 1, 4); !open || len(res.State.Lines) != 0 {
		t.Errorf("the ticket has %d lines", len(res.State.Lines))
	}
}

// TestGuestClosedBillEndsTheSession: once the bill is closed the phone's
// session is over, and the next sitting's bill is not its to read.
func TestGuestClosedBillEndsTheSession(t *testing.T) {
	c := newChargeWorld(t, "")
	token := c.sessionFor(c.code)
	path := "/order/api/t/" + c.code + "/bill?phone=abcdefgh"
	if rec := send(c.handler, http.MethodGet, path, "", withSession(token, nil)); rec.Code != http.StatusOK {
		t.Fatalf("the bill before it closed: %d %s", rec.Code, rec.Body)
	}
	ctx := context.Background()
	staff := svcorder.Meta{VenueID: 1, ActorID: 1, DeviceID: 1}
	if _, err := c.orders.TenderPayment(ctx, staff, c.orderID, cashOf(44000)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.orders.Close(ctx, staff, c.orderID); err != nil {
		t.Fatal(err)
	}
	c.seat(t) // the next party
	wantSessionRequired(t, "the bill after its sitting", send(c.handler, http.MethodGet, path, "", withSession(token, nil)))
}

// TestGuestPayingFirstWithNoSession: where guests pay first, a phone with no
// session at a table with a ticket is held to pay as any is, asks nobody,
// and may pay for its own order.
func TestGuestPayingFirstWithNoSession(t *testing.T) {
	p := newPrepayWorld(t, "")
	p.seat(t)
	rec := p.orderAs("", "pf1")
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	held := decodeRound(t, rec)
	if held.Status != svcorder.RoundUnpaid || held.Due == nil {
		t.Fatalf("round = %+v, want it held to be paid", held)
	}
	pay := send(p.handler, http.MethodPost, "/order/api/t/"+p.code+"/orders/"+held.ID+"/charge", "", nil)
	if pay.Code != http.StatusCreated {
		t.Errorf("paying for its own order with no session: %d %s", pay.Code, pay.Body)
	}
}

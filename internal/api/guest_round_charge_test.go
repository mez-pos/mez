package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/gateway/gatewaytest"
	"github.com/byjoellobo/mez/internal/service/charges"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// prepayWorld is a venue where guests pay first, and can: the setting says
// so and a fake gateway is there. No table is seated. The venue has no tax,
// so two oat flat whites ask 440.00.
type prepayWorld struct {
	*orderingWorld
	gw *gatewaytest.Fake
}

func newPrepayWorld(t *testing.T, publicURL string) *prepayWorld {
	t.Helper()
	g := newGuestWorld(t)
	g.openTestShift(t)
	ctx := context.Background()
	if err := g.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: svcorder.SettingPayMode, Value: string(svcorder.PayFirst), UpdatedAt: seededAt})
	}); err != nil {
		t.Fatal(err)
	}
	orders, err := svcorder.New(svcorder.Options{DB: g.db, CanCharge: true})
	if err != nil {
		t.Fatal(err)
	}
	gw := gatewaytest.New()
	svc, err := charges.New(charges.Options{
		DB: g.db, Orders: orders,
		Gateways: func(context.Context, int64) (gateway.Gateway, error) { return gw, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	g.rebuild(t, WithMenus(g.db), WithGuests(g.codes, "/order/"), WithGuestOrders(orders), WithCharges(svc, publicURL))
	_, _, body := g.guest(http.MethodGet, "/order/api/t/"+g.code+"/menu")
	var m guestMenuResponse
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	o := &orderingWorld{guestWorld: g, orders: orders, white: m.Items[0].ID, oat: m.ModifierGroups[0].Modifiers[1].ID}
	return &prepayWorld{orderingWorld: o, gw: gw}
}

// hold places a round, which is held unpaid, and answers its id.
func (p *prepayWorld) hold(t *testing.T, key string) string {
	t.Helper()
	rec := p.place(key, 2)
	got := decodeRound(t, rec)
	if rec.Code != http.StatusCreated || got.Status != svcorder.RoundUnpaid || got.Due == nil || got.Due.Minor != 44000 {
		t.Fatalf("holding a round: %d %s", rec.Code, rec.Body)
	}
	return got.ID
}

func (p *prepayWorld) pay(code, round string, header map[string]string) *httptest.ResponseRecorder {
	return send(p.handler, http.MethodPost, "/order/api/t/"+code+"/orders/"+round+"/charge", "", header)
}

func (p *prepayWorld) ask(id string) guestChargeResponse {
	rec := send(p.handler, http.MethodGet, "/order/api/t/"+p.code+"/bill/charge/"+id, "", nil)
	var got guestChargeResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	return got
}

// TestAGuestPaysForARoundAndItIsSent: the whole of paying first, as a
// phone sees it. Place Order answers unpaid; the round's charge gives the
// gateway's page; until the gateway says paid the table has no bill; then
// the charge is paid, the round is sent, and the bill shows it settled.
func TestAGuestPaysForARoundAndItIsSent(t *testing.T) {
	p := newPrepayWorld(t, "https://cafe.example")
	round := p.hold(t, "k1")

	rec := p.pay(p.code, round, map[string]string{"Host": "evil.example", "X-Forwarded-Host": "evil.example", "Origin": "https://evil.example"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("starting: %d %s", rec.Code, rec.Body)
	}
	started := decodeCharge(t, rec)
	if !strings.HasPrefix(started.ID, "chg_") || started.Status != charges.Open || started.Amount.Minor != 44000 ||
		started.URL != "https://pay.test/fake_0001" || started.Round != round {
		t.Errorf("charge = %+v", started)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a charge's answer may be cached")
	}
	// The gateway sends the guest back to their table at the venue's own
	// address, whatever host the request claimed.
	asked := p.gw.Started[0]
	if asked.ReturnURL != "https://cafe.example/order/t/"+p.code || asked.Amount.Minor != 44000 || !strings.HasPrefix(asked.Description, "Order at table ") {
		t.Errorf("the gateway was asked for %+v", asked)
	}

	// Tapped again: the same charge, and 200.
	if again := p.pay(p.code, round, nil); again.Code != http.StatusOK || decodeCharge(t, again).ID != started.ID || len(p.gw.Started) != 1 {
		t.Errorf("tapped again: %d %s", again.Code, again.Body)
	}
	if got := p.ask(started.ID); got.Status != charges.Open || got.URL == "" || got.Round != round {
		t.Errorf("before paying: %+v", got)
	}
	if bill, _ := p.bill(t); bill.Open {
		t.Fatal("an unpaid round is on the bill")
	}

	p.gw.Pay("fake_0001", gateway.UPI)
	if got := p.ask(started.ID); got.Status != charges.Paid || got.URL != "" || got.Round != round {
		t.Fatalf("after paying: %+v", got)
	}
	bill, _ := p.bill(t)
	if !bill.Open || len(bill.Lines) != 1 || bill.Total.Minor != 44000 || bill.Paid.Minor != 44000 || bill.BalanceDue.Minor != 0 {
		t.Errorf("the bill: open %v, %d lines, total %v, paid %v, owing %v", bill.Open, len(bill.Lines), bill.Total, bill.Paid, bill.BalanceDue)
	}
	// The phone asking after its round with its key is told it is sent.
	if told := decodeRound(t, p.place("k1", 2)); told.Status != svcorder.RoundSent || told.Due != nil {
		t.Errorf("the round, asked after: %+v", told)
	}
	// And there is nothing more to pay for it.
	if done := p.pay(p.code, round, nil); done.Code != http.StatusConflict || errorOf(t, done).Code != CodeNothingToPay {
		t.Errorf("paying for it again: %d %s", done.Code, done.Body)
	}
}

func TestPayingForARoundIsRefused(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, p *prepayWorld)
		// path is the table's code and the round, given the held round's id.
		path   func(t *testing.T, p *prepayWorld, round string) (code, id string)
		status int
		code   ErrorCode
	}{
		{name: "a round that is not there", path: func(_ *testing.T, p *prepayWorld, _ string) (string, string) { return p.code, "rnd_NOTTHERE" },
			status: http.StatusNotFound, code: CodeRequestNotFound},
		{name: "another table's round", path: func(t *testing.T, p *prepayWorld, round string) (string, string) { return p.tableCode(t, 5), round },
			status: http.StatusNotFound, code: CodeRequestNotFound},
		{name: "with the venue shut", prepare: func(t *testing.T, p *prepayWorld) {
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
		}, status: http.StatusConflict, code: CodeNoOpenShift},
		{name: "with the gateway down", prepare: func(_ *testing.T, p *prepayWorld) { p.gw.Down(true) },
			status: http.StatusServiceUnavailable, code: CodeGatewayUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPrepayWorld(t, "")
			round := p.hold(t, "k1")
			if tt.prepare != nil {
				tt.prepare(t, p)
			}
			code, id := p.code, round
			if tt.path != nil {
				code, id = tt.path(t, p, round)
			}
			rec := p.pay(code, id, nil)
			if rec.Code != tt.status || errorOf(t, rec).Code != tt.code {
				t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body, tt.status, tt.code)
			}
			if strings.Contains(rec.Body.String(), "fake_") || strings.Contains(rec.Body.String(), "pay.test") {
				t.Errorf("a refusal names the gateway's own: %s", rec.Body)
			}
		})
	}
}

// TestARequestIsNotPaidFor: where guests do not pay first a round is a
// request for staff, and has no charge to start.
func TestARequestIsNotPaidFor(t *testing.T) {
	c := newChargeWorld(t, "")
	other := "/order/api/t/" + c.tableCode(t, 5)
	rec := send(c.handler, http.MethodPost, other+"/orders", `{"lines":[{"item_id":`+itoa(c.white)+`,"modifier_ids":[`+itoa(c.oat)+`],"qty":1}]}`,
		map[string]string{"Content-Type": "application/json", HeaderIdempotencyKey: "r1"})
	request := decodeRound(t, rec)
	if rec.Code != http.StatusCreated || request.Status != svcorder.RoundWaiting {
		t.Fatalf("request: %d %s", rec.Code, rec.Body)
	}
	paid := send(c.handler, http.MethodPost, other+"/orders/"+request.ID+"/charge", "", nil)
	if paid.Code != http.StatusNotFound || errorOf(t, paid).Code != CodeRequestNotFound || len(c.gw.Started) != 0 {
		t.Errorf("paying for a request: %d %s", paid.Code, paid.Body)
	}
}

// TestWithNoGatewayARoundHasNoCharge: the route is not there.
func TestWithNoGatewayARoundHasNoCharge(t *testing.T) {
	o := newOrderingWorld(t)
	rec := send(o.handler, http.MethodPost, "/order/api/t/"+o.code+"/orders/rnd_ANYTHING/charge", "", nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want the route absent", rec.Code)
	}
}

package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/gateway"
	"github.com/byjoellobo/mez/internal/gateway/gatewaytest"
	"github.com/byjoellobo/mez/internal/service/charges"
	svcorder "github.com/byjoellobo/mez/internal/service/order"
)

// chargeWorld is orderingWorld at a venue with a payment gateway, a fake
// one, and table 4 seated with two oat flat whites: 440.00 owed.
type chargeWorld struct {
	*orderingWorld
	gw      *gatewaytest.Fake
	charges *charges.Service
	orderID int64
}

func newChargeWorld(t *testing.T, publicURL string) *chargeWorld {
	t.Helper()
	o := newOrderingWorld(t)
	gw := gatewaytest.New()
	svc, err := charges.New(charges.Options{
		DB: o.db, Orders: o.orders,
		Gateways: func(context.Context, int64) (gateway.Gateway, error) { return gw, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	o.rebuild(t, WithMenus(o.db), WithGuests(o.codes, "/order/"), WithGuestOrders(o.orders), WithCharges(svc, publicURL),
		WithStatic(map[string]fs.FS{"/order/": app}))
	seated := o.seat(t)
	if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	return &chargeWorld{orderingWorld: o, gw: gw, charges: svc, orderID: seated.OrderID}
}

func (c *chargeWorld) start() *httptest.ResponseRecorder {
	return c.phoneSend(http.MethodPost, "/order/api/t/"+c.code+"/bill/charge", "", nil)
}

func (c *chargeWorld) ask(id string) *httptest.ResponseRecorder {
	return c.phoneSend(http.MethodGet, "/order/api/t/"+c.code+"/bill/charge/"+id, "", nil)
}

func decodeCharge(t *testing.T, rec *httptest.ResponseRecorder) guestChargeResponse {
	t.Helper()
	var got guestChargeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	return got
}

// TestGuestPaysOnline: the bill offers it; the phone is sent to the
// gateway's page; and only once the gateway says paid is the bill paid.
func TestGuestPaysOnline(t *testing.T) {
	c := newChargeWorld(t, "https://cafe.example")
	if bill, _ := c.bill(t); !bill.Online || bill.BalanceDue.Minor != 44000 {
		t.Fatalf("the bill does not offer paying online: %+v", bill)
	}

	rec := c.start()
	if rec.Code != http.StatusCreated {
		t.Fatalf("starting: %d %s", rec.Code, rec.Body)
	}
	started := decodeCharge(t, rec)
	if !strings.HasPrefix(started.ID, "chg_") || started.Status != charges.Open || started.Amount.Minor != 44000 || started.URL != "https://pay.test/fake_0001" {
		t.Errorf("charge = %+v", started)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("a charge's answer may be cached")
	}
	// The gateway was told to send the guest back to their own table, at
	// the venue's own address, with nothing after it for a gateway to break.
	asked := c.gw.Started[0]
	if asked.ReturnURL != "https://cafe.example/order/t/"+c.code || asked.Amount.Minor != 44000 || !strings.HasPrefix(asked.Description, "Table 4") {
		t.Errorf("the gateway was asked for %+v", asked)
	}
	// Tapped again: the same charge, and 200.
	if again := c.start(); again.Code != http.StatusOK || decodeCharge(t, again).ID != started.ID || len(c.gw.Started) != 1 {
		t.Errorf("starting twice: %d %s", again.Code, again.Body)
	}

	// Asked before paying: open, and the bill owes what it did.
	if got := decodeCharge(t, c.ask(started.ID)); got.Status != charges.Open || got.URL == "" {
		t.Errorf("before paying: %+v", got)
	}
	if bill, _ := c.bill(t); bill.Paid.Minor != 0 {
		t.Fatal("an unpaid charge is on the bill")
	}

	c.gw.Pay("fake_0001", gateway.UPI)
	paid := c.ask(started.ID)
	if got := decodeCharge(t, paid); paid.Code != http.StatusOK || got.Status != charges.Paid || got.URL != "" {
		t.Fatalf("after paying: %d %+v", paid.Code, got)
	}
	bill, _ := c.bill(t)
	if bill.Paid.Minor != 44000 || bill.BalanceDue.Minor != 0 || bill.Online || bill.UPI != nil {
		t.Errorf("the bill: paid %v, due %v, online %v", bill.Paid, bill.BalanceDue, bill.Online)
	}
	res, err := c.orders.Order(context.Background(), 1, c.orderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Tenders) != 1 || res.State.Tenders[0].Method != order.PaymentUPI || !res.State.Invoiced {
		t.Errorf("tenders %+v, invoiced %v", res.State.Tenders, res.State.Invoiced)
	}
	// Nothing left to pay.
	if rec := c.start(); rec.Code != http.StatusConflict || errorOf(t, rec).Code != CodeNothingToPay {
		t.Errorf("starting on a settled bill: %d %s", rec.Code, rec.Body)
	}
}

// TestTheReturnAddressIsNeverTheRequests: whatever host a request claims,
// the gateway is given the venue's own address, or none.
func TestTheReturnAddressIsNeverTheRequests(t *testing.T) {
	for _, public := range []string{"https://cafe.example", ""} {
		c := newChargeWorld(t, public)
		rec := c.phoneSend(http.MethodPost, "/order/api/t/"+c.code+"/bill/charge", "", map[string]string{
			"Host": "evil.example", "X-Forwarded-Host": "evil.example", "X-Forwarded-Proto": "https", "Origin": "https://evil.example", "Referer": "https://evil.example/x",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("starting: %d %s", rec.Code, rec.Body)
		}
		got := c.gw.Started[0].ReturnURL
		if strings.Contains(got, "evil") {
			t.Fatalf("the gateway was told to send the guest to %q", got)
		}
		if public == "" && got != "" {
			t.Errorf("with no address set, the gateway was given %q", got)
		}
	}
}

func TestGuestChargeWhenItFails(t *testing.T) {
	c := newChargeWorld(t, "")
	id := decodeCharge(t, c.start()).ID
	c.gw.Fail("fake_0001")
	if got := decodeCharge(t, c.ask(id)); got.Status != charges.Failed || got.URL != "" {
		t.Errorf("a failed charge: %+v", got)
	}
	if bill, _ := c.bill(t); bill.Paid.Minor != 0 || !bill.Online {
		t.Errorf("after a failure the bill: paid %v, online %v", bill.Paid, bill.Online)
	}
}

// TestGuestChargeTakenAndNotOnTheBill: the table paid cash meanwhile. The
// phone is told the truth, and nothing is forced onto the bill.
func TestGuestChargeTakenAndNotOnTheBill(t *testing.T) {
	c := newChargeWorld(t, "")
	id := decodeCharge(t, c.start()).ID
	if _, err := c.orders.TenderPayment(context.Background(), cashier, c.orderID, cashOf(44000)); err != nil {
		t.Fatal(err)
	}
	c.gw.Pay("fake_0001", gateway.Card)
	rec := c.ask(id)
	if got := decodeCharge(t, rec); got.Status != charges.Unapplied {
		t.Fatalf("charge = %+v", got)
	}
	// Why is the venue's business: the phone is told to ask staff.
	if strings.Contains(rec.Body.String(), "problem") || strings.Contains(rec.Body.String(), "another way") {
		t.Errorf("the phone was told the venue's reasons: %s", rec.Body)
	}
}

// TestGuestChargeWithTheGatewayDown: starting says so in a guest's words;
// asking about one already started answers it as it was.
func TestGuestChargeWithTheGatewayDown(t *testing.T) {
	c := newChargeWorld(t, "")
	id := decodeCharge(t, c.start()).ID
	c.gw.Pay("fake_0001", gateway.UPI)
	c.gw.Down(true)

	asked := c.ask(id)
	if got := decodeCharge(t, asked); asked.Code != http.StatusOK || got.Status != charges.Open {
		t.Errorf("asking with the gateway down: %d %+v", asked.Code, got)
	}
	// Another table tries to start one.
	other := "/order/api/t/" + c.tableCode(t, 5)
	if _, err := c.orders.Open(context.Background(), cashier, openAt(5)); err != nil {
		t.Fatal(err)
	}
	if rec := c.phoneSend(http.MethodPost, other+"/orders", `{"lines":[{"item_id":`+itoa(c.white)+`,"modifier_ids":[`+itoa(c.oat)+`],"qty":1}]}`,
		map[string]string{HeaderIdempotencyKey: "t5", "Content-Type": "application/json"}); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	rec := c.phoneSend(http.MethodPost, other+"/bill/charge", "", nil)
	got := errorOf(t, rec)
	if rec.Code != http.StatusServiceUnavailable || got.Code != CodeGatewayUnavailable || !strings.Contains(got.Message, "pay a member of staff") {
		t.Errorf("starting with the gateway down: %d %+v", rec.Code, got)
	}

	c.gw.Down(false)
	if got := decodeCharge(t, c.ask(id)); got.Status != charges.Paid {
		t.Errorf("once it is back: %+v", got)
	}
}

// TestAChargeIsItsTablesOwn: another table's code does not read it.
func TestAChargeIsItsTablesOwn(t *testing.T) {
	c := newChargeWorld(t, "")
	id := decodeCharge(t, c.start()).ID
	other := c.phoneSend(http.MethodGet, "/order/api/t/"+c.tableCode(t, 5)+"/bill/charge/"+id, "", nil)
	if other.Code != http.StatusNotFound || errorOf(t, other).Code != CodeChargeNotFound {
		t.Errorf("table 5 asking about table 4's charge: %d %s", other.Code, other.Body)
	}
	if c.gw.Asked != 0 {
		t.Error("the gateway was asked on another table's say-so")
	}
	if rec := c.ask("chg_NOPE0000"); rec.Code != http.StatusNotFound {
		t.Errorf("a charge that does not exist: %d", rec.Code)
	}
}

// TestNoGatewayNoOnlinePayment: the normal venue. The bill does not offer
// it and the routes are not there.
func TestNoGatewayNoOnlinePayment(t *testing.T) {
	o := newOrderingWorld(t)
	o.seat(t)
	if rec := o.place("k1", 1); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	bill, body := o.bill(t)
	if bill.Online || strings.Contains(body, "online") {
		t.Errorf("a venue with no gateway offers paying online: %s", body)
	}
	rec := o.phoneSend(http.MethodPost, "/order/api/t/"+o.code+"/bill/charge", "", nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("the route is there: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "<title>") {
		t.Error("the app's page answered an API call")
	}
}

func cashOf(minor int64) svcorder.TenderPayment {
	return svcorder.TenderPayment{Method: order.PaymentCash, Amount: money.Amount{Minor: minor, Currency: "INR"}}
}

func openAt(table int64) svcorder.OpenOrder {
	return svcorder.OpenOrder{Channel: order.ChannelCounter, TableID: table, Covers: 2}
}

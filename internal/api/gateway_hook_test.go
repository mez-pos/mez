package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mez-pos/mez/internal/gateway"
	"github.com/mez-pos/mez/internal/gateway/gatewaytest"
	"github.com/mez-pos/mez/internal/service/charges"
)

// hookTeller is a gateway.Notified for the API's tests: a call is the
// gateway's when it carries the right header, and its body is the id of
// the charge it is about.
type hookTeller struct{}

func (hookTeller) Name() string { return "test" }

func (hookTeller) Notice(header http.Header, body []byte) (string, error) {
	if header.Get("X-Test-Signature") != "good" {
		return "", fmt.Errorf("%w: not signed", gateway.ErrRefused)
	}
	return strings.TrimSpace(string(body)), nil
}

// hookWorld is chargeWorld whose gateway can call the venue.
func newHookWorld(t *testing.T, notices gateway.Notified) *chargeWorld {
	t.Helper()
	o := newOrderingWorld(t)
	gw := gatewaytest.New()
	svc, err := charges.New(charges.Options{
		DB: o.db, Orders: o.orders, Notices: notices,
		Gateways: func(context.Context, int64) (gateway.Gateway, error) { return gw, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	o.rebuild(t, WithMenus(o.db), WithGuests(o.codes, "/order/"), WithGuestOrders(o.orders), WithCharges(svc, ""),
		WithStatic(map[string]fs.FS{"/order/": app}))
	seated := o.seat(t)
	if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	return &chargeWorld{orderingWorld: o, gw: gw, charges: svc, orderID: seated.OrderID}
}

func hook(c *chargeWorld, path, signature, body string) *httptest.ResponseRecorder {
	h := map[string]string{"Content-Type": "application/json"}
	if signature != "" {
		h["X-Test-Signature"] = signature
	}
	return send(c.handler, http.MethodPost, path, body, h)
}

// TestGatewayHook: the gateway calls, with no table code and no session,
// and the payment lands. A call that is not the gateway's is refused and
// changes nothing.
func TestGatewayHook(t *testing.T) {
	c := newHookWorld(t, hookTeller{})
	id := decodeCharge(t, c.start()).ID
	c.gw.Pay("fake_0001", gateway.UPI)

	forged := hook(c, "/order/api/hook/test", "", "fake_0001")
	if forged.Code != http.StatusUnauthorized || errorOf(t, forged).Code != CodeUnauthenticated {
		t.Fatalf("a call with no signature: %d %s", forged.Code, forged.Body)
	}
	if bad := hook(c, "/order/api/hook/test", "bad", "fake_0001"); bad.Code != http.StatusUnauthorized {
		t.Errorf("a call with a wrong signature: %d", bad.Code)
	}
	if got, _ := c.charges.Charge(context.Background(), 1, id); got.Status != charges.Open || c.gw.Asked != 0 {
		t.Fatalf("a forged call moved the charge to %s, and asked the gateway %d times", got.Status, c.gw.Asked)
	}

	rec := hook(c, "/order/api/hook/test", "good", "fake_0001")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("the gateway's call: %d %s", rec.Code, rec.Body)
	}
	if got, _ := c.charges.Charge(context.Background(), 1, id); got.Status != charges.Paid {
		t.Errorf("charge is %s", got.Status)
	}
	if bill, _ := c.bill(t); bill.Paid.Minor != 44000 || bill.BalanceDue.Minor != 0 {
		t.Errorf("the bill: paid %v, due %v", bill.Paid, bill.BalanceDue)
	}

	// Sent again, about a charge nobody knows, or about nothing: taken.
	for _, body := range []string{"fake_0001", "fake_9999", ""} {
		if rec := hook(c, "/order/api/hook/test", "good", body); rec.Code != http.StatusNoContent {
			t.Errorf("a notice about %q: %d %s", body, rec.Code, rec.Body)
		}
	}
	res, err := c.orders.Order(context.Background(), 1, c.orderID)
	if err != nil || len(res.State.Tenders) != 1 {
		t.Errorf("tenders %+v, %v: the notice sent twice paid twice", res.State.Tenders, err)
	}
}

func TestGatewayHookIsPostOnlyAndBounded(t *testing.T) {
	c := newHookWorld(t, hookTeller{})
	if rec := send(c.handler, http.MethodGet, "/order/api/hook/test", "", nil); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d", rec.Code)
	}
	big := hook(c, "/order/api/hook/test", "good", strings.Repeat("x", maxHookBytes+1))
	if big.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("a notice over the limit: %d", big.Code)
	}
}

// TestGatewayHookWithNoSecretSet: a venue that takes no notices answers
// that there is nothing here, signed or not.
func TestGatewayHookWithNoSecretSet(t *testing.T) {
	c := newHookWorld(t, nil)
	c.start()
	c.gw.Pay("fake_0001", gateway.UPI)
	if rec := hook(c, "/order/api/hook/test", "good", "fake_0001"); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, body %s", rec.Code, rec.Body)
	}
	if c.gw.Asked != 0 {
		t.Error("the gateway was asked")
	}
}

// TestNoGatewayNoHook: the normal venue has no such route.
func TestNoGatewayNoHook(t *testing.T) {
	o := newOrderingWorld(t)
	rec := send(o.handler, http.MethodPost, "/order/api/hook/razorpay", "{}", nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("the route is there: %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "<title>") {
		t.Error("the app's page answered a gateway's call")
	}
}

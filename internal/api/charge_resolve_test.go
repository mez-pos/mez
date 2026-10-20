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

	"github.com/mez-pos/mez/internal/gateway"
)

// unappliedWorld is chargeWorld with staff signed in at the terminal, and
// a charge the bill could not take: table 4 paid cash while its guest was
// on the gateway's page.
type unappliedWorld struct {
	*chargeWorld
	staffToken string
	chargeID   string
}

func newUnappliedWorld(t *testing.T) *unappliedWorld {
	t.Helper()
	c := newChargeWorld(t, "")
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	c.rebuild(t, WithMenus(c.db), WithGuests(c.codes, "/order/"), WithGuestOrders(c.orders), WithGuestRequests(c.orders),
		WithCharges(c.charges, ""), WithStatic(map[string]fs.FS{"/order/": app}))
	id := decodeCharge(t, c.start()).ID
	if _, err := c.orders.TenderPayment(context.Background(), cashier, c.orderID, cashOf(44000)); err != nil {
		t.Fatal(err)
	}
	c.gw.Pay("fake_0001", gateway.UPI)
	if got := decodeCharge(t, c.ask(id)); got.Status != "unapplied" {
		t.Fatalf("charge = %+v, want it unapplied", got)
	}
	return &unappliedWorld{chargeWorld: c, staffToken: c.signIn(t, "2468").Token, chargeID: id}
}

func (u *unappliedWorld) staff(method, path, body string) *httptest.ResponseRecorder {
	h := u.asStaff(u.staffToken)
	h["Content-Type"] = "application/json"
	return send(u.handler, method, path, body, h)
}

func (u *unappliedWorld) list(t *testing.T) []unappliedChargeResponse {
	t.Helper()
	rec := u.staff(http.MethodGet, "/api/v1/guest-requests", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing: %d %s", rec.Code, rec.Body)
	}
	var got guestRequestsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body, err)
	}
	if got.Unapplied == nil {
		t.Fatalf("the list is missing, not empty: %s", rec.Body)
	}
	return got.Unapplied
}

// TestUnappliedChargesReachStaff: everything a person needs to find the
// payment at the gateway and decide what to do with it.
func TestUnappliedChargesReachStaff(t *testing.T) {
	u := newUnappliedWorld(t)
	got := u.list(t)
	if len(got) != 1 {
		t.Fatalf("%d unapplied charges listed, want 1", len(got))
	}
	c := got[0]
	if c.ID != u.chargeID || c.TableID != 4 || c.TableName != "4" || !strings.HasPrefix(c.OrderID, "ord_") ||
		c.Gateway != "test" || c.GatewayID != "fake_0001" || c.Reference != "pay_fake_0001" || c.Method != "upi" ||
		c.Amount.Minor != 44000 || c.Problem == "" || c.At.IsZero() {
		t.Errorf("unapplied charge = %+v", c)
	}
}

func TestResolvingAnUnappliedCharge(t *testing.T) {
	u := newUnappliedWorld(t)
	path := "/api/v1/charges/" + u.chargeID + "/resolve"

	// Nobody, a guest under their own mount, and a body that says nothing.
	if rec := send(u.handler, http.MethodPost, path, `{"how":"refunded"}`, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("nobody resolves it: %d", rec.Code)
	}
	if rec := send(u.handler, http.MethodPost, "/order"+path, `{"how":"refunded"}`, nil); rec.Code != http.StatusNotFound {
		t.Errorf("a guest reaches for it: %d", rec.Code)
	}
	for _, body := range []string{`{}`, `{"how":""}`, `{"how":"forgiven"}`} {
		rec := u.staff(http.MethodPost, path, body)
		if got := errorOf(t, rec); rec.Code != http.StatusBadRequest || got.Code != CodeInvalidRequest || got.Field != "how" {
			t.Errorf("%s: %d %+v", body, rec.Code, got)
		}
	}
	if len(u.list(t)) != 1 {
		t.Fatal("the charge was resolved by a request that should not have")
	}

	if rec := u.staff(http.MethodPost, path, `{"how":"refunded"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("resolving: %d %s", rec.Code, rec.Body)
	}
	if got := u.list(t); len(got) != 0 {
		t.Errorf("still listed: %+v", got)
	}
	// Said twice, the first stands and the second is told so.
	again := u.staff(http.MethodPost, path, `{"how":"recorded"}`)
	if again.Code != http.StatusConflict || errorOf(t, again).Code != CodeChargeNotWaiting {
		t.Errorf("resolved twice: %d %s", again.Code, again.Body)
	}
	unknown := u.staff(http.MethodPost, "/api/v1/charges/chg_NOPE0000/resolve", `{"how":"refunded"}`)
	if unknown.Code != http.StatusNotFound || errorOf(t, unknown).Code != CodeChargeNotFound {
		t.Errorf("a charge that does not exist: %d %s", unknown.Code, unknown.Body)
	}
}

// TestNoGatewayNoUnappliedList: the normal venue answers an empty list and
// has no route to resolve anything.
func TestNoGatewayNoUnappliedList(t *testing.T) {
	w := newRequestsWorld(t)
	rec := w.staffCall(http.MethodGet, "/api/v1/guest-requests")
	var got guestRequestsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Unapplied == nil || len(got.Unapplied) != 0 {
		t.Errorf("unapplied = %#v, want an empty list", got.Unapplied)
	}
	if rec := w.staffCall(http.MethodPost, "/api/v1/charges/chg_NOPE0000/resolve"); rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("the route is there: %d", rec.Code)
	}
}

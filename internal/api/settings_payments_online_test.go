package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"github.com/mez-pos/mez/internal/gateway"
	"github.com/mez-pos/mez/internal/service/settings"
)

// onlineWorld is chargeWorld with the settings mounted and the owner
// signed in: a venue with a gateway, read from the back office.
func newOnlineWorld(t *testing.T, publicURL string) (*chargeWorld, map[string]string) {
	t.Helper()
	c := newChargeWorld(t, publicURL)
	svc, err := settings.New(settings.Options{DB: c.db})
	if err != nil {
		t.Fatal(err)
	}
	app := fstest.MapFS{"index.html": {Data: []byte("<title>the guest app</title>")}}
	c.rebuild(t, WithMenus(c.db), WithGuests(c.codes, "/order/"), WithGuestOrders(c.orders), WithSettings(svc),
		WithCharges(c.charges, publicURL), WithStatic(map[string]fs.FS{"/order/": app}))
	return c, c.asStaff(c.signInAs(t, "stf_PRIYA001", "2468").Token)
}

func readPayments(t *testing.T, h http.Handler, staff map[string]string) (paymentSettingsResponse, map[string]json.RawMessage) {
	t.Helper()
	rec := send(h, http.MethodGet, "/api/v1/settings/payments", "", staff)
	if rec.Code != http.StatusOK {
		t.Fatalf("reading the payment settings: %d %s", rec.Code, rec.Body)
	}
	var got paymentSettingsResponse
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	return got, raw
}

// TestPaymentSettingsSayWhichGatewayIsSet: its name, where guests are sent
// back, whether its calls are taken, and when it last took a payment. The
// answer has those four things about it and nothing else: no key has
// anywhere to be.
func TestPaymentSettingsSayWhichGatewayIsSet(t *testing.T) {
	c, owner := newOnlineWorld(t, "https://cafe.example/")
	got, raw := readPayments(t, c.handler, owner)
	if !got.CanCharge || got.Online == nil {
		t.Fatalf("payments = %+v", got)
	}
	on := *got.Online
	if on.Gateway != "test" || on.Webhooks || on.ReturnURL != "https://cafe.example/order/" || on.LastPaidAt != nil {
		t.Errorf("online = %+v", on)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw["online"], &fields); err != nil {
		t.Fatal(err)
	}
	for k := range fields {
		switch k {
		case "gateway", "webhooks", "return_url", "last_paid_at":
		default:
			t.Errorf("online carries %q", k)
		}
	}

	// A guest pays: the settings now say when.
	before := time.Now().Add(-time.Minute)
	id := decodeCharge(t, c.start()).ID
	c.gw.Pay("fake_0001", gateway.Card)
	if paid := decodeCharge(t, c.ask(id)); paid.Status != "paid" {
		t.Fatalf("charge = %+v", paid)
	}
	got, _ = readPayments(t, c.handler, owner)
	if got.Online == nil || got.Online.LastPaidAt == nil || got.Online.LastPaidAt.Before(before) {
		t.Errorf("after a payment: %+v", got.Online)
	}
	// Saving the settings answers with the same.
	rec := send(c.handler, http.MethodPut, "/api/v1/settings/payments", `{"upi_address":"","upi_payee":""}`,
		with(owner, "Content-Type", "application/json"))
	var saved paymentSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || rec.Code != http.StatusOK || saved.Online == nil || saved.Online.Gateway != "test" {
		t.Errorf("after a save: %d %s", rec.Code, rec.Body)
	}
}

// TestPaymentSettingsWithNoAddressSet: the gateway is still named, and
// where a guest is sent back is empty.
func TestPaymentSettingsWithNoAddressSet(t *testing.T) {
	c, owner := newOnlineWorld(t, "")
	got, _ := readPayments(t, c.handler, owner)
	if got.Online == nil || got.Online.Gateway != "test" || got.Online.ReturnURL != "" {
		t.Errorf("online = %+v", got.Online)
	}
}

// TestPaymentSettingsWithNoGateway: nothing is said of paying online.
func TestPaymentSettingsWithNoGateway(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)
	got, raw := readPayments(t, w.handler, owner)
	if got.CanCharge || got.Online != nil {
		t.Errorf("payments = %+v", got)
	}
	if _, there := raw["online"]; there {
		t.Error("the answer has an online part with no gateway set")
	}
}

func with(h map[string]string, k, v string) map[string]string {
	out := map[string]string{k: v}
	for a, b := range h {
		out[a] = b
	}
	return out
}

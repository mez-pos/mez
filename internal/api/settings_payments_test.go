package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/service/settings"
)

// TestPaymentSettingsOverHTTP: the owner reads where the venue is paid,
// changes it, and a refusal comes back with its code and field.
func TestPaymentSettingsOverHTTP(t *testing.T) {
	w := newTill(t)
	svc, err := settings.New(settings.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithSettings(svc))
	owner := w.asStaff(w.signInAs(t, "stf_PRIYA001", "2468").Token)

	call := func(method, body string) (int, paymentSettingsResponse, Error) {
		t.Helper()
		rec := send(w.handler, method, "/api/v1/settings/payments", body, owner)
		if rec.Code >= 400 {
			return rec.Code, paymentSettingsResponse{}, errorOf(t, rec)
		}
		var out paymentSettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v: %s", method, err, rec.Body)
		}
		return rec.Code, out, Error{}
	}

	status, out, _ := call(http.MethodGet, "")
	if status != http.StatusOK || out.UPIAddress != "" || out.VenueName == "" || out.Currency != "INR" || out.MaxPayee != 60 {
		t.Fatalf("a new venue: %d %+v", status, out)
	}

	body := `{"upi_address":"cafe@okbank","upi_payee":"Filter Coffee"}`
	if status, out, _ = call(http.MethodPut, body); status != http.StatusOK || out.UPIAddress != "cafe@okbank" || out.UPIPayee != "Filter Coffee" {
		t.Fatalf("saving: %d %+v", status, out)
	}

	status, _, refused := call(http.MethodPut, `{"upi_address":"not an address","upi_payee":""}`)
	if status != http.StatusUnprocessableEntity || refused.Code != CodeInvalidSetting || refused.Field != "upi_address" {
		t.Errorf("a bad address: %d %+v", status, refused)
	}
	if _, out, _ = call(http.MethodGet, ""); out.UPIAddress != "cafe@okbank" {
		t.Errorf("a refused update changed the address: %+v", out)
	}

	t.Run("a cashier cannot read or change it", func(t *testing.T) {
		cashier := w.asStaff(w.signIn(t, "2468").Token)
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			if rec := send(w.handler, method, "/api/v1/settings/payments", body, cashier); rec.Code != http.StatusForbidden {
				t.Errorf("%s: %d", method, rec.Code)
			}
		}
	})
	if rec := send(w.handler, http.MethodDelete, "/api/v1/settings/payments", "", owner); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("DELETE: %d", rec.Code)
	}
}

// TestGuestBillOffersUPI: with an address set and something owed, the bill
// carries a link for exactly what is left. With none, or nothing owed, it
// carries no link at all.
func TestGuestBillOffersUPI(t *testing.T) {
	o := newOrderingWorld(t)
	o.seat(t)
	if rec := o.place("k1", 2); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	if got, body := o.bill(t); got.UPI != nil || strings.Contains(body, "upi") {
		t.Fatalf("a venue with no address offers UPI: %s", body)
	}

	svc, err := settings.New(settings.Options{DB: o.db})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := svc.UpdatePayments(ctx, 1, settings.PaymentsInput{UPIAddress: "cafe@okbank"}); err != nil {
		t.Fatal(err)
	}

	got, _ := o.bill(t)
	if got.UPI == nil {
		t.Fatal("no UPI on a bill with money owed and an address set")
	}
	// 2 x (180 + 40) = 440.00, untaxed here, and all of it owed.
	if got.UPI.Address != "cafe@okbank" || got.UPI.Payee == "" || got.UPI.Amount != got.BalanceDue || got.BalanceDue.Minor != 44000 {
		t.Errorf("upi = %+v, balance %v", got.UPI, got.BalanceDue)
	}
	u, err := url.Parse(got.UPI.Link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "upi" || q.Get("pa") != "cafe@okbank" || q.Get("am") != "440.00" || q.Get("cu") != "INR" || q.Get("tn") != "Table 4" || q.Get("pn") != got.UPI.Payee {
		t.Errorf("link = %s", got.UPI.Link)
	}

	// The payee the owner chose is the one in the link.
	if err := svc.UpdatePayments(ctx, 1, settings.PaymentsInput{UPIAddress: "cafe@okbank", UPIPayee: "Filter Coffee"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := o.bill(t); got.UPI == nil || got.UPI.Payee != "Filter Coffee" {
		t.Errorf("payee = %+v", got.UPI)
	}

	// A table with nothing on its bill has nothing to pay.
	other := "/order/api/t/" + o.tableCode(t, 5) + "/bill"
	if _, _, body := o.guest(http.MethodGet, other); strings.Contains(body, `"upi"`) {
		t.Errorf("a bill with nothing owed offers UPI: %s", body)
	}
}

// tableCode is the live code of the table with that id.
func (o *orderingWorld) tableCode(t *testing.T, tableID int64) string {
	t.Helper()
	list, err := o.codes.List(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.TableID == tableID {
			return c.Code
		}
	}
	t.Fatalf("table %d has no code", tableID)
	return ""
}

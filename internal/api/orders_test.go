package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/auth"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// register is a till with an order service behind it: a GST cafe, a paired
// terminal, Asha the cashier signed in, and Priya the owner to authorise
// what Asha cannot do alone.
type register struct {
	*till
	staffToken string
	ownerPIN   string
}

func newRegister(t *testing.T) *register {
	t.Helper()
	w := newTill(t)

	// The venue this till was seeded with is untaxed; give it GST, which is
	// what makes the totals worth asserting.
	ctx := context.Background()
	err := w.db.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.UpdateVenue(ctx, gen.UpdateVenueParams{
			ID: 1, Name: "Filter Coffee House", LegalName: "Filter Coffee House",
			Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4,
			TaxProfile: "gst_in", TaxRegistrationID: "29ABCDE1234F1Z5",
			CashRoundingStepMinor: 1, UpdatedAt: seededAt,
		}); err != nil {
			return err
		}
		for key, value := range map[string]string{
			svcorder.SettingTaxStateCode: "29",
			svcorder.SettingTaxPricing:   "exclusive",
		} {
			if err := q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: key, Value: value, UpdatedAt: seededAt}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("giving the venue GST: %v", err)
	}

	orders, err := svcorder.New(svcorder.Options{DB: w.db})
	if err != nil {
		t.Fatal(err)
	}
	w.rebuild(t, WithOrders(orders))

	r := &register{till: w, ownerPIN: "2468"}
	r.staffToken = w.signIn(t, "2468").Token
	return r
}

// call sends a request as the signed-in cashier.
func (r *register) call(t *testing.T, method, path, body string) (int, commandResponse, Error) {
	t.Helper()
	return r.callWith(t, method, path, body, r.asStaff(r.staffToken))
}

// callKeyed sends a request carrying an idempotency key.
func (r *register) callKeyed(t *testing.T, method, path, body, key string) (int, commandResponse, Error) {
	t.Helper()
	header := r.asStaff(r.staffToken)
	header[HeaderIdempotencyKey] = key
	return r.callWith(t, method, path, body, header)
}

func (r *register) callWith(t *testing.T, method, path, body string, header map[string]string) (int, commandResponse, Error) {
	t.Helper()
	rec := send(r.handler, method, path, body, header)
	if rec.Code >= 400 {
		return rec.Code, commandResponse{}, errorOf(t, rec)
	}
	var out commandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %v: %s", method, path, err, rec.Body)
	}
	return rec.Code, out, Error{}
}

// ok runs a request that must succeed and returns the order it answered.
func (r *register) ok(t *testing.T, method, path, body string) commandResponse {
	t.Helper()
	status, res, apiErr := r.call(t, method, path, body)
	if status >= 400 {
		t.Fatalf("%s %s: %d %+v", method, path, status, apiErr)
	}
	return res
}

// override is a manager's authorisation, as a request body fragment.
func (r *register) override() string {
	return `"authorised_by":"stf_PRIYA001","pin":"` + r.ownerPIN + `"`
}

// openOrder opens one and returns its public ID.
func (r *register) openOrder(t *testing.T) string {
	t.Helper()
	status, res, apiErr := r.call(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter","table_id":4,"covers":2}`)
	if status != http.StatusCreated {
		t.Fatalf("open: %d %+v", status, apiErr)
	}
	return res.Order.ID
}

// TestASaleOverHTTP is the phase's done-when, over the API: open an order,
// add lines, send the kitchen its ticket, split the bill, close it, and find
// the invoice on the order that comes back.
func TestASaleOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)

	res := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":1,"name":"Masala dosa","qty":2,"unit_price":{"minor":12000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)
	if len(res.Order.Lines) != 1 || res.Order.Lines[0].Qty != 2 || len(res.Events) != 1 {
		t.Fatalf("after one line: %+v", res.Order)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":2,"name":"Filter coffee","qty":3,"unit_price":{"minor":4000,"currency":"INR"},"station":"bar","tax_rate":5000,"hsn":"996331"}`)

	sent := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/tickets", `{"station":"kitchen"}`)
	if len(sent.Order.Tickets) != 1 || !sent.Order.Lines[0].Sent {
		t.Errorf("tickets = %+v", sent.Order.Tickets)
	}

	// 360.00 of food, 5% GST added at the till.
	if due := sent.Order.Totals.BalanceDue; due.Minor != 37800 || due.Currency != "INR" {
		t.Fatalf("balance due = %+v, want 378.00", due)
	}
	if sent.Order.Tax == nil || sent.Order.Tax.Tax.Minor != 1800 {
		t.Errorf("tax = %+v", sent.Order.Tax)
	}

	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments",
		`{"method":"cash","amount":{"minor":20000,"currency":"INR"},"tip":{"minor":0,"currency":"INR"}}`)
	paid := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments",
		`{"method":"card","amount":{"minor":17800,"currency":"INR"},"tip":{"minor":2000,"currency":"INR"},"reference":"AUTH 1"}`)
	if !paid.Order.Totals.BalanceDue.IsZero() || paid.Order.Totals.Tips.Minor != 2000 || len(paid.Order.Tenders) != 2 {
		t.Fatalf("after paying: %+v", paid.Order.Totals)
	}

	closed := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/close", `{}`)
	if closed.Order.Status != "closed" || closed.Order.ClosedAt == nil {
		t.Fatalf("closed = %+v", closed.Order)
	}
	if len(closed.Order.Invoices) != 1 || closed.Order.Invoices[0].InvoiceNo != "INV/26-27/000001" {
		t.Errorf("invoices = %+v", closed.Order.Invoices)
	}
	if len(closed.Events) != 2 {
		t.Errorf("close appended %d events, want the invoice and the close", len(closed.Events))
	}

	// And the order reads back the same as the last command answered.
	rec := send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", r.asStaff(r.staffToken))
	var read orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &read); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("read back: %d %s (%v)", rec.Code, rec.Body, err)
	}
	if read.LastSeq != closed.Order.LastSeq || read.Totals != closed.Order.Totals {
		t.Errorf("the read differs from the command's answer")
	}
}

// TestTheRestOfTheCommands covers every remaining endpoint once, including
// the five that need a manager.
func TestTheRestOfTheCommands(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	line := func(n int) string {
		return fmt.Sprintf(`{"item_id":%d,"name":"Dish %d","qty":2,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","course":%d,"tax_rate":5000,"hsn":"996331"}`, n, n, n)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line(1)) // line 2
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line(2)) // line 3
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line(3)) // line 4

	res := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/qty", `{"from":2,"to":1}`)
	if res.Order.Lines[0].Qty != 1 {
		t.Errorf("qty = %d", res.Order.Lines[0].Qty)
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/3/void",
		`{`+r.override()+`,"reason_code":"wrong_item"}`)
	if res.Order.Lines[1].Status != "voided" || res.Order.Lines[1].AuthorisedBy != 2 {
		t.Errorf("voided line = %+v", res.Order.Lines[1])
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/4/comp",
		`{`+r.override()+`,"reason_code":"regular"}`)
	if res.Order.Lines[2].Status != "comped" {
		t.Errorf("comped line = %+v", res.Order.Lines[2])
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/discounts",
		`{`+r.override()+`,"scope":"order","kind":"pct","basis_points":1000,"reason":"regular"}`)
	if len(res.Order.Discounts) != 1 || res.Order.Discounts[0].Value.Minor != 1000 {
		t.Errorf("discounts = %+v", res.Order.Discounts)
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/notes", `{"scope":"line","line_id":2,"text":"no chilli"}`)
	if len(res.Order.Notes) != 1 {
		t.Errorf("notes = %+v", res.Order.Notes)
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/courses/1/fire", ``)
	if !res.Order.Lines[0].CourseFired {
		t.Errorf("course 1 was not fired")
	}

	// Pay it, void that payment, pay again, invoice, close, reopen, refund.
	due := res.Order.Totals.BalanceDue
	pay := fmt.Sprintf(`{"method":"card","amount":{"minor":%d,"currency":"INR"}}`, due.Minor)
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", pay)
	tender := res.Order.Tenders[0].ID
	res = r.ok(t, http.MethodPost, fmt.Sprintf("/api/v1/orders/%s/payments/%d/void", id, tender),
		`{`+r.override()+`,"reason":"wrong card"}`)
	if res.Order.Tenders[0].Status != "voided" {
		t.Errorf("tender = %+v", res.Order.Tenders[0])
	}
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", pay)
	tender = res.Order.Tenders[1].ID

	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/invoice", ``)
	if len(res.Order.Invoices) != 1 {
		t.Fatalf("invoices = %+v", res.Order.Invoices)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/close", ``)
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/reopen",
		`{`+r.override()+`,"reason":"guest was overcharged"}`)
	if res.Order.Status != "reopened" {
		t.Errorf("status = %s", res.Order.Status)
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/void", `{`+r.override()+`,"reason_code":"stale"}`)
	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/refunds",
		fmt.Sprintf(`{%s,"amount":{"minor":1000,"currency":"INR"},"against_payment_id":%d,"reason":"stale"}`, r.override(), tender))
	if len(res.Order.Refunds) != 1 || res.Order.Totals.Refunded.Minor != 1000 {
		t.Errorf("refunds = %+v", res.Order.Refunds)
	}
}

// TestEveryErrorShape is the other half of the contract: what a client is
// told when the bill will not allow something.
func TestEveryErrorShape(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":1,"name":"Dosa","qty":1,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
		code   ErrorCode
	}{
		{"an order that does not exist", http.MethodGet, "/api/v1/orders/ord_NOSUCH01", "", http.StatusNotFound, CodeOrderNotFound},
		{"a command on an order that does not exist", http.MethodPost, "/api/v1/orders/ord_NOSUCH01/notes", `{"scope":"order","text":"x"}`, http.StatusNotFound, CodeOrderNotFound},
		{"a line that does not exist", http.MethodPost, "/api/v1/orders/" + id + "/lines/99/void", `{` + r.override() + `,"reason_code":"x"}`, http.StatusConflict, CodeLineNotFound},
		{"a quantity that changed underneath", http.MethodPost, "/api/v1/orders/" + id + "/lines/2/qty", `{"from":9,"to":1}`, http.StatusConflict, CodeQtyMismatch},
		{"a tender larger than the bill", http.MethodPost, "/api/v1/orders/" + id + "/payments", `{"method":"cash","amount":{"minor":99999,"currency":"INR"}}`, http.StatusConflict, CodeOverTender},
		{"an invoice before the bill is settled", http.MethodPost, "/api/v1/orders/" + id + "/invoice", ``, http.StatusConflict, CodeBalanceDue},
		{"a course fired twice", http.MethodPost, "/api/v1/orders/" + id + "/courses/1/fire", ``, http.StatusConflict, CodeCourseAlreadyFired},
		{"a ticket for a station with nothing waiting", http.MethodPost, "/api/v1/orders/" + id + "/tickets", `{"station":"bar"}`, http.StatusConflict, CodeNothingToSend},
		{"a payment in another currency", http.MethodPost, "/api/v1/orders/" + id + "/payments", `{"method":"cash","amount":{"minor":100,"currency":"GBP"}}`, http.StatusUnprocessableEntity, CodeInvalidPayload},
		{"a line with no name", http.MethodPost, "/api/v1/orders/" + id + "/lines", `{"item_id":1,"qty":1,"unit_price":{"minor":100,"currency":"INR"}}`, http.StatusUnprocessableEntity, CodeInvalidPayload},
		{"a line id that is not a number", http.MethodPost, "/api/v1/orders/" + id + "/lines/abc/qty", `{"from":1,"to":2}`, http.StatusBadRequest, CodeInvalidRequest},
		{"a body that is not JSON", http.MethodPost, "/api/v1/orders/" + id + "/notes", `{`, http.StatusBadRequest, CodeInvalidRequest},
		{"a field the endpoint does not take", http.MethodPost, "/api/v1/orders/" + id + "/notes", `{"scope":"order","text":"x","colour":"red"}`, http.StatusBadRequest, CodeInvalidRequest},
		{"a reopen of an order that is open", http.MethodPost, "/api/v1/orders/" + id + "/reopen", `{` + r.override() + `,"reason":"x"}`, http.StatusConflict, CodeOrderNotClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The course fired twice needs to have been fired once.
			if tt.code == CodeCourseAlreadyFired {
				r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/courses/1/fire", ``)
			}
			status, _, apiErr := r.call(t, tt.method, tt.path, tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Fatalf("%d %s, want %d %s", status, apiErr.Code, tt.status, tt.code)
			}
			if apiErr.Message == "" {
				t.Error("an error with no message for the cashier to read")
			}
		})
	}
}

// TestOrderRoutesAreGuarded checks the chain in front of every order route.
func TestOrderRoutesAreGuarded(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)

	t.Run("a device with nobody signed in", func(t *testing.T) {
		rec := send(r.handler, http.MethodPost, "/api/v1/orders", `{"channel":"counter"}`, r.asDevice())
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeSignInRequired {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("no device at all", func(t *testing.T) {
		rec := send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", nil)
		if rec.Code != http.StatusUnauthorized || errorOf(t, rec).Code != CodeUnauthenticated {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("a cashier voiding without a manager", func(t *testing.T) {
		status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/void",
			`{"authorised_by":"stf_ASHA0001","pin":"2468","reason_code":"x"}`)
		if status != http.StatusForbidden || apiErr.Code != CodeForbidden {
			t.Errorf("%d %s", status, apiErr.Code)
		}
	})

	t.Run("a cashier authorising a refund as themselves", func(t *testing.T) {
		status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/refunds",
			`{"authorised_by":"stf_ASHA0001","pin":"2468","amount":{"minor":1,"currency":"INR"},"against_payment_id":1,"reason":"x"}`)
		if status != http.StatusForbidden || apiErr.Code != CodeForbidden {
			t.Errorf("%d %s", status, apiErr.Code)
		}
	})

	t.Run("a kitchen screen cannot work the register", func(t *testing.T) {
		// The kitchen role holds kitchen.view and nothing else, so the
		// caller's own permission is still checked on every route.
		if _, err := r.db.Writer().ExecContext(context.Background(),
			`UPDATE staff SET role_id = (SELECT id FROM roles WHERE name = 'kitchen') WHERE public_id = 'stf_ASHA0001'`); err != nil {
			t.Fatal(err)
		}
		status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/void",
			`{`+r.override()+`,"reason_code":"x"}`)
		if status != http.StatusForbidden || apiErr.Field != string(auth.PermOrderOpen) {
			t.Errorf("%d %+v", status, apiErr)
		}
	})

	t.Run("another venue's order", func(t *testing.T) {
		second := newTill(t)
		rec := send(r.handler, http.MethodGet, "/api/v1/orders/"+id, "", second.asDevice())
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})

	t.Run("a method the route does not take", func(t *testing.T) {
		rec := send(r.handler, http.MethodDelete, "/api/v1/orders/"+id+"/close", "", r.asStaff(r.staffToken))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
}

// TestIdempotencyKeyOverHTTP is the header doing its job: a terminal on
// flaky wifi retries a payment and is charged once.
func TestIdempotencyKeyOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":1,"name":"Dosa","qty":1,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)
	pay := `{"method":"card","amount":{"minor":5000,"currency":"INR"}}`

	status, first, apiErr := r.callKeyed(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", pay, "pay-1")
	if status != http.StatusOK || first.Replayed {
		t.Fatalf("first: %d %+v %+v", status, first.Replayed, apiErr)
	}
	status, again, _ := r.callKeyed(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", pay, "pay-1")
	if status != http.StatusOK || !again.Replayed {
		t.Fatalf("retry: %d replayed=%v", status, again.Replayed)
	}
	if len(again.Order.Tenders) != 1 || !jsonEqual(t, first.Events, again.Events) {
		t.Errorf("the retry is not the first answer: %+v", again.Order.Tenders)
	}

	// The same key on a different request is refused.
	other := `{"method":"cash","amount":{"minor":1000,"currency":"INR"}}`
	status, _, apiErr = r.callKeyed(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", other, "pay-1")
	if status != http.StatusConflict || apiErr.Code != CodeIdempotencyKeyReused {
		t.Errorf("%d %s", status, apiErr.Code)
	}

	// And an open repeated under one key opens one order.
	status, openedOnce, _ := r.callKeyed(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter"}`, "open-1")
	if status != http.StatusCreated {
		t.Fatalf("open: %d", status)
	}
	status, openedTwice, _ := r.callKeyed(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter"}`, "open-1")
	if status != http.StatusOK || !openedTwice.Replayed || openedTwice.Order.ID != openedOnce.Order.ID {
		t.Errorf("the repeated open made another order: %d %+v", status, openedTwice.Order.ID)
	}
}

// TestAVenueThatCannotSellSaysSo covers the setup failure a till can hit:
// GST configured with no state code.
func TestAVenueThatCannotSellSaysSo(t *testing.T) {
	r := newRegister(t)
	ctx := context.Background()
	if err := r.db.WriteTx(ctx, func(q *gen.Queries) error {
		return q.DeleteSetting(ctx, gen.DeleteSettingParams{VenueID: 1, Key: svcorder.SettingTaxStateCode})
	}); err != nil {
		t.Fatal(err)
	}
	status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders", `{"channel":"counter"}`)
	if status != http.StatusConflict || apiErr.Code != CodeVenueNotConfigured {
		t.Errorf("%d %s", status, apiErr.Code)
	}
}

// TestWithoutTheOrderServiceTheRoutesDoNotExist keeps the mounting honest.
func TestWithoutTheOrderServiceTheRoutesDoNotExist(t *testing.T) {
	h := newTestServer(t, testConfig(), WithDevices(workingDevices{}), WithStaff(brokenStaff{})).Handler()
	for _, path := range []string{"/api/v1/orders", "/api/v1/orders/ord_X"} {
		if rec := send(h, http.MethodGet, path, "", bearerOf("nfd_anything")); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
}

// jsonEqual compares two values by their JSON, which is what a client sees.
func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	left, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	right, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(left) == string(right)
}

// TestModifyALineOverHTTP: a line's modifiers change until the kitchen has
// it, a stale From is refused, and a sent line is refused with the code a
// terminal turns into "void it and add it again".
func TestModifyALineOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	const dairy = `{"modifier_id":3,"name":"Dairy","price":{"minor":0,"currency":"INR"}}`
	const oat = `{"modifier_id":4,"name":"Oat","price":{"minor":5000,"currency":"INR"}}`
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":5,"name":"Flat white","qty":2,"unit_price":{"minor":21000,"currency":"INR"},"modifiers":[`+dairy+`],"station":"bar","course":1,"tax_rate":5000,"hsn":"996331"}`)

	res := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/modifiers", `{"from":[`+dairy+`],"to":[`+oat+`]}`)
	line := res.Order.Lines[0]
	if len(line.Modifiers) != 1 || line.Modifiers[0].Name != "Oat" || line.Gross.Minor != 52000 {
		t.Fatalf("line after the change = %+v", line)
	}
	if len(res.Events) != 1 || res.Events[0].Type != "line.modified" {
		t.Errorf("events = %+v, want one line.modified", res.Events)
	}

	tests := []struct {
		name string
		body string
		code ErrorCode
	}{
		{"from modifiers the line no longer has", `{"from":[` + dairy + `],"to":[]}`, CodeModifiersMismatch},
		{"no change", `{"from":[` + oat + `],"to":[` + oat + `]}`, CodeInvalidPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/modifiers", tt.body)
			if status < 400 || apiErr.Code != tt.code {
				t.Errorf("status %d, code %q, want %q", status, apiErr.Code, tt.code)
			}
		})
	}

	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/tickets", `{"station":"bar","line_ids":[2]}`)
	status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/modifiers", `{"from":[`+oat+`],"to":[`+dairy+`]}`)
	if status != http.StatusConflict || apiErr.Code != CodeLineInKitchen {
		t.Errorf("a sent line: status %d, code %q, want 409 line_in_kitchen", status, apiErr.Code)
	}
}

// TestRemoveALineRungUpByMistake: the cashier takes off a line nothing has
// happened to, with no manager, and the void records them and the reason.
// Once it has gone to the kitchen, or money is on the order, it needs a
// manager's void instead (ADR-073).
func TestRemoveALineRungUpByMistake(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	line := `{"item_id":1,"name":"Upma","qty":1,"unit_price":{"minor":9000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line) // 2
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line) // 3
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines", line) // 4

	res := r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/remove", ``)
	removed := res.Order.Lines[0]
	if removed.Status != "voided" || removed.ReasonCode != "entered_in_error" || removed.AuthorisedBy == 0 {
		t.Fatalf("removed line = %+v", removed)
	}
	if len(res.Events) != 1 || res.Events[0].Type != "line.voided" || res.Events[0].ActorID != removed.AuthorisedBy {
		t.Errorf("events = %+v, want one line.voided by the caller", res.Events)
	}

	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/tickets", `{"station":"kitchen","line_ids":[3]}`)
	status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/3/remove", ``)
	if status != http.StatusConflict || apiErr.Code != CodeNeedsApproval {
		t.Errorf("a sent line: %d %q, want 409 needs_approval", status, apiErr.Code)
	}

	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/payments", `{"method":"cash","amount":{"minor":1000,"currency":"INR"}}`)
	status, _, apiErr = r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/4/remove", ``)
	if status != http.StatusConflict || apiErr.Code != CodeNeedsApproval {
		t.Errorf("after a payment: %d %q, want 409 needs_approval", status, apiErr.Code)
	}

	status, _, apiErr = r.call(t, http.MethodPost, "/api/v1/orders/"+id+"/lines/2/remove", ``)
	if status != http.StatusConflict || (apiErr.Code != CodeLineNotActive && apiErr.Code != CodeNeedsApproval) {
		t.Errorf("removing twice: %d %q", status, apiErr.Code)
	}
}

func (r *register) openTickets(t *testing.T) openTicketsResponse {
	t.Helper()
	rec := send(r.handler, http.MethodGet, "/api/v1/orders", "", r.asStaff(r.staffToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /orders: %d %s", rec.Code, rec.Body)
	}
	var out openTicketsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestOpenTicketsHoldAndResume: the list shows every unclosed ticket,
// oldest first, with the venue's late threshold; a held ticket says so and
// takes nothing but notes until it is resumed (ADR-074).
func TestOpenTicketsHoldAndResume(t *testing.T) {
	r := newRegister(t)
	line := `{"item_id":1,"name":"Upma","qty":1,"unit_price":{"minor":9000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`

	if got := r.openTickets(t); got.LateAfterMinutes != 20 || len(got.Orders) != 0 {
		t.Fatalf("an empty venue: %+v", got)
	}
	first := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+first+"/lines", line)
	second := r.openOrder(t)

	list := r.openTickets(t)
	if len(list.Orders) != 2 || list.Orders[0].ID != first || list.Orders[1].ID != second {
		t.Fatalf("list = %+v", list.Orders)
	}
	if o := list.Orders[0]; o.Lines != 1 || o.Total.Minor == 0 || o.TableID != 4 || o.Covers != 2 || o.Status != "open" {
		t.Errorf("first ticket = %+v", o)
	}

	res := r.ok(t, http.MethodPost, "/api/v1/orders/"+first+"/hold", ``)
	if res.Order.Status != "held" || res.Events[0].Type != "order.held" {
		t.Fatalf("after hold: %s %+v", res.Order.Status, res.Events)
	}
	if got := r.openTickets(t).Orders[0].Status; got != "held" {
		t.Errorf("the list says %q", got)
	}
	tests := []struct {
		name, path, body string
		code             ErrorCode
	}{
		{"a line on a held ticket", "/lines", line, CodeOrderHeld},
		{"holding twice", "/hold", ``, CodeOrderHeld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+first+tt.path, tt.body)
			if status != http.StatusConflict || apiErr.Code != tt.code {
				t.Errorf("%d %q, want 409 %q", status, apiErr.Code, tt.code)
			}
		})
	}
	r.ok(t, http.MethodPost, "/api/v1/orders/"+first+"/notes", `{"scope":"order","text":"back in five"}`)

	res = r.ok(t, http.MethodPost, "/api/v1/orders/"+first+"/resume", ``)
	if res.Order.Status != "open" {
		t.Errorf("after resume: %s", res.Order.Status)
	}
	status, _, apiErr := r.call(t, http.MethodPost, "/api/v1/orders/"+first+"/resume", ``)
	if status != http.StatusConflict || apiErr.Code != CodeOrderNotHeld {
		t.Errorf("resuming twice: %d %q", status, apiErr.Code)
	}

	// Closed tickets leave the list; a venue setting moves the threshold.
	r.ok(t, http.MethodPost, "/api/v1/orders/"+second+"/close", ``)
	if err := r.db.WriteTx(context.Background(), func(q *gen.Queries) error {
		return q.SetSetting(context.Background(), gen.SetSettingParams{
			VenueID: 1, Key: svcorder.SettingTicketLateMinutes, Value: "12", UpdatedAt: seededAt,
		})
	}); err != nil {
		t.Fatal(err)
	}
	list = r.openTickets(t)
	if len(list.Orders) != 1 || list.Orders[0].ID != first || list.LateAfterMinutes != 12 {
		t.Errorf("after closing one and setting 12 minutes: %+v", list)
	}
}

func TestLateAfterFallsBackToTwentyMinutes(t *testing.T) {
	r := newRegister(t)
	for _, v := range []string{"", "0", "-5", "ten", "601"} {
		if err := r.db.WriteTx(context.Background(), func(q *gen.Queries) error {
			return q.SetSetting(context.Background(), gen.SetSettingParams{
				VenueID: 1, Key: svcorder.SettingTicketLateMinutes, Value: v, UpdatedAt: seededAt,
			})
		}); err != nil {
			t.Fatal(err)
		}
		if got := r.openTickets(t).LateAfterMinutes; got != 20 {
			t.Errorf("setting %q gave %d minutes, want 20", v, got)
		}
	}
}

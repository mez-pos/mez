package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestCreditRefusalsOverHTTP: the codes a till switches on when a credit
// note is refused.
func TestCreditRefusalsOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	path := "/api/v1/orders/" + id
	added := r.ok(t, http.MethodPost, path+"/lines",
		`{"item_id":1,"name":"Dosa","qty":2,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)

	// Nothing to credit before the invoice.
	status, _, apiErr := r.call(t, http.MethodPost, path+"/credit-notes/preview", `{"lines":[{"line_id":2,"qty":1}]}`)
	if status != http.StatusConflict || apiErr.Code != CodeNotInvoiced {
		t.Errorf("before the invoice: %d %s", status, apiErr.Code)
	}

	paid := r.ok(t, http.MethodPost, path+"/payments", fmt.Sprintf(`{"method":"card","amount":{"minor":%d,"currency":"INR"}}`, added.Order.Totals.BalanceDue.Minor))
	pay := paid.Order.Tenders[0].ID
	tests := []struct {
		name   string
		body   string
		status int
		code   ErrorCode
	}{
		{"three of two", fmt.Sprintf(`{%s,"credit":{"lines":[{"line_id":2,"qty":3}]},"refunds":[{"payment_id":%d,"amount":{"minor":100,"currency":"INR"}}],"reason":"x"}`, r.override(), pay), 409, CodeOverCredit},
		{"refunds short", fmt.Sprintf(`{%s,"credit":{"lines":[{"line_id":2,"qty":1}]},"refunds":[{"payment_id":%d,"amount":{"minor":100,"currency":"INR"}}],"reason":"x"}`, r.override(), pay), 422, CodeRefundsMismatch},
		{"a cashier approving", fmt.Sprintf(`{"authorised_by":"stf_ASHA0001","pin":"2468","credit":{"lines":[{"line_id":2,"qty":1}]},"refunds":[{"payment_id":%d,"amount":{"minor":10500,"currency":"INR"}}],"reason":"x"}`, pay), 403, CodeForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, path+"/credit-notes", tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Errorf("%d %s, want %d %s", status, apiErr.Code, tt.status, tt.code)
			}
		})
	}
}

// TestPastBillsOverHTTP: a settled bill is found by its number, and its
// lines say what has been credited.
func TestPastBillsOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	path := "/api/v1/orders/" + id
	added := r.ok(t, http.MethodPost, path+"/lines",
		`{"item_id":1,"name":"Dosa","qty":2,"unit_price":{"minor":10000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)
	paid := r.ok(t, http.MethodPost, path+"/payments", fmt.Sprintf(`{"method":"cash","amount":{"minor":%d,"currency":"INR"}}`, added.Order.Totals.BalanceDue.Minor))
	r.ok(t, http.MethodPost, path+"/close", ``)

	list := func(q string) pastBillsResponse {
		t.Helper()
		rec := send(r.handler, http.MethodGet, "/api/v1/orders/closed?q="+q, "", r.asStaff(r.staffToken))
		var out pastBillsResponse
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
			t.Fatalf("closed %q: %d %s", q, rec.Code, rec.Body)
		}
		return out
	}
	if got := list(""); len(got.Bills) != 1 || got.Bills[0].ID != id || got.Bills[0].InvoiceNo != "INV/26-27/000001" {
		t.Errorf("latest = %+v", got)
	}
	if got := list("INV%2F26-27%2F000001"); len(got.Bills) != 1 {
		t.Errorf("by number = %+v", got)
	}
	if got := list("7"); len(got.Bills) != 0 {
		t.Errorf("a number nobody has = %+v", got)
	}

	res := r.ok(t, http.MethodPost, path+"/credit-notes", fmt.Sprintf(
		`{%s,"credit":{"lines":[{"line_id":2,"qty":1}]},"refunds":[{"payment_id":%d,"amount":{"minor":10500,"currency":"INR"}}],"reason":"cold"}`,
		r.override(), paid.Order.Tenders[0].ID))
	if l := res.Order.Lines[0]; l.CreditedQty != 1 || l.Credited.Minor != 10500 {
		t.Errorf("line after the credit = %+v", l)
	}
	if got := list(""); got.Bills[0].Credited.Minor != 10500 {
		t.Errorf("credited on the list = %+v", got.Bills[0])
	}
}

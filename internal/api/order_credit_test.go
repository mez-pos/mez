package api

import (
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

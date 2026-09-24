package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestSplitOverHTTP: an odd total three ways, to the paisa, and the codes a
// till switches on when a split is refused.
func TestSplitOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	path := "/api/v1/orders/" + id
	// ₹10.01 and 5% GST: a total that does not divide by three.
	added := r.ok(t, http.MethodPost, path+"/lines",
		`{"item_id":9,"name":"Biscuit","qty":1,"unit_price":{"minor":1001,"currency":"INR"},"station":"bar","tax_rate":5000,"hsn":"996331"}`)
	total := added.Order.Totals.Total.Minor
	if total%3 == 0 {
		t.Fatalf("total %d divides by three", total)
	}

	rec := send(r.handler, http.MethodPost, path+"/split", `{"kind":"evenly","ways":3}`, r.asStaff(r.staffToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("split: %d %s", rec.Code, rec.Body)
	}
	var sp splitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sp); err != nil {
		t.Fatal(err)
	}
	var sum int64
	for i, sh := range sp.Shares {
		sum += sh.Amount.Minor
		if sh.Payer != i || sh.LineIDs == nil {
			t.Errorf("share %d = %+v", i, sh)
		}
	}
	first, last := sp.Shares[0].Amount.Minor, sp.Shares[2].Amount.Minor
	if len(sp.Shares) != 3 || sum != total || sp.Total.Minor != total || first-last != 1 || sp.Kind != "evenly" {
		t.Errorf("split = %+v, want three shares summing to %d, the first carrying the odd paisa", sp, total)
	}

	tests := []struct {
		name   string
		body   string
		status int
		code   ErrorCode
	}{
		{"no ways", `{"kind":"evenly"}`, http.StatusUnprocessableEntity, CodeSplitInvalid},
		{"over the bill", `{"kind":"amount","amounts":[{"minor":999999,"currency":"INR"}]}`, http.StatusUnprocessableEntity, CodeSplitOverAllocated},
		{"a line with no payer", `{"kind":"item","ways":2,"items":[]}`, http.StatusUnprocessableEntity, CodeSplitUnassigned},
		{"not JSON", `{`, http.StatusBadRequest, CodeInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, path+"/split", tt.body)
			if status != tt.status || apiErr.Code != tt.code {
				t.Errorf("%d %s, want %d %s", status, apiErr.Code, tt.status, tt.code)
			}
		})
	}

	r.ok(t, http.MethodPost, path+"/payments", `{"method":"cash","amount":{"minor":500,"currency":"INR"},"tip":{"minor":0,"currency":"INR"}}`)
	status, _, apiErr := r.call(t, http.MethodPost, path+"/split", `{"kind":"item","ways":1,"items":[{"line_id":2,"payers":[0]}]}`)
	if status != http.StatusConflict || apiErr.Code != CodeSplitAfterPayment {
		t.Errorf("by item after a payment: %d %s", status, apiErr.Code)
	}
}

package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
)

// TestATipOverTheCeilingNeedsAManagerOverHTTP: a payment's tip over the
// venue's ceiling, half the payment unless the owner says otherwise, is
// refused unless someone who holds tip.approve lets it through with their PIN
// (ADR-261). The cashier takes the payment; Priya the owner approves.
func TestATipOverTheCeilingNeedsAManagerOverHTTP(t *testing.T) {
	r := newRegister(t)
	id := r.openOrder(t)
	r.ok(t, http.MethodPost, "/api/v1/orders/"+id+"/lines",
		`{"item_id":1,"name":"Masala dosa","qty":4,"unit_price":{"minor":12000,"currency":"INR"},"station":"kitchen","tax_rate":5000,"hsn":"996331"}`)
	payments := "/api/v1/orders/" + id + "/payments"
	// 100.00 on the card with a tip of 50.01: a paisa over half.
	over := `"method":"card","amount":{"minor":10000,"currency":"INR"},"tip":{"minor":5001,"currency":"INR"}`

	var priya int64
	if err := r.db.Writer().QueryRowContext(context.Background(),
		`SELECT id FROM staff WHERE public_id = 'stf_PRIYA001'`).Scan(&priya); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		body   string
		status int
		code   ErrorCode
		field  string
	}{
		{"over the ceiling with nobody", `{` + over + `}`, http.StatusForbidden, CodeTipNeedsApproval, "tip"},
		{"over it with an empty override", `{` + over + `,"authorised_by":"","pin":""}`, http.StatusForbidden, CodeTipNeedsApproval, "tip"},
		{"the owner's wrong PIN", `{` + over + `,"authorised_by":"stf_PRIYA001","pin":"0000"}`, http.StatusUnauthorized, CodeInvalidPIN, ""},
		{"the cashier approving their own tip", `{` + over + `,"authorised_by":"stf_ASHA0001","pin":"2468"}`, http.StatusForbidden, CodeForbidden, "authorised_by"},
		{"someone not on the staff", `{` + over + `,"authorised_by":"stf_NOBODY01","pin":"2468"}`, http.StatusUnauthorized, CodeInvalidPIN, ""},
		// The staff member's id is the API's to set from a checked PIN, so a
		// body naming one by number is not a way round it.
		{"an internal id in the body", `{` + over + `,"authorised_by":2}`, http.StatusBadRequest, CodeInvalidRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _, apiErr := r.call(t, http.MethodPost, payments, tt.body)
			if status != tt.status || apiErr.Code != tt.code || apiErr.Field != tt.field {
				t.Fatalf("%d %+v, want %d %s on %q", status, apiErr, tt.status, tt.code, tt.field)
			}
		})
	}
	t.Run("the refusal says why in the till's words", func(t *testing.T) {
		_, _, apiErr := r.call(t, http.MethodPost, payments, `{`+over+`}`)
		if apiErr.Message != "A tip this large needs a manager's approval." {
			t.Errorf("message = %q", apiErr.Message)
		}
	})

	// Nothing was recorded by any of them.
	if res := r.ok(t, http.MethodGet, "/api/v1/orders/"+id, ""); len(res.Order.Tenders) != 0 {
		t.Fatalf("a refused tip left a payment: %+v", res.Order.Tenders)
	}

	// At the ceiling needs nobody.
	at := r.ok(t, http.MethodPost, payments, `{"method":"card","amount":{"minor":10000,"currency":"INR"},"tip":{"minor":5000,"currency":"INR"}}`)
	if tendered := tenderedIn(t, at); tendered.AuthorisedBy != 0 {
		t.Errorf("a tip at the ceiling carries an approver: %+v", tendered)
	}

	// The owner approves, and is on the payment.
	approved := r.ok(t, http.MethodPost, payments, `{`+over+`,`+r.override()+`}`)
	if tendered := tenderedIn(t, approved); tendered.AuthorisedBy != priya || tendered.Tip.Minor != 5001 {
		t.Errorf("the approved payment = %+v, want authorised by %d", tendered, priya)
	}

	// A waiter takes payments at the table and holds no tip.approve: they
	// cannot let their own tip through either.
	if _, err := r.db.Writer().ExecContext(context.Background(),
		`UPDATE staff SET role_id = (SELECT id FROM roles WHERE name = 'waiter' AND venue_id = 1) WHERE public_id = 'stf_ASHA0001'`); err != nil {
		t.Fatal(err)
	}
	status, _, apiErr := r.call(t, http.MethodPost, payments, `{`+over+`,"authorised_by":"stf_ASHA0001","pin":"2468"}`)
	if status != http.StatusForbidden || apiErr.Code != CodeForbidden || apiErr.Field != "authorised_by" {
		t.Errorf("a waiter approving: %d %+v", status, apiErr)
	}
}

// tenderedIn is the payment a command recorded.
func tenderedIn(t *testing.T, res commandResponse) order.PaymentTendered {
	t.Helper()
	for _, e := range res.Events {
		if p, ok := e.Payload.(order.PaymentTendered); ok {
			return p
		}
	}
	t.Fatalf("no payment in %+v", res.Events)
	return order.PaymentTendered{}
}

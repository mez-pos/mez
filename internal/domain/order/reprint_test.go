package order

import (
	"errors"
	"testing"
)

// TestInvoiceRefusals: a reprint names an invoice this order has and says
// who allowed it and why; a price snapshot must be of this bill.
func TestInvoiceRefusals(t *testing.T) {
	paid := []Payload{opened(), coffee(1), pay(PaymentCash, 22000), invoiceNo("NF/2026-27/000001")}
	prices := func(total int64) *InvoicePrices { return &InvoicePrices{Total: inr(total)} }

	tests := []struct {
		name string
		last Payload
		want error
	}{
		{"a reprint of another order's invoice", InvoiceReprinted{InvoiceNo: "NF/2026-27/000009", Reason: "guest_copy", AuthorisedBy: 2}, ErrInvoiceNotFound},
		{"a reprint with no reason", InvoiceReprinted{InvoiceNo: "NF/2026-27/000001", AuthorisedBy: 2}, ErrInvalidPayload},
		{"a reprint nobody allowed", InvoiceReprinted{InvoiceNo: "NF/2026-27/000001", Reason: "guest_copy"}, ErrInvalidPayload},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Project(stream(append(paid, tt.last)...), Taxes{}); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}

	t.Run("prices of another bill", func(t *testing.T) {
		wrong := InvoiceIssued{InvoiceNo: "NF/2026-27/000001", Series: "NF", TaxSnapshot: []byte(`{}`), Prices: prices(1)}
		_, err := Project(stream(opened(), coffee(1), pay(PaymentCash, 22000), wrong), Taxes{})
		if !errors.Is(err, ErrInvalidPayload) {
			t.Errorf("err = %v, want ErrInvalidPayload", err)
		}
	})
	t.Run("prices of this bill", func(t *testing.T) {
		right := InvoiceIssued{InvoiceNo: "NF/2026-27/000001", Series: "NF", TaxSnapshot: []byte(`{}`), Prices: prices(22000)}
		if _, err := Project(stream(opened(), coffee(1), pay(PaymentCash, 22000), right), Taxes{}); err != nil {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a reprint before the close counts", func(t *testing.T) {
		s, err := Project(stream(append(paid, InvoiceReprinted{InvoiceNo: "NF/2026-27/000001", Reason: "lost", AuthorisedBy: 2})...), Taxes{})
		if err != nil || s.Invoices[0].Reprints != 1 {
			t.Errorf("reprints %+v, %v", s.Invoices, err)
		}
	})
}

// TestInvoicedBillIsFrozen: once invoiced, nothing may change what was sold
// until a reopen; notes and a reprint still may.
func TestInvoicedBillIsFrozen(t *testing.T) {
	paid := []Payload{opened(), coffee(1), pay(PaymentCash, 22000), invoiceNo("NF/2026-27/000001")}
	for _, change := range []Payload{
		coffee(1),
		LineQtyChanged{LineID: 2, From: 1, To: 2},
		LineVoided{LineID: 2, ReasonCode: "wrong_item", AuthorisedBy: 2},
		LineComped{LineID: 2, ReasonCode: "regular", AuthorisedBy: 2},
		off(ScopeOrder, 0, 100),
	} {
		if _, err := Project(stream(append(paid, change)...), Taxes{}); !errors.Is(err, ErrOrderInvoiced) {
			t.Errorf("%s after the invoice: %v, want ErrOrderInvoiced", change.EventType(), err)
		}
	}
	after := append(paid, NoteAdded{Scope: ScopeOrder, Text: "table 4"}, closed,
		OrderReopened{Reason: "wrong item", AuthorisedBy: 2}, coffee(1))
	if _, err := Project(stream(after...), Taxes{}); err != nil {
		t.Errorf("a note, a close, a reopen and a new line: %v", err)
	}
}

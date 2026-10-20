package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/order"
)

type clockAt time.Time

func (c clockAt) Now() time.Time { return time.Time(c) }

// settled is table 4's bill of a dosa and two coffees, 200.00 and 5% GST,
// paid in two parts: 100.00 in cash and 110.00 by UPI.
func settled(t *testing.T, s *Service) Result {
	t.Helper()
	ctx := context.Background()
	opened := open(t, s)
	must(t)(s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))
	must(t)(s.AddLine(ctx, cashier(""), opened.OrderID, coffee(2)))
	must(t)(s.TenderPayment(ctx, cashier(""), opened.OrderID, TenderPayment{Method: order.PaymentCash, Amount: inr(10000)}))
	return must(t)(s.TenderPayment(ctx, cashier(""), opened.OrderID, TenderPayment{Method: order.PaymentUPI, Amount: inr(11000), Reference: "upi-ref-4471"}))
}

// TestAReceiptIsTheInvoiceTheCounterIssued: the same number, the seller as
// the invoice named it, the lines as billed and the tax by component, with
// how it was paid and nothing of who took it.
func TestAReceiptIsTheInvoiceTheCounterIssued(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	paid := settled(t, s)
	if !paid.State.Invoiced || len(paid.State.Invoices) != 1 {
		t.Fatalf("the bill is not invoiced: %+v", paid.State.Invoices)
	}

	got, err := s.Receipt(ctx, 1, 4, paid.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := s.Invoice(ctx, 1, paid.OrderID, paid.State.Invoices[0].Seq)
	if err != nil {
		t.Fatal(err)
	}
	if got.InvoiceNo != issued.InvoiceNo || got.InvoiceNo == "" || !got.IssuedAt.Equal(issued.IssuedAt) || got.TimeZone != "Asia/Kolkata" || got.Bill != paid.PublicID {
		t.Errorf("receipt %s issued %v in %q, want %s at %v", got.InvoiceNo, got.IssuedAt, got.TimeZone, issued.InvoiceNo, issued.IssuedAt)
	}
	if got.Seller != issued.Prices.Seller || got.Seller.Registration == "" {
		t.Errorf("seller = %+v", got.Seller)
	}
	if len(got.Lines) != 2 || got.Lines[0].Name != "Masala dosa" || got.Lines[1].Qty != 2 || got.Lines[1].Gross != inr(8000) {
		t.Errorf("lines = %+v", got.Lines)
	}
	if got.Items != inr(20000) || !got.Discounts.IsZero() || got.Total != inr(21000) || got.TaxIncluded {
		t.Errorf("items %v, discounts %v, total %v, tax included %v", got.Items, got.Discounts, got.Total, got.TaxIncluded)
	}
	if len(got.Taxes) != 2 || got.Taxes[0].Name != "CGST" || got.Taxes[0].Amount != inr(500) || got.Taxes[1].Name != "SGST" {
		t.Errorf("taxes = %+v", got.Taxes)
	}
	want := []ReceiptPayment{{Method: order.PaymentCash, Amount: inr(10000)}, {Method: order.PaymentUPI, Amount: inr(11000)}}
	if len(got.Payments) != 2 || got.Payments[0] != want[0] || got.Payments[1] != want[1] {
		t.Errorf("payments = %+v, want %+v", got.Payments, want)
	}
	if !got.Credited.IsZero() || !got.Refunded.IsZero() {
		t.Errorf("credited %v, refunded %v", got.Credited, got.Refunded)
	}

	t.Run("it is the same once the bill is closed", func(t *testing.T) {
		must(t)(s.Close(ctx, cashier(""), paid.OrderID))
		closed, err := s.Receipt(ctx, 1, 4, paid.PublicID)
		if err != nil || closed.InvoiceNo != got.InvoiceNo || closed.Total != got.Total || len(closed.Lines) != 2 {
			t.Errorf("after the close: %+v, %v", closed, err)
		}
	})
}

// TestAReceiptLeavesOutAVoidedPayment: a payment taken by mistake and voided
// was never the guest's.
func TestAReceiptLeavesOutAVoidedPayment(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	opened := open(t, s)
	must(t)(s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))
	wrong := must(t)(s.TenderPayment(ctx, cashier(""), opened.OrderID, TenderPayment{Method: order.PaymentCard, Amount: inr(5000)}))
	must(t)(s.VoidPayment(ctx, cashier(""), opened.OrderID, VoidPayment{PaymentID: wrong.State.Tenders[0].ID, Reason: "wrong card", AuthorisedBy: 7}))
	paid := must(t)(s.TenderPayment(ctx, cashier(""), opened.OrderID, TenderPayment{Method: order.PaymentCash, Amount: inr(12600)}))

	got, err := s.Receipt(ctx, 1, 4, paid.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Payments) != 1 || got.Payments[0] != (ReceiptPayment{Method: order.PaymentCash, Amount: inr(12600)}) {
		t.Errorf("payments = %+v", got.Payments)
	}
}

// TestThereIsNoReceipt: for a bill that is not settled, one asked for from
// another table or venue, one that is not there, or one too old. Each is the
// same answer.
func TestThereIsNoReceipt(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	paid := settled(t, s)

	owing := must(t)(s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 6, Covers: 1}))
	must(t)(s.AddLine(ctx, cashier(""), owing.OrderID, dosa(1)))

	tests := []struct {
		name         string
		venue, table int64
		bill         string
	}{
		{"a bill not yet settled", 1, 6, owing.PublicID},
		{"from another table", 1, 5, paid.PublicID},
		{"from another venue", 2, 4, paid.PublicID},
		{"no such bill", 1, 4, "ord_NOSUCH01"},
		{"nothing asked for", 1, 4, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Receipt(ctx, tt.venue, tt.table, tt.bill); !errors.Is(err, ErrReceiptNotFound) {
				t.Errorf("err = %v, want ErrReceiptNotFound", err)
			}
		})
	}

	t.Run("too old to give out", func(t *testing.T) {
		at := func(after time.Duration) *Service {
			later, err := New(Options{DB: d, Clock: clockAt(now.Add(after))})
			if err != nil {
				t.Fatal(err)
			}
			return later
		}
		if _, err := at(ReceiptLife).Receipt(ctx, 1, 4, paid.PublicID); err != nil {
			t.Errorf("on its last day: %v", err)
		}
		if _, err := at(ReceiptLife+time.Second).Receipt(ctx, 1, 4, paid.PublicID); !errors.Is(err, ErrReceiptNotFound) {
			t.Errorf("a second past it: %v", err)
		}
	})
}

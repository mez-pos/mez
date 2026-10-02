package order

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/order"
)

// TestInvoiceReadAndReprint: an invoice reads back as issued, and a reprint
// counts without allocating, after the close as well as before.
func TestInvoiceReadAndReprint(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, id, dosa(1)))
	must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentUPI, Amount: inr(12600), Reference: "UPI 42"}))
	must(t)(s.Close(ctx, m, id))

	seq := int64(4) // opened 1, line 2, tender 3, invoice 4
	inv, err := s.Invoice(ctx, 1, id, seq)
	if err != nil {
		t.Fatalf("Invoice: %v", err)
	}
	if inv.Prices == nil || inv.Prices.Total != inr(12600) || inv.Tax.Tax != inr(600) || !inv.IssuedAt.Equal(now) ||
		inv.TimeZone != "Asia/Kolkata" || len(inv.Tenders) != 1 || inv.Reprints != 0 || inv.InvoiceNo != "INV/26-27/000001" {
		t.Fatalf("invoice = %+v", inv)
	}

	cmd := ReprintInvoice{InvoiceSeq: seq, Reason: "guest_copy", AuthorisedBy: 7}
	res := must(t)(s.Reprint(ctx, m, id, cmd))
	if len(res.Events) != 1 || res.Events[0].Type != order.TypeInvoiceReprinted || res.State.Invoices[0].Reprints != 1 {
		t.Fatalf("reprint = %+v", res.Events)
	}
	if again, _ := s.Invoice(ctx, 1, id, seq); again.Reprints != 1 {
		t.Errorf("reprints read back = %d", again.Reprints)
	}
	register, err := d.Queries().InvoicesForVenue(ctx, 1)
	if err != nil || len(register) != 1 {
		t.Errorf("a reprint touched the register: %d invoices, %v", len(register), err)
	}

	if _, err := s.Reprint(ctx, m, id, ReprintInvoice{InvoiceSeq: 3, Reason: "x", AuthorisedBy: 7}); !errors.Is(err, order.ErrInvoiceNotFound) {
		t.Errorf("reprint of another number: %v", err)
	}
	if _, err := s.Invoice(ctx, 1, id, 3); !errors.Is(err, ErrInvoiceNotIssued) {
		t.Errorf("read of another number: %v", err)
	}
	if _, err := s.Invoice(ctx, 2, id, seq); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("another venue: %v", err)
	}
}

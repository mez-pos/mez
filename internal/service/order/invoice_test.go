package order

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mez-pos/mez/internal/domain/order"
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

// tickingClock moves on by a second at every read, so two reads can never
// tie and the order they happened in is the order of their times.
type tickingClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *tickingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(time.Second)
	return c.at
}

// TestInvoiceNumbersFollowTheirTimestamps: settles that race must come out
// with invoice N dated no later than N+1. The time is read inside the
// transaction that allocates the number, so the single writer orders both;
// read before it, a settle could take its time, lose the lock, and stamp the
// higher number with the earlier time.
func TestInvoiceNumbersFollowTheirTimestamps(t *testing.T) {
	tests := []struct {
		name  string
		bills int
	}{
		{"two bills", 2},
		{"a rush of bills", 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, d, _ := newService(t)
			s.clock = &tickingClock{at: now}
			ctx := context.Background()
			m := cashier("")

			ids := make([]int64, tt.bills)
			for i := range ids {
				ids[i] = open(t, s).OrderID
				must(t)(s.AddLine(ctx, m, ids[i], dosa(1)))
			}

			// Every settle is released at once.
			var wg sync.WaitGroup
			release := make(chan struct{})
			errs := make(chan error, tt.bills)
			for _, id := range ids {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-release
					_, err := s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: inr(12600)})
					errs <- err
				}()
			}
			close(release)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatalf("settling: %v", err)
				}
			}

			register, err := d.Queries().InvoicesForVenue(ctx, 1)
			if err != nil || len(register) != tt.bills {
				t.Fatalf("register has %d invoices, %v; want %d", len(register), err, tt.bills)
			}
			for i := 1; i < len(register); i++ {
				prev, cur := register[i-1], register[i]
				if cur.Seq != prev.Seq+1 {
					t.Fatalf("invoice numbers %d then %d: a gap or a repeat", prev.Seq, cur.Seq)
				}
				if !cur.IssuedAt.After(prev.IssuedAt) {
					t.Errorf("invoice %s at %s is dated no later than %s at %s",
						cur.InvoiceNo, cur.IssuedAt.Format(time.TimeOnly), prev.InvoiceNo, prev.IssuedAt.Format(time.TimeOnly))
				}
			}
		})
	}
}

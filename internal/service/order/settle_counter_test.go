package order

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
)

// takeaway opens what the till rings up at the counter with no table.
func takeaway(t *testing.T, s *Service) Result {
	t.Helper()
	return must(t)(s.Open(context.Background(), cashier(""), OpenOrder{Channel: order.ChannelCounter}))
}

// kinds is the types of a command's events, in order.
func kinds(res Result) []order.Type {
	out := make([]order.Type, len(res.Events))
	for i, e := range res.Events {
		out[i] = e.Type
	}
	return out
}

// sentTo is the stations a command's events sent tickets to.
func sentTo(res Result) []string {
	var out []string
	for _, e := range res.Events {
		if pl, ok := e.Payload.(order.TicketSent); ok {
			out = append(out, pl.Station)
		}
	}
	return out
}

// TestTheSettlingPaymentOfACounterSale (ADR-383): a sale rung up at the
// counter with no table is sent, paid, invoiced and closed by the payment
// that settles it, in the one command. A table's ticket and a part payment
// are left as they were.
func TestTheSettlingPaymentOfACounterSale(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		// open starts the order; send taps Send before paying.
		open   func(t *testing.T, s *Service) Result
		send   bool
		part   bool
		closed bool
		sent   []string
	}{
		{name: "never sent: the payment sends it and closes it", open: takeaway, closed: true, sent: []string{"kitchen", "bar"}},
		{name: "sent first: the payment only closes it", open: takeaway, send: true, closed: true},
		{name: "a part payment sends nothing and leaves it open", open: takeaway, part: true},
		{name: "a table's ticket paid in full stays at its table (ADR-372)", open: open},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, _ := newService(t)
			id := tt.open(t, s).OrderID
			must(t)(s.AddLine(ctx, cashier(""), id, dosa(1)))
			res := must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
			if tt.send {
				res = must(t)(s.Send(ctx, cashier(""), id))
			}
			amount := res.State.Totals.BalanceDue
			if tt.part {
				amount = inr(amount.Minor / 2)
			}

			paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: amount}))

			if got := paid.State.Status == order.StatusClosed; got != tt.closed {
				t.Fatalf("closed = %v (status %s), want %v; events %v", got, paid.State.Status, tt.closed, kinds(paid))
			}
			if got := sentTo(paid); !slices.Equal(got, tt.sent) {
				t.Errorf("sent to %v, want %v", got, tt.sent)
			}
			if !tt.closed {
				return
			}
			for _, l := range paid.State.Lines {
				if !l.Sent {
					t.Errorf("line %d (%s) never reached its station", l.ID, l.Name)
				}
			}
			got := kinds(paid)
			if !paid.State.Invoiced || got[len(got)-1] != order.TypeOrderClosed {
				t.Errorf("invoiced %v, events %v: want the close last, after the invoice", paid.State.Invoiced, got)
			}
			// Whatever was sent went before the money, so a ticket never
			// follows a closed bill.
			if i := slices.Index(got, order.TypePaymentTendered); i < 0 || slices.Contains(got[i:], order.TypeTicketSent) {
				t.Errorf("events %v: the tickets must come before the payment", got)
			}
		})
	}
}

// TestClosingACounterSaleItsPaymentClosedIsDone: New Order on a till still
// running the code from before ADR-383 closes after paying. The order is
// closed already, so the close is answered as done, and appends nothing.
func TestClosingACounterSaleItsPaymentClosedIsDone(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id := takeaway(t, s).OrderID
	res := must(t)(s.AddLine(ctx, cashier(""), id, coffee(2)))
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCard, Amount: res.State.Totals.BalanceDue}))
	again, err := s.Close(ctx, cashier("new-order"), id)
	if err != nil {
		t.Fatalf("closing a closed order: %v, want done", err)
	}
	if again.State.Status != order.StatusClosed || len(again.Events) != 0 || again.State.LastSeq != paid.State.LastSeq {
		t.Fatalf("after the second close: status %s, %d events, last seq %d (was %d)", again.State.Status, len(again.Events), again.State.LastSeq, paid.State.LastSeq)
	}
	// A close on an order that is not settled is still refused for that.
	other := takeaway(t, s).OrderID
	must(t)(s.AddLine(ctx, cashier(""), other, coffee(1)))
	if _, err := s.Close(ctx, cashier(""), other); err == nil || errors.Is(err, order.ErrOrderClosed) {
		t.Fatalf("closing an unpaid order: %v, want it refused as owing", err)
	}
}

// TestACounterSaleClosesAtTheRoundedCash (ADR-377, ADR-383): cash rounded
// to the venue's step settles the bill, so it closes it too.
func TestACounterSaleClosesAtTheRoundedCash(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	if _, err := d.Writer().ExecContext(ctx, `UPDATE venues SET cash_rounding_step_minor = 50 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	id := takeaway(t, s).OrderID
	// ₹41.23 and 5% GST is ₹43.29, off the fifty paise step.
	odd := coffee(1)
	odd.UnitPrice = inr(4123)
	res := must(t)(s.AddLine(ctx, cashier(""), id, odd))
	due := res.State.Totals.CashDue
	if due == res.State.Totals.BalanceDue {
		t.Fatalf("the balance %v is on the step: the test needs one that is not", due)
	}
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCash, Amount: due}))
	if paid.State.Status != order.StatusClosed {
		t.Fatalf("status %s after cash of %v, want closed", paid.State.Status, due)
	}
}

// TestAVoidedPaymentReopensACounterSaleItClosed (ADR-084): closing at the
// payment takes nothing away from putting a wrong payment right.
func TestAVoidedPaymentReopensACounterSaleItClosed(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id := takeaway(t, s).OrderID
	res := must(t)(s.AddLine(ctx, cashier(""), id, coffee(1)))
	paid := must(t)(s.TenderPayment(ctx, cashier(""), id, TenderPayment{Method: order.PaymentCard, Amount: res.State.Totals.BalanceDue}))
	voided := must(t)(s.VoidPayment(ctx, cashier(""), id, VoidPayment{PaymentID: paid.State.Tenders[0].ID, Reason: "wrong card", AuthorisedBy: 7}))
	if voided.State.Status == order.StatusClosed || voided.State.Totals.BalanceDue != res.State.Totals.BalanceDue {
		t.Fatalf("after the void: status %s, due %v, want open again and owing %v", voided.State.Status, voided.State.Totals.BalanceDue, res.State.Totals.BalanceDue)
	}
}

package charges

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/gateway"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// TestAnOrderToCollectIsPaidForAndSent: the whole of pickup's money. A
// charge with no table is asked for what the order asks; until the gateway
// says paid there is no ticket and nothing in the kitchen; then the order is
// a ticket of its own with a code, sent, and paid in full with one tender.
func TestAnOrderToCollectIsPaidForAndSent(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_PICK0001", 0)

	c, err := w.s.StartRound(ctx, 1, 0, "rnd_PICK0001", "Order to collect", "https://cafe.example/order/p/code")
	if err != nil {
		t.Fatal(err)
	}
	if c.TableID != 0 || c.OrderID != 0 || c.Round != "rnd_PICK0001" || c.Amount != inr(16800) || c.Status != Open {
		t.Fatalf("charge = %+v", c)
	}
	// Asked again it is the same charge, and the gateway is asked once.
	if again, err := w.s.StartRound(ctx, 1, 0, "rnd_PICK0001", "", ""); err != nil || !again.Replayed || again.PublicID != c.PublicID || len(w.gw.Started) != 1 {
		t.Errorf("asked again: %+v, %v", again, err)
	}
	// A table's code does not pay for it.
	if _, err := w.s.StartRound(ctx, 1, 4, "rnd_PICK0001", "", ""); !errors.Is(err, svcorder.ErrRoundNotFound) {
		t.Errorf("started from table 4: %v", err)
	}

	if got, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || got.Status != Open || got.OrderID != 0 {
		t.Fatalf("before paying: %+v, %v", got, err)
	}

	w.gw.Pay(gatewayID(1), gateway.UPI)
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Paid || got.OrderID == 0 || got.OrderID == w.ticket.OrderID || got.TableID != 0 {
		t.Fatalf("after paying: %+v, %v", got, err)
	}
	res, err := w.orders.Order(ctx, 1, got.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	st := res.State
	if st.Channel != order.ChannelWeb || st.TableID != 0 || len(st.Lines) != 2 || len(st.Tickets) != 2 {
		t.Errorf("its ticket: channel %s, table %d, %d lines, %d tickets", st.Channel, st.TableID, len(st.Lines), len(st.Tickets))
	}
	if len(st.Tenders) != 1 || st.Tenders[0].Amount != inr(16800) || st.Tenders[0].Method != order.PaymentUPI ||
		!st.Totals.BalanceDue.IsZero() || !st.Invoiced {
		t.Errorf("tenders %+v, owed %v, invoiced %v", st.Tenders, st.Totals.BalanceDue, st.Invoiced)
	}
	round, err := w.orders.TrackRound(ctx, 1, 0, "rnd_PICK0001")
	if err != nil || !order.PickupCodeOK(round.PickupCode) || round.Progress != order.ProgressSent {
		t.Errorf("the order = %+v, %v", round, err)
	}
	// Table 4's bill is as it was.
	if table := w.state(t); len(table.Lines) != 2 || len(table.Tenders) != 0 || table.Totals.BalanceDue != inr(16800) {
		t.Errorf("table 4 after a pickup was paid: %d lines, %d tenders, %v owed", len(table.Lines), len(table.Tenders), table.Totals.BalanceDue)
	}

	// Settled again, and again: one ticket, one tender.
	if again, err := w.s.Settle(ctx, 1, c.PublicID); err != nil || again.Status != Paid || again.OrderID != got.OrderID {
		t.Errorf("settled again: %+v, %v", again, err)
	}
	if after, _ := w.orders.Order(ctx, 1, got.OrderID); len(after.State.Tenders) != 1 || len(after.State.Lines) != 2 {
		t.Errorf("settled twice: %d tenders, %d lines", len(after.State.Tenders), len(after.State.Lines))
	}
}

// TestMoneyForAnOrderToCollectThatCannotGoIsSaidWithNoTable: the gateway
// took another amount. Nothing is sent, and staff are shown the charge as
// one for an order to collect, where a table's name would be.
func TestMoneyForAnOrderToCollectThatCannotGoIsSaidWithNoTable(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	w.hold(t, "rnd_PICK0001", 0)
	c, err := w.s.StartRound(ctx, 1, 0, "rnd_PICK0001", "", "")
	if err != nil {
		t.Fatal(err)
	}
	w.gw.PayAmount(gatewayID(1), inr(100))
	got, err := w.s.Settle(ctx, 1, c.PublicID)
	if err != nil || got.Status != Unapplied || got.OrderID != 0 {
		t.Fatalf("a wrong amount: %+v, %v", got, err)
	}
	waiting, err := w.s.Waiting(ctx, 1)
	if err != nil || len(waiting) != 1 {
		t.Fatalf("waiting for staff: %+v, %v", waiting, err)
	}
	if u := waiting[0]; u.TableID != 0 || u.TableName != ToCollect || u.PublicID != c.PublicID || u.OrderPublicID != "" {
		t.Errorf("unapplied = %+v", u)
	}
	// A table cannot let go of it: it is no table's.
	if _, err := w.s.LetGo(ctx, 1, 4, c.PublicID); !errors.Is(err, ErrChargeNotFound) {
		t.Errorf("let go from table 4: %v", err)
	}
}

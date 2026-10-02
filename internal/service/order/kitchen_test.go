package order

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/domain/kitchen"
	"github.com/byjoellobo/mez/internal/domain/order"
)

type laterClock struct{ at time.Time }

func (c laterClock) Now() time.Time { return c.at }

// TestTheKitchenBoard: each station sees only its tickets; a bump moves one
// to ready, a recall back; the kitchen still bumps after the bill is paid;
// a bumped ticket leaves the board once the recall window has passed.
func TestTheKitchenBoard(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	id := open(t, s).OrderID                  // table 4
	must(t)(s.AddLine(ctx, m, id, dosa(1)))   // 2, kitchen
	must(t)(s.AddLine(ctx, m, id, coffee(1))) // 3, bar
	must(t)(s.Send(ctx, m, id))

	board := func(station string) Board {
		t.Helper()
		b, err := s.Board(ctx, 1, station)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	k := board("kitchen")
	if len(k.Tickets) != 1 || k.Tickets[0].Ticket.Station != "kitchen" || k.Tickets[0].Ticket.Status != kitchen.TicketFired || k.Tables[4] != "4" {
		t.Fatalf("kitchen board = %+v", k)
	}
	if b := board("bar"); len(b.Tickets) != 1 || b.Tickets[0].Ticket.LineIDs[0] != 3 {
		t.Fatalf("bar board = %+v", b.Tickets)
	}
	seq := k.Tickets[0].Ticket.Seq

	// A device bumps: no staff member, the device is the actor.
	kds := Meta{VenueID: 1, DeviceID: 9}
	bumped := must(t)(s.Bump(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	if bumped.Events[0].ActorID != 0 || bumped.Events[0].DeviceID != 9 {
		t.Errorf("bump by %d on %d", bumped.Events[0].ActorID, bumped.Events[0].DeviceID)
	}
	if tk := board("kitchen").Tickets[0].Ticket; tk.Status != kitchen.TicketReady {
		t.Errorf("after the bump: %s", tk.Status)
	}
	if _, err := s.Bump(ctx, kds, id, BumpTicket{TicketSeq: seq}); err == nil {
		t.Error("bumped twice")
	}
	must(t)(s.Recall(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	if tk := board("kitchen").Tickets[0].Ticket; tk.Status != kitchen.TicketFired {
		t.Errorf("after the recall: %s", tk.Status)
	}
	if _, err := s.Bump(ctx, kds, id, BumpTicket{TicketSeq: 99}); !errors.Is(err, order.ErrTicketNotFound) {
		t.Errorf("bumped no ticket: %v", err)
	}

	// Paid and closed, the kitchen still finishes the dosa.
	res := must(t)(s.Order(ctx, 1, id))
	must(t)(s.TenderPayment(ctx, m, id, TenderPayment{Method: order.PaymentCash, Amount: res.State.Totals.BalanceDue}))
	if st := must(t)(s.Order(ctx, 1, id)).State.Status; st != order.StatusClosed {
		must(t)(s.Close(ctx, m, id))
	}
	must(t)(s.Bump(ctx, kds, id, BumpTicket{TicketSeq: seq}))

	// Past the recall window, the bumped ticket is off the board.
	later, err := New(Options{DB: d, Clock: laterClock{now.Add(RecallWindow + time.Minute)}})
	if err != nil {
		t.Fatal(err)
	}
	if b, err := later.Board(ctx, 1, "kitchen"); err != nil || len(b.Tickets) != 0 {
		t.Errorf("after the window: %+v, %v", b.Tickets, err)
	}
	if b, _ := later.Board(ctx, 1, "bar"); len(b.Tickets) != 1 {
		t.Errorf("the coffee, never bumped, left the bar board: %+v", b.Tickets)
	}
}

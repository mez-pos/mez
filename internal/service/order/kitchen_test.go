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
	// A cook picks it up: in progress, still on the board, and bumped from there.
	started := must(t)(s.Start(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	if started.Events[0].Type != order.TypeTicketStarted || started.Events[0].DeviceID != 9 {
		t.Errorf("start = %+v", started.Events[0])
	}
	if b := board("kitchen"); len(b.Tickets) != 1 || b.Tickets[0].Ticket.Status != kitchen.TicketInProgress || b.Tickets[0].Ticket.StartedAt.IsZero() {
		t.Errorf("after the start: %+v", b.Tickets)
	}
	if _, err := s.Start(ctx, kds, id, BumpTicket{TicketSeq: seq}); err == nil {
		t.Error("started twice")
	}
	must(t)(s.Bump(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	if tk := board("kitchen").Tickets[0].Ticket; tk.Status != kitchen.TicketReady {
		t.Errorf("bumped from in progress: %s", tk.Status)
	}
	must(t)(s.Recall(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	if _, err := s.Start(ctx, kds, id, BumpTicket{TicketSeq: 99}); !errors.Is(err, order.ErrTicketNotFound) {
		t.Errorf("starting a ticket that is not there: %v", err)
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

// TestAStationsDay: the board carries the station's trading day so far: the
// tickets it was sent, those still in the kitchen and those on the pass, and
// how long from sent to ready, with the slowest named by its dish. A ticket
// sent before the trading day began is not in it.
func TestAStationsDay(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	m := cashier("")
	kds := Meta{VenueID: 1, DeviceID: 9}
	at := func(after time.Duration) *Service {
		t.Helper()
		svc, err := New(Options{DB: d, Clock: laterClock{now.Add(after)}})
		if err != nil {
			t.Fatal(err)
		}
		return svc
	}

	board := func(svc *Service, station string) Board {
		t.Helper()
		b, err := svc.Board(ctx, 1, station)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	id := open(t, s).OrderID
	must(t)(s.AddLine(ctx, m, id, dosa(1)))   // 2, kitchen
	must(t)(s.AddLine(ctx, m, id, coffee(1))) // 3, bar
	second := dosa(2)
	second.Name = "Upma"
	must(t)(s.AddLine(ctx, m, id, second)) // 4, kitchen
	must(t)(s.Send(ctx, m, id))

	b := board(s, "kitchen")
	if day := b.Day; day.Sent != 1 || day.InKitchen != 1 || day.Ready != 0 || day.Average != 0 || day.Slowest != nil || day.SlowestDish != nil {
		t.Fatalf("a ticket sent and not made: %+v", day)
	}
	seq := b.Tickets[0].Ticket.Seq

	// Seven minutes on, it goes on the pass.
	must(t)(at(7*time.Minute).Bump(ctx, kds, id, BumpTicket{TicketSeq: seq}))
	day := board(at(8*time.Minute), "kitchen").Day
	if day.Sent != 1 || day.InKitchen != 0 || day.Ready != 1 || day.Average != 7*time.Minute {
		t.Errorf("after the bump: %+v", day.DayStats)
	}
	if slow := day.SlowestDish; slow == nil || slow.Name != "Masala dosa" || slow.More != 1 || slow.Took != 7*time.Minute {
		t.Errorf("the slowest: %+v", slow)
	}

	// The bar's day is its own.
	if bar := board(s, "bar").Day; bar.Sent != 1 || bar.InKitchen != 1 || bar.Ready != 0 {
		t.Errorf("the bar's day: %+v", bar.DayStats)
	}

	// Tomorrow, after the day has turned, yesterday's tickets are not today's.
	if next := board(at(26*time.Hour), "kitchen").Day; next.Sent != 0 || next.Ready != 0 || next.SlowestDish != nil {
		t.Errorf("the next trading day: %+v", next)
	}
}

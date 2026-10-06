package order

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/order"
)

var kds = Meta{VenueID: 1, DeviceID: 9}

func track(t *testing.T, s *Service, table int64, id string) Round {
	t.Helper()
	got, err := s.TrackRound(context.Background(), 1, table, id)
	if err != nil {
		t.Fatalf("tracking %s: %v", id, err)
	}
	return got
}

// ticketsOf is the seqs of the tickets holding any of the given lines, by
// station.
func ticketsOf(t *testing.T, s *Service, orderID int64, lineIDs ...int64) map[string]int64 {
	t.Helper()
	res, err := s.Order(context.Background(), 1, orderID)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, tk := range res.State.Tickets {
		for _, id := range tk.LineIDs {
			for _, want := range lineIDs {
				if id == want {
					out[tk.Station] = tk.Seq
				}
			}
		}
	}
	return out
}

// TestARequestIsTrackedThroughTheKitchen: nothing is said of the kitchen
// until staff accept; then the round follows its own two tickets, the bar's
// and the kitchen's, and is ready when both are.
func TestARequestIsTrackedThroughTheKitchen(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	if got := track(t, w.s, 4, id); got.Status != RoundWaiting || got.Progress != "" || len(got.Lines) != 2 {
		t.Fatalf("a waiting request = %+v", got)
	}

	accepted, err := w.s.AcceptRound(ctx, cashier("a1"), id)
	if err != nil {
		t.Fatal(err)
	}
	orderID := accepted.Round.OrderID
	if got := track(t, w.s, 4, id); got.Status != RoundSent || got.Progress != order.ProgressSent || got.OrderID != orderID {
		t.Fatalf("accepted = %+v, want sent and with the kitchen", got)
	}

	// The ticket was opened for the round: order.opened is 1, its lines 2 and 3.
	tickets := ticketsOf(t, w.s, orderID, 2, 3)
	if len(tickets) != 2 {
		t.Fatalf("tickets = %v, want the bar's and the kitchen's", tickets)
	}
	steps := []struct {
		name string
		do   func() (Result, error)
		want order.Progress
	}{
		{"the bar starts", func() (Result, error) { return w.s.Start(ctx, kds, orderID, BumpTicket{TicketSeq: tickets["bar"]}) }, order.ProgressPreparing},
		{"the coffees are up", func() (Result, error) { return w.s.Bump(ctx, kds, orderID, BumpTicket{TicketSeq: tickets["bar"]}) }, order.ProgressPreparing},
		{"the brownie is up", func() (Result, error) { return w.s.Bump(ctx, kds, orderID, BumpTicket{TicketSeq: tickets["kitchen"]}) }, order.ProgressReady},
		{"the brownie is recalled", func() (Result, error) {
			return w.s.Recall(ctx, kds, orderID, BumpTicket{TicketSeq: tickets["kitchen"]})
		}, order.ProgressPreparing},
	}
	for _, step := range steps {
		if _, err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if got := track(t, w.s, 4, id); got.Progress != step.want {
			t.Errorf("%s: progress %q, want %q", step.name, got.Progress, step.want)
		}
	}

	t.Run("a declined request has none", func(t *testing.T) {
		no := w.ask(t, "k2", 6)
		if _, err := w.s.DeclineRound(ctx, cashier("d1"), no); err != nil {
			t.Fatal(err)
		}
		if got := track(t, w.s, 6, no); got.Status != RoundDeclined || got.Progress != "" {
			t.Errorf("declined = %+v", got)
		}
	})
}

// TestARoundIsTrackedApartFromTheRestOfItsTicket: at a seated table the
// waiter's dosa is on the same ticket. It going out says nothing of the
// guest's round, and a second round is tracked apart from the first.
func TestARoundIsTrackedApartFromTheRestOfItsTicket(t *testing.T) {
	s, d, _ := newService(t)
	g := seedGuestMenu(t, d)
	ctx := context.Background()
	opened := open(t, s) // table 4
	must(t)(s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))
	sentDosa := must(t)(s.Send(ctx, cashier(""), opened.OrderID))

	first, err := s.PlaceRound(ctx, guest("k1"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.PlaceRound(ctx, guest("k2"), g.twoOatAndABrownie(4))
	if err != nil {
		t.Fatal(err)
	}

	// The waiter's dosa is bumped, and the first round's two tickets.
	for _, tk := range sentDosa.State.Tickets {
		must(t)(s.Bump(ctx, kds, opened.OrderID, BumpTicket{TicketSeq: tk.Seq}))
	}
	if got := track(t, s, 4, first.PublicID); got.Progress != order.ProgressSent {
		t.Fatalf("with only the waiter's dosa out, the first round is %q", got.Progress)
	}
	res := must(t)(s.Order(ctx, 1, opened.OrderID))
	bumped := 0
	for _, tk := range res.State.Tickets {
		if tk.Status == "fired" && bumped < 2 {
			must(t)(s.Bump(ctx, kds, opened.OrderID, BumpTicket{TicketSeq: tk.Seq}))
			bumped++
		}
	}
	if got := track(t, s, 4, first.PublicID); got.Progress != order.ProgressReady {
		t.Errorf("the first round is %q, want ready", got.Progress)
	}
	if got := track(t, s, 4, second.PublicID); got.Progress != order.ProgressSent {
		t.Errorf("the second round is %q, want it still with the kitchen", got.Progress)
	}
}

// TestAPaidRoundIsTracked: unpaid, a round has no progress; paid and sent,
// it is with the kitchen.
func TestAPaidRoundIsTracked(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	held := w.hold(t, "k1", 4)
	if got := track(t, w.s, 4, held.PublicID); got.Status != RoundUnpaid || got.Progress != "" || got.Due != inr(58800) {
		t.Fatalf("unpaid = %+v", got)
	}
	if _, err := w.s.SendPaidRound(context.Background(), 1, held.PublicID); err != nil {
		t.Fatal(err)
	}
	if got := track(t, w.s, 4, held.PublicID); got.Status != RoundSent || got.Progress != order.ProgressSent {
		t.Errorf("paid and sent = %+v", got)
	}
}

// TestARoundIsItsTablesOwnToTrack: another table, another venue, and an id
// that is no round, are all the same answer.
func TestARoundIsItsTablesOwnToTrack(t *testing.T) {
	w := newRequestWorld(t)
	id := w.ask(t, "k1", 4)
	tests := []struct {
		name         string
		venue, table int64
		id           string
	}{
		{"another table", 1, 5, id},
		{"another venue", 2, 4, id},
		{"no such round", 1, 4, "rnd_NOSUCH01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := w.s.TrackRound(context.Background(), tt.venue, tt.table, tt.id); !errors.Is(err, ErrRoundNotFound) {
				t.Errorf("err = %v, want ErrRoundNotFound", err)
			}
		})
	}
}

// TestARoundSentBeforeItKnewItsLines: it is told it was sent and no more,
// and never another round's progress.
func TestARoundSentBeforeItKnewItsLines(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	if _, err := w.s.AcceptRound(ctx, cashier("a1"), id); err != nil {
		t.Fatal(err)
	}
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE guest_rounds SET first_seq = NULL WHERE public_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if got := track(t, w.s, 4, id); got.Status != RoundSent || got.Progress != "" {
		t.Errorf("round = %+v, want sent with no progress", got)
	}
}

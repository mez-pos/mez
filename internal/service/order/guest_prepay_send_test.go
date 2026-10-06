package order

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

func (w *prepayWorld) hold(t *testing.T, key string, tableID int64) Round {
	t.Helper()
	round, err := party(t, w.s).PlaceRound(context.Background(), guest(key), w.g.twoOatAndABrownie(tableID))
	if err != nil || round.Status != RoundUnpaid {
		t.Fatalf("holding a round: %+v, %v", round, err)
	}
	return round
}

func (w *prepayWorld) closeShift(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CloseShift(ctx, gen.CloseShiftParams{ID: 1, ClosedBy: 1, ClosedAt: sql.NullTime{Time: now, Valid: true}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (w *prepayWorld) nextShift(t *testing.T) {
	t.Helper()
	w.closeShift(t)
	ctx := context.Background()
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_NEXT0001", VenueID: 1, TradingDay: "2026-09-19", OpenedBy: 1, OpenedAt: now.Add(12 * time.Hour)})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestAPaidRoundOpensATicketAndGoesToTheKitchen: at a table nobody seated,
// the paid round is the table's ticket, a guest's own, with its lines sent,
// and it is nobody's on staff. Sent again, nothing more happens.
func TestAPaidRoundOpensATicketAndGoesToTheKitchen(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	held := w.hold(t, "k1", 4)

	payable, err := w.s.PayableRound(ctx, 1, 4, held.PublicID)
	if err != nil || payable.Due != inr(58800) || payable.Status != RoundUnpaid {
		t.Fatalf("payable = %+v, %v", payable, err)
	}

	sent, err := w.s.SendPaidRound(ctx, 1, held.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != RoundSent || sent.OrderID == 0 {
		t.Fatalf("sent = %+v", sent)
	}
	res, err := w.s.Order(ctx, 1, sent.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	st := res.State
	if st.Channel != order.ChannelQRTable || len(st.Lines) != len(held.Lines) || len(st.Tickets) == 0 {
		t.Fatalf("the ticket: channel %s, %d lines, %d kitchen tickets", st.Channel, len(st.Lines), len(st.Tickets))
	}
	// What the bill rose by is what the round asked for.
	if st.Totals.BalanceDue != inr(58800) {
		t.Errorf("the ticket owes %v, want the 588.00 the round asked", st.Totals.BalanceDue)
	}
	h, err := w.s.OrderHistory(ctx, 1, sent.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range h.Events {
		if e.ActorID != 0 || e.DeviceID != 0 {
			t.Errorf("%s is by actor %d on device %d, want nobody", e.Type, e.ActorID, e.DeviceID)
		}
	}

	again, err := w.s.SendPaidRound(ctx, 1, held.PublicID)
	if err != nil || again.OrderID != sent.OrderID || again.Status != RoundSent {
		t.Fatalf("again = %+v, %v", again, err)
	}
	if res, _ := w.s.Order(ctx, 1, sent.OrderID); len(res.State.Lines) != len(held.Lines) {
		t.Errorf("sent twice, the ticket has %d lines", len(res.State.Lines))
	}
	if n := orderCount(t, w.d); n != 1 {
		t.Errorf("%d orders, want the one", n)
	}
	// Paid for, it is not to be paid for again.
	if _, err := w.s.PayableRound(ctx, 1, 4, held.PublicID); !errors.Is(err, ErrRoundPaid) {
		t.Errorf("a sent round is payable: %v", err)
	}
	// A phone asking after it with its key is told it is sent.
	told, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil || told.Status != RoundSent || told.OrderID != sent.OrderID || told.Due.Minor != 0 {
		t.Errorf("told = %+v, %v", told, err)
	}
}

// TestAPaidRoundGoesOntoTheTablesTicket: where staff seated the table, the
// lines join that ticket and only they are sent.
func TestAPaidRoundGoesOntoTheTablesTicket(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	opened := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))
	must(t)(w.s.AddLine(ctx, cashier(""), opened.OrderID, dosa(1)))
	held := w.hold(t, "k1", 4)

	sent, err := w.s.SendPaidRound(ctx, 1, held.PublicID)
	if err != nil || sent.OrderID != opened.OrderID {
		t.Fatalf("sent = %+v, %v; want it on order %d", sent, err, opened.OrderID)
	}
	res, err := w.s.Order(ctx, 1, opened.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.State.Lines) != 1+len(held.Lines) {
		t.Fatalf("%d lines on the ticket", len(res.State.Lines))
	}
	var onTickets int
	for _, k := range res.State.Tickets {
		onTickets += len(k.LineIDs)
	}
	if onTickets != len(held.Lines) {
		t.Errorf("%d lines went to the kitchen, want the round's %d and not the waiter's", onTickets, len(held.Lines))
	}
}

func TestAPaidRoundThatCannotBeSent(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, w *prepayWorld)
		want    error
		// payable is what asking to pay for it now answers, or nil for yes.
		payable error
	}{
		{"with the venue shut", func(t *testing.T, w *prepayWorld) { w.closeShift(t) }, shift.ErrNoOpenShift, shift.ErrNoOpenShift},
		{"in the shift after its own", func(t *testing.T, w *prepayWorld) { w.nextShift(t) }, ErrRoundLapsed, ErrRoundLapsed},
		{"onto a ticket put on hold since", func(t *testing.T, w *prepayWorld) {
			ctx := context.Background()
			opened := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
			must(t)(w.s.Hold(ctx, cashier(""), opened.OrderID))
		}, ErrTableBusy, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newPrepayWorld(t, true, PayFirst)
			ctx := context.Background()
			held := w.hold(t, "k1", 4)
			tt.prepare(t, w)
			if _, err := w.s.SendPaidRound(ctx, 1, held.PublicID); !errors.Is(err, tt.want) {
				t.Fatalf("sending: %v, want %v", err, tt.want)
			}
			if _, err := w.s.PayableRound(ctx, 1, 4, held.PublicID); !errors.Is(err, tt.payable) {
				t.Errorf("payable: %v, want %v", err, tt.payable)
			}
			// Still held: nothing was sent.
			row, err := w.d.Queries().GuestRoundByPublicID(ctx, held.PublicID)
			if err != nil || row.Status != string(RoundWaiting) || row.OrderID.Valid {
				t.Errorf("the round is %s on order %v, %v", row.Status, row.OrderID, err)
			}
		})
	}
}

// TestOnlyAHeldRoundIsPaidFor: a request for staff, a round at another
// table or venue, and one that is not there are all not found.
func TestOnlyAHeldRoundIsPaidFor(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	held := w.hold(t, "k1", 4)

	if _, err := w.s.PayableRound(ctx, 1, 5, held.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("from another table: %v", err)
	}
	if _, err := w.s.PayableRound(ctx, 2, 4, held.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("from another venue: %v", err)
	}
	if _, err := w.s.SendPaidRound(ctx, 1, "rnd_NOTTHERE"); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("one that is not there: %v", err)
	}

	// A request, placed with paying first turned off.
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		return q.SetSetting(ctx, gen.SetSettingParams{VenueID: 1, Key: SettingPayMode, Value: string(PayAtTable), UpdatedAt: now})
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := party(t, w.s).PlaceRound(ctx, guest("r1"), w.g.twoOatAndABrownie(6))
	if err != nil || request.Status != RoundWaiting {
		t.Fatalf("request = %+v, %v", request, err)
	}
	if _, err := w.s.PayableRound(ctx, 1, 6, request.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("paying for a request: %v", err)
	}
	if _, err := w.s.SendPaidRound(ctx, 1, request.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("sending a request as paid: %v", err)
	}
}

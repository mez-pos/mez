package order

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
)

// requestWorld is a service that counts what it tells the terminals, over a
// venue with the guest menu.
type requestWorld struct {
	s      *Service
	d      *db.DB
	g      guestMenu
	nudges atomic.Int64
}

func newRequestWorld(t *testing.T) *requestWorld {
	t.Helper()
	_, d, _ := newService(t)
	w := &requestWorld{d: d, g: seedGuestMenu(t, d)}
	s, err := New(Options{DB: d, Clock: fixedClock{}, OnChange: func(int64) { w.nudges.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	w.s = s
	return w
}

// ask places a request at a table and returns its id.
func (w *requestWorld) ask(t *testing.T, key string, table int64) string {
	t.Helper()
	round, err := party(t, w.s).PlaceRound(context.Background(), guest(key), w.g.twoOatAndABrownie(table))
	if err != nil {
		t.Fatalf("placing %s: %v", key, err)
	}
	if round.Status != RoundWaiting {
		t.Fatalf("round %s is %s, want it waiting", key, round.Status)
	}
	return round.PublicID
}

func (w *requestWorld) floorVersion(t *testing.T) int64 {
	t.Helper()
	v, err := w.d.FloorVersion(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func (w *requestWorld) closeShift(t *testing.T) {
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

// TestARequestTellsTheTerminals: a request is no order event, so the floor
// version is what says a table wants to order.
func TestARequestTellsTheTerminals(t *testing.T) {
	w := newRequestWorld(t)
	before := w.floorVersion(t)
	id := w.ask(t, "k1", 4)
	if w.floorVersion(t) != before+1 || w.nudges.Load() != 1 {
		t.Errorf("floor version moved %d and the terminals were told %d times, want 1 and 1", w.floorVersion(t)-before, w.nudges.Load())
	}

	waiting, err := w.s.WaitingRounds(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(waiting) != 1 {
		t.Fatalf("%d waiting, want 1", len(waiting))
	}
	got := waiting[0]
	if got.PublicID != id || got.TableID != 4 || got.TableName != "4" || !got.PlacedAt.Equal(now) ||
		got.Subtotal != inr(56000) || len(got.Lines) != 2 || got.Lines[0].Name != "Flat white" {
		t.Errorf("request = %+v", got)
	}
}

// TestAcceptingARequest: the table gets a ticket on the guest's channel, by
// the member of staff who accepted, with the lines as they were priced and
// each at its station.
func TestAcceptingARequest(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	version, nudges := w.floorVersion(t), w.nudges.Load()

	got, err := w.s.AcceptRound(ctx, cashier("click-1"), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Round.Status != RoundSent || got.Round.OrderID == 0 || got.OrderPublicID == "" || got.Replayed {
		t.Fatalf("decision = %+v", got)
	}
	if w.floorVersion(t) != version+1 || w.nudges.Load() != nudges+1 {
		t.Error("accepting did not tell the terminals")
	}

	res, err := w.s.Order(ctx, 1, got.Round.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	st := res.State
	if st.Channel != order.ChannelQRTable || st.Service != order.ServiceDineIn || st.TableID != 4 || st.OpenedBy != 7 {
		t.Errorf("ticket = channel %s, service %s, table %d, opened by %d", st.Channel, st.Service, st.TableID, st.OpenedBy)
	}
	if len(st.Lines) != 2 || !st.Lines[0].Sent || !st.Lines[1].Sent || len(st.Tickets) != 2 {
		t.Fatalf("lines %+v, tickets %+v", st.Lines, st.Tickets)
	}
	if st.Lines[0].UnitPrice != inr(18000) || st.Lines[0].Modifiers[0].Name != "Oat" || st.Lines[0].Qty != 2 {
		t.Errorf("first line = %+v", st.Lines[0])
	}
	// 560.00 plus 5% GST.
	if st.Totals.Total != inr(58800) {
		t.Errorf("total %v, want 588.00", st.Totals.Total)
	}
	h, err := w.s.OrderHistory(ctx, 1, got.Round.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range h.Events {
		if e.ActorID != 7 || e.DeviceID != 3 {
			t.Errorf("%s at seq %d is by actor %d on device %d, want the cashier", e.Type, e.Seq, e.ActorID, e.DeviceID)
		}
	}

	// Nothing is waiting, and the guest's own key now reads as sent.
	if waiting, _ := w.s.WaitingRounds(ctx, 1); len(waiting) != 0 {
		t.Errorf("%d still waiting", len(waiting))
	}
	again, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil || again.Status != RoundSent || !again.Replayed {
		t.Errorf("the guest's retry = %+v, %v", again, err)
	}

	// A second click is the first one's answer, and sends nothing more.
	twice, err := w.s.AcceptRound(ctx, cashier("click-2"), id)
	if err != nil || !twice.Replayed || twice.OrderPublicID != got.OrderPublicID {
		t.Errorf("accepted twice = %+v, %v", twice, err)
	}
	if res, _ := w.s.Order(ctx, 1, got.Round.OrderID); len(res.State.Lines) != 2 {
		t.Errorf("%d lines after a second accept", len(res.State.Lines))
	}
}

// TestAcceptingKeepsThePriceTheGuestSaw: the menu moved after the guest
// asked. The bill has what they were shown.
func TestAcceptingKeepsThePriceTheGuestSaw(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	if _, err := w.d.Writer().ExecContext(ctx, `UPDATE items SET price = price * 2`); err != nil {
		t.Fatal(err)
	}
	if m, err := w.d.Menu(ctx, 1); err != nil || m.Items[0].Price != inr(36000) {
		t.Fatalf("the menu did not move: %+v, %v", m.Items[0].Price, err)
	}

	got, err := w.s.AcceptRound(ctx, cashier(""), id)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.s.Order(ctx, 1, got.Round.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if res.State.Lines[0].UnitPrice != inr(18000) || res.State.Totals.Subtotal != inr(56000) {
		t.Errorf("unit price %v, subtotal %v: the request was priced again", res.State.Lines[0].UnitPrice, res.State.Totals.Subtotal)
	}
}

// TestASecondRequestJoinsTheTicket: two phones asked before anyone answered.
// Each is answered on its own, and both end on the one ticket.
func TestASecondRequestJoinsTheTicket(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	first, second := w.ask(t, "k1", 4), w.ask(t, "k2", 4)

	a, err := w.s.AcceptRound(ctx, cashier(""), first)
	if err != nil {
		t.Fatal(err)
	}
	if waiting, _ := w.s.WaitingRounds(ctx, 1); len(waiting) != 1 || waiting[0].PublicID != second {
		t.Fatalf("waiting = %+v, want the second request still there", waiting)
	}
	b, err := w.s.AcceptRound(ctx, cashier(""), second)
	if err != nil {
		t.Fatal(err)
	}
	if a.Round.OrderID != b.Round.OrderID {
		t.Errorf("orders %d and %d: one table, one ticket", a.Round.OrderID, b.Round.OrderID)
	}
	if res, _ := w.s.Order(ctx, 1, a.Round.OrderID); len(res.State.Lines) != 4 || len(res.State.Tickets) != 4 {
		t.Errorf("%d lines and %d tickets, want 4 and 4", len(res.State.Lines), len(res.State.Tickets))
	}
}

// TestAcceptingAfterStaffSeatTheTable: a waiter opened the ticket while the
// request waited. Accepting adds to it and opens no second one.
func TestAcceptingAfterStaffSeatTheTable(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	seated := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 2}))

	got, err := w.s.AcceptRound(ctx, cashier(""), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Round.OrderID != seated.OrderID || orderCount(t, w.d) != 1 {
		t.Errorf("accepted onto order %d with %d open, want the waiter's %d alone", got.Round.OrderID, orderCount(t, w.d), seated.OrderID)
	}
}

func TestDecliningARequest(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	version, nudges := w.floorVersion(t), w.nudges.Load()

	got, err := w.s.DeclineRound(ctx, cashier(""), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Round.Status != RoundDeclined || got.OrderPublicID != "" || got.Replayed {
		t.Errorf("decision = %+v", got)
	}
	if w.floorVersion(t) != version+1 || w.nudges.Load() != nudges+1 {
		t.Error("declining did not tell the terminals")
	}
	if orderCount(t, w.d) != 0 {
		t.Error("a declined request became an order")
	}
	if waiting, _ := w.s.WaitingRounds(ctx, 1); len(waiting) != 0 {
		t.Errorf("%d still waiting", len(waiting))
	}

	// Who and when are kept.
	var by int64
	var at sql.NullTime
	if err := w.d.Reader().QueryRow(`SELECT decided_by, decided_at FROM guest_rounds WHERE public_id = ?`, id).Scan(&by, &at); err != nil {
		t.Fatal(err)
	}
	if by != 7 || !at.Valid || !at.Time.Equal(now) {
		t.Errorf("declined by %d at %v, want the cashier at %v", by, at, now)
	}

	// Declined twice is declined once; accepted after is refused; and the
	// guest's own retry is told.
	twice, err := w.s.DeclineRound(ctx, cashier(""), id)
	if err != nil || !twice.Replayed || w.floorVersion(t) != version+1 {
		t.Errorf("declined twice = %+v, %v", twice, err)
	}
	if _, err := w.s.AcceptRound(ctx, cashier(""), id); !errors.Is(err, ErrRoundDecided) {
		t.Errorf("accepting a declined request: %v", err)
	}
	again, err := party(t, w.s).PlaceRound(ctx, guest("k1"), w.g.twoOatAndABrownie(4))
	if err != nil || again.Status != RoundDeclined {
		t.Errorf("the guest's retry = %+v, %v", again, err)
	}
}

func TestAnsweringIsRefused(t *testing.T) {
	tests := []struct {
		name string
		do   func(t *testing.T, w *requestWorld) error
		want error
	}{
		{"for a request that does not exist", func(_ *testing.T, w *requestWorld) error {
			_, err := w.s.AcceptRound(context.Background(), cashier(""), "rnd_NOPE0000")
			return err
		}, ErrRoundNotFound},
		{"for another venue's request", func(t *testing.T, w *requestWorld) error {
			id := w.ask(t, "k1", 4)
			_, err := w.s.DeclineRound(context.Background(), Meta{VenueID: 2, ActorID: 7}, id)
			return err
		}, ErrRoundNotFound},
		{"by nobody", func(t *testing.T, w *requestWorld) error {
			id := w.ask(t, "k1", 4)
			_, err := w.s.AcceptRound(context.Background(), guest(""), id)
			return err
		}, ErrInvalidCommand},
		{"declining one already accepted", func(t *testing.T, w *requestWorld) error {
			id := w.ask(t, "k1", 4)
			if _, err := w.s.AcceptRound(context.Background(), cashier(""), id); err != nil {
				t.Fatal(err)
			}
			_, err := w.s.DeclineRound(context.Background(), cashier(""), id)
			return err
		}, ErrRoundDecided},
		{"accepting with the venue shut", func(t *testing.T, w *requestWorld) error {
			id := w.ask(t, "k1", 4)
			w.closeShift(t)
			_, err := w.s.AcceptRound(context.Background(), cashier(""), id)
			return err
		}, shift.ErrNoOpenShift},
		{"accepting onto a ticket on hold", func(t *testing.T, w *requestWorld) error {
			id := w.ask(t, "k1", 4)
			ctx := context.Background()
			seated := must(t)(w.s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4}))
			must(t)(w.s.Hold(ctx, cashier(""), seated.OrderID))
			_, err := w.s.AcceptRound(ctx, cashier(""), id)
			return err
		}, ErrTableBusy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newRequestWorld(t)
			if err := tt.do(t, w); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestARequestLapsesWithItsShift: nobody answered before the close. The next
// shift does not see it, cannot accept it, and it does not count against the
// table.
func TestARequestLapsesWithItsShift(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)
	w.closeShift(t)

	if waiting, err := w.s.WaitingRounds(ctx, 1); err != nil || len(waiting) != 0 {
		t.Errorf("with the venue shut: %d waiting, %v", len(waiting), err)
	}
	later := now.Add(12 * 60 * 60 * 1e9)
	err := w.d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_NEXT0001", VenueID: 1, TradingDay: "2026-09-19", OpenedBy: 1, OpenedAt: later})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if waiting, _ := w.s.WaitingRounds(ctx, 1); len(waiting) != 0 {
		t.Errorf("%d waiting in the next shift", len(waiting))
	}
	if _, err := w.s.AcceptRound(ctx, cashier(""), id); !errors.Is(err, ErrRoundLapsed) {
		t.Errorf("accepting yesterday's request: %v", err)
	}
	// It can still be declined, to put it away.
	if got, err := w.s.DeclineRound(ctx, cashier(""), id); err != nil || got.Round.Status != RoundDeclined {
		t.Errorf("declining yesterday's request: %+v, %v", got, err)
	}
}

// TestTwoTillsAcceptAtOnce: the kitchen gets the round once, and both tills
// are told it was accepted.
func TestTwoTillsAcceptAtOnce(t *testing.T) {
	w := newRequestWorld(t)
	ctx := context.Background()
	id := w.ask(t, "k1", 4)

	const tills = 5
	got := make([]Decision, tills)
	errs := make([]error, tills)
	var wg sync.WaitGroup
	for i := range tills {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.AcceptRound(ctx, cashier(""), id)
		}()
	}
	wg.Wait()
	var lines [][]int64
	for i := range tills {
		if errs[i] != nil {
			t.Fatalf("till %d: %v", i, errs[i])
		}
		if got[i].Round.Status != RoundSent || got[i].Round.OrderID != got[0].Round.OrderID {
			t.Errorf("till %d got %+v", i, got[i])
		}
	}
	res, err := w.s.Order(ctx, 1, got[0].Round.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range res.State.Tickets {
		lines = append(lines, tk.LineIDs)
	}
	if len(res.State.Lines) != 2 || !reflect.DeepEqual(lines, [][]int64{{2}, {3}}) {
		t.Errorf("%d lines, tickets %v: the round went through more than once", len(res.State.Lines), lines)
	}
	if orderCount(t, w.d) != 1 {
		t.Errorf("%d tickets open at the table, want 1", orderCount(t, w.d))
	}
}

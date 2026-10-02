package db

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/order"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// closeTestShift closes the venue's open shift by hand.
func closeTestShift(t *testing.T, d *DB, venueID int64) {
	t.Helper()
	_, err := d.Writer().ExecContext(context.Background(),
		`UPDATE shifts SET status = 'closed', closed_by = 1, closed_at = '2026-09-18 20:00:00+00:00'
		 WHERE venue_id = ? AND status = 'open'`, venueID)
	if err != nil {
		t.Fatalf("closing venue %d's shift: %v", venueID, err)
	}
}

// eventShifts is the shift each of the order's events was stamped with,
// by seq from 1.
func eventShifts(t *testing.T, d *DB, orderID int64) []int64 {
	t.Helper()
	rows, err := d.Queries().ShiftOfOrderEvents(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.ShiftID
	}
	return out
}

func TestAnOrderWithNoShiftOpenIsRefused(t *testing.T) {
	d := migratedDB(t)
	seedVenue(t, d)

	_, _, err := d.OpenOrder(context.Background(), NewOrder{
		PublicID: "ord_NOSHIFT1", VenueID: 1,
		Opened: ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", OpenedBy: 1}),
	}, untaxed)
	if !errors.Is(err, shift.ErrNoOpenShift) {
		t.Fatalf("err = %v, want ErrNoOpenShift", err)
	}
	var n int
	if err := d.Reader().QueryRow("SELECT count(*) FROM orders").Scan(&n); err != nil || n != 0 {
		t.Errorf("orders left behind: %d, %v", n, err)
	}
}

func TestEventsCarryTheShiftTheyHappenedIn(t *testing.T) {
	d := migratedDB(t)
	ctx := context.Background()
	seedTradingVenue(t, d)

	orderID := openTestOrder(t, d, "ord_SHIFTS01", untaxed)
	mustAppend(t, d, orderID, untaxed, emit(dish(1)))
	mustAppend(t, d, orderID, untaxed, emit(order.TicketSent{Station: "kitchen", LineIDs: []int64{2}}))

	row, err := d.Queries().OrderByID(ctx, orderID)
	if err != nil {
		t.Fatal(err)
	}
	var opened int64
	if err := d.Reader().QueryRow("SELECT shift_id FROM orders WHERE id = ?", orderID).Scan(&opened); err != nil {
		t.Fatal(err)
	}
	if opened != 1 {
		t.Errorf("order %s opened in shift %d, want 1", row.PublicID, opened)
	}

	// Close the shift: the kitchen can still finish the ticket, stamped
	// with no shift, but nothing that touches the bill goes through.
	closeTestShift(t, d, 1)
	mustAppend(t, d, orderID, untaxed, emit(order.TicketBumped{TicketSeq: 3, Station: "kitchen"}))
	_, _, err = d.AppendOrder(ctx, orderID, untaxed, emit(dish(2)))
	if !errors.Is(err, shift.ErrNoOpenShift) {
		t.Fatalf("a line after close: err = %v, want ErrNoOpenShift", err)
	}
	// One refused event refuses the whole append, the allowed one with it.
	_, _, err = d.AppendOrder(ctx, orderID, untaxed, emit(order.TicketRecalled{TicketSeq: 3, Station: "kitchen"}, dish(2)))
	if !errors.Is(err, shift.ErrNoOpenShift) {
		t.Fatalf("a mixed append after close: err = %v, want ErrNoOpenShift", err)
	}

	// The next shift: the tab carries over, and its next line is the new
	// shift's.
	err = d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.CreateShift(ctx, gen.CreateShiftParams{PublicID: "shf_NEXT0001", VenueID: 1, TradingDay: "2026-09-19", OpenedBy: 1, OpenedAt: at})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	mustAppend(t, d, orderID, untaxed, emit(dish(2)))

	got := eventShifts(t, d, orderID)
	want := []int64{1, 1, 1, 0, 2}
	if len(got) != len(want) {
		t.Fatalf("event shifts = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event shifts = %v, want %v", got, want)
		}
	}
}

func TestAnotherVenuesShiftDoesNotCount(t *testing.T) {
	d := migratedDB(t)
	seedVenue(t, d)
	insertVenue(t, d, 2) // opens a shift at venue 2 only

	_, _, err := d.OpenOrder(context.Background(), NewOrder{
		PublicID: "ord_NOSHIFT2", VenueID: 1,
		Opened: ev(order.OrderOpened{Channel: order.ChannelCounter, Currency: "INR", OpenedBy: 1}),
	}, untaxed)
	if !errors.Is(err, shift.ErrNoOpenShift) {
		t.Fatalf("err = %v, want ErrNoOpenShift", err)
	}
}

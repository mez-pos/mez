package order

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/menu"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

func (g guestMenu) pickup() PlacePickup {
	return PlacePickup{Picks: g.twoOatAndABrownie(0).Picks}
}

// TestAnOrderToCollectIsHeldUntilItIsPaid: priced from the menu as a table's
// is, with its tax, and kept unpaid with no table. Nothing is an order, and
// staff are asked nothing: there is nobody at a table for them to see.
func TestAnOrderToCollectIsHeldUntilItIsPaid(t *testing.T) {
	// Pay at the table is the venue's choice for its tables. An order to
	// collect is paid first all the same.
	w := newPrepayWorld(t, true, PayAtTable)
	ctx := context.Background()

	got, err := w.s.PlacePickup(ctx, guest("p1"), w.g.pickup())
	if err != nil {
		t.Fatal(err)
	}
	// 2 x (180.00 + 40.00) + 120.00, and 5% GST: what a table is asked.
	if got.Status != RoundUnpaid || got.TableID != 0 || got.OrderID != 0 || got.Subtotal != inr(56000) || got.Due != inr(58800) ||
		len(got.Lines) != 2 || got.Lines[0].Name != "Flat white" || got.PublicID == "" {
		t.Fatalf("the order = %+v", got)
	}
	if orderCount(t, w.d) != 0 {
		t.Error("an unpaid order to collect opened a ticket")
	}
	if n := w.waitingForStaff(t); n != 0 {
		t.Errorf("%d requests for staff to answer, want none", n)
	}

	// Stored with no table, and found again as it is.
	var tableIsNull bool
	if err := w.d.Writer().QueryRowContext(ctx, `SELECT table_id IS NULL FROM guest_rounds WHERE public_id = ?`, got.PublicID).Scan(&tableIsNull); err != nil || !tableIsNull {
		t.Errorf("stored with a table: null %v, %v", tableIsNull, err)
	}
	payable, err := w.s.PayableRound(ctx, 1, 0, got.PublicID)
	if err != nil || payable.Due != inr(58800) || payable.Status != RoundUnpaid || payable.TableID != 0 {
		t.Errorf("payable = %+v, %v", payable, err)
	}
	// It is no table's: a table's code does not find it.
	if _, err := w.s.PayableRound(ctx, 1, 4, got.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("asked for from table 4: %v", err)
	}
	if _, err := w.s.TrackRound(ctx, 1, 4, got.PublicID); !errors.Is(err, ErrRoundNotFound) {
		t.Errorf("tracked from table 4: %v", err)
	}

	// The same key is the same order; another key is another.
	again, err := w.s.PlacePickup(ctx, guest("p1"), w.g.pickup())
	if err != nil || again.PublicID != got.PublicID || !again.Replayed || again.Due != inr(58800) {
		t.Errorf("placed again = %+v, %v", again, err)
	}
	other, err := w.s.PlacePickup(ctx, guest("p2"), w.g.pickup())
	if err != nil || other.PublicID == got.PublicID {
		t.Errorf("another order = %+v, %v", other, err)
	}
}

// TestAnOrderToCollectThatIsRefused: with no way to pay, with the venue
// shut, with a dish gone, with a key that was a table's round, or with
// nothing in it.
func TestAnOrderToCollectThatIsRefused(t *testing.T) {
	ctx := context.Background()

	t.Run("a venue where guests cannot pay online", func(t *testing.T) {
		w := newPrepayWorld(t, false, PayFirst)
		if _, err := w.s.PlacePickup(ctx, guest("p1"), w.g.pickup()); !errors.Is(err, ErrPickupOff) {
			t.Errorf("err = %v, want ErrPickupOff", err)
		}
	})
	t.Run("the venue is shut", func(t *testing.T) {
		w := newPrepayWorld(t, true, PayFirst)
		w.closeShift(t)
		if _, err := w.s.PlacePickup(ctx, guest("p1"), w.g.pickup()); !errors.Is(err, shift.ErrNoOpenShift) {
			t.Errorf("err = %v, want no open shift", err)
		}
	})
	t.Run("a dish that is not on the menu", func(t *testing.T) {
		w := newPrepayWorld(t, true, PayFirst)
		_, err := w.s.PlacePickup(ctx, guest("p1"), PlacePickup{Picks: []menu.Pick{{ItemID: 9999, Qty: 1}}})
		if !errors.Is(err, menu.ErrNoSuchItem) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("nothing in it, or no key", func(t *testing.T) {
		w := newPrepayWorld(t, true, PayFirst)
		if _, err := w.s.PlacePickup(ctx, guest("p1"), PlacePickup{}); !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("an empty order: %v", err)
		}
		if _, err := w.s.PlacePickup(ctx, guest(""), w.g.pickup()); !errors.Is(err, ErrInvalidCommand) {
			t.Errorf("no key: %v", err)
		}
	})
	t.Run("a key that was a table's round", func(t *testing.T) {
		w := newPrepayWorld(t, true, PayFirst)
		w.hold(t, "k1", 4)
		if _, err := w.s.PlacePickup(ctx, guest("k1"), w.g.pickup()); !errors.Is(err, ErrIdempotencyKeyReused) {
			t.Errorf("err = %v, want the key refused", err)
		}
	})
}

// TestOrdersToCollectLeftUnpaidAreBounded: the venue's code is on a door.
// Past the bound no more are held, and a table's rounds are not counted in
// it nor held back by it.
func TestOrdersToCollectLeftUnpaidAreBounded(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	for i := range MaxUnpaidPickups {
		if _, err := w.s.PlacePickup(ctx, guest(fmt.Sprintf("p%d", i)), w.g.pickup()); err != nil {
			t.Fatalf("order %d: %v", i+1, err)
		}
	}
	if _, err := w.s.PlacePickup(ctx, guest("over"), w.g.pickup()); !errors.Is(err, ErrTooManyWaiting) {
		t.Fatalf("past the bound: %v", err)
	}
	// One that was already placed still answers.
	if again, err := w.s.PlacePickup(ctx, guest("p0"), w.g.pickup()); err != nil || !again.Replayed {
		t.Errorf("one already placed, with the venue at its bound: %+v, %v", again, err)
	}
	// A table still orders.
	if round, err := w.s.PlaceRound(ctx, guest("t4"), w.g.twoOatAndABrownie(4)); err != nil || round.Status != RoundUnpaid {
		t.Errorf("table 4 with the venue's pickups at their bound: %+v, %v", round, err)
	}
}

// TestTheSameOrderToCollectPlacedAtOnceIsOne: a phone that sends twice in
// the same moment has one order.
func TestTheSameOrderToCollectPlacedAtOnceIsOne(t *testing.T) {
	w := newPrepayWorld(t, true, PayFirst)
	ctx := context.Background()
	const taps = 12
	got := make([]Round, taps)
	errs := make([]error, taps)
	var wg sync.WaitGroup
	for i := range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = w.s.PlacePickup(ctx, guest("p1"), w.g.pickup())
		}()
	}
	wg.Wait()
	for i := range taps {
		if errs[i] != nil {
			t.Fatalf("tap %d: %v", i, errs[i])
		}
		if got[i].PublicID != got[0].PublicID {
			t.Errorf("tap %d is order %s, and tap 0 %s", i, got[i].PublicID, got[0].PublicID)
		}
	}
	var n int
	if err := w.d.Writer().QueryRowContext(ctx, `SELECT count(*) FROM guest_rounds WHERE table_id IS NULL`).Scan(&n); err != nil || n != 1 {
		t.Errorf("%d orders to collect after %d taps, %v", n, taps, err)
	}
}

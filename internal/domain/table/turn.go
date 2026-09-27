package table

import (
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// ErrOccupied refuses clearing a table that still has an order open at it.
var ErrOccupied = errors.New("the table has an order open at it")

// Turn is what the floor knows about one table (ADR-086): the orders at it
// not yet closed, when the last one to close there closed, and when the
// table was last cleared. A table's status is derived from these; only
// ClearedAt is stored for the table itself.
type Turn struct {
	Open         []order.State
	LastClosedAt time.Time // zero if no order has closed at the table
	ClearedAt    time.Time // zero if it has never been cleared
}

// View is a table's tile: its status and what the floor shows with it.
type View struct {
	Status Status
	Covers int64 // guests at the table, over its open orders
	// Since is when the table entered its turn: the first open order's
	// opening while occupied, the last closing while dirty, zero when free.
	Since time.Time
	// Balance is what is still owed over the open orders. Valid is false
	// when nothing is open, so a free table shows no amount at all.
	Balance      money.Amount
	BalanceValid bool
}

// Derive works out a table's tile from its turn.
//
// With orders open, the busiest one decides: any order with an active line
// that is not settling makes the table ordered, since another round is
// being taken; otherwise any settling order makes it settling; otherwise
// the guests are seated with nothing ordered yet. A held order still sits
// at its table. With nothing open, the table is dirty if an order closed
// there after it was last cleared, and free otherwise.
func Derive(t Turn) (View, error) {
	if len(t.Open) == 0 {
		v := View{Status: Settled(t.LastClosedAt, t.ClearedAt)}
		if v.Status == StatusDirty {
			v.Since = t.LastClosedAt
		}
		return v, nil
	}
	v := View{Status: StatusSeated}
	settling := false
	for i, o := range t.Open {
		if o.Status == order.StatusClosed {
			return View{}, fmt.Errorf("table: a closed order is not open at the table")
		}
		v.Covers += o.Covers
		if v.Since.IsZero() || o.OpenedAt.Before(v.Since) {
			v.Since = o.OpenedAt
		}
		if i == 0 {
			v.Balance, v.BalanceValid = o.Totals.BalanceDue, true
		} else {
			sum, err := v.Balance.Add(o.Totals.BalanceDue)
			if err != nil {
				return View{}, fmt.Errorf("table: adding the balances: %w", err)
			}
			v.Balance = sum
		}
		switch {
		case o.Status == order.StatusSettling:
			settling = true
		case hasActiveLine(o):
			v.Status = StatusOrdered
		}
	}
	if v.Status == StatusSeated && settling {
		v.Status = StatusSettling
	}
	return v, nil
}

// Settled is the status of a table with no order open at it: dirty if an
// order closed there after it was last cleared, free otherwise.
func Settled(lastClosedAt, clearedAt time.Time) Status {
	if !lastClosedAt.IsZero() && lastClosedAt.After(clearedAt) {
		return StatusDirty
	}
	return StatusFree
}

// Clear checks that a table can be cleared: dirty goes to free, and a table
// already free stays so, since two staff clearing the same table at once
// is not a mistake. A table with an order open at it is refused.
func Clear(unclosed int64, lastClosedAt, clearedAt time.Time) error {
	if unclosed > 0 {
		return fmt.Errorf("%w: %d", ErrOccupied, unclosed)
	}
	status := Settled(lastClosedAt, clearedAt)
	if status == StatusFree {
		return nil
	}
	_, err := Transition(status, StatusFree)
	return err
}

func hasActiveLine(o order.State) bool {
	for _, l := range o.Lines {
		if l.Status == order.LineStatusActive {
			return true
		}
	}
	return false
}

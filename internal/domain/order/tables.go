package order

import (
	"errors"
	"fmt"
	"slices"
)

// Where an order sits (phase 08, Task 2c, ADR-088). An order has one table,
// TableID, which it moves off with order.table_changed. A party that spreads
// over more tables joins them to the order with order.tables_joined and lets
// them go with order.tables_parted: one order, one bill, several tables.
// Whether a table is free to move to or join is the floor's to say; the
// order only checks that its own story adds up.

// The refusals of this file.
var (
	// ErrTableMismatch means a move names a table the order is not at: it
	// was moved by another till since the screen was drawn.
	ErrTableMismatch = errors.New("the order is not at that table")
	// ErrTableAlreadyAt means the order already sits at the table named.
	ErrTableAlreadyAt = errors.New("the order is already at that table")
	// ErrTableNotJoined means a part names a table not joined to the order.
	ErrTableNotJoined = errors.New("that table is not joined to the order")
	// ErrNoTable means a join on an order with no table of its own: seat it
	// at one first.
	ErrNoTable = errors.New("the order has no table to join others to")
	// ErrServiceNoTable means a seating on a delivery order: it is not eaten
	// at a table (ADR-146).
	ErrServiceNoTable = errors.New("a delivery order has no table")
)

// OrderTableChanged moves the order from one table to another. From is where
// the till saw it, so a move made on a stale screen is refused rather than
// applied to wherever the order has gone since. From is 0 for a counter
// ticket being seated at a table.
type OrderTableChanged struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// TablesJoined puts more tables with the order: a party of ten over three
// tables is one order at the first, joined by the other two.
type TablesJoined struct {
	Tables []int64 `json:"tables"`
}

// TablesParted lets joined tables go again.
type TablesParted struct {
	Tables []int64 `json:"tables"`
}

func (OrderTableChanged) EventType() Type { return TypeOrderTableChanged }
func (TablesJoined) EventType() Type      { return TypeTablesJoined }
func (TablesParted) EventType() Type      { return TypeTablesParted }

// Tables is every table the order sits at: its own first, then those
// joined to it, in the order they were joined. None for a counter ticket.
func (s State) Tables() []int64 {
	if s.TableID == 0 {
		return nil
	}
	return append([]int64{s.TableID}, s.Joined...)
}

func (p *projector) changeTable(pl OrderTableChanged) error {
	if pl.From != p.s.TableID {
		return fmt.Errorf("%w: it is at %d, not %d", ErrTableMismatch, p.s.TableID, pl.From)
	}
	if pl.To <= 0 {
		return invalid("a table to move to is required")
	}
	if pl.To == p.s.TableID || slices.Contains(p.s.Joined, pl.To) {
		return fmt.Errorf("%w: %d", ErrTableAlreadyAt, pl.To)
	}
	// A seating is the guest deciding to eat in: a takeaway becomes dine-in.
	// A delivery cannot be seated.
	if p.s.Service == ServiceDelivery {
		return ErrServiceNoTable
	}
	p.s.Service = ServiceDineIn
	p.s.TableID = pl.To
	return nil
}

func (p *projector) joinTables(pl TablesJoined) error {
	if p.s.TableID == 0 {
		return ErrNoTable
	}
	if len(pl.Tables) == 0 {
		return invalid("no tables to join")
	}
	for i, t := range pl.Tables {
		if t <= 0 {
			return invalid("table %d is not a table", t)
		}
		if t == p.s.TableID || slices.Contains(p.s.Joined, t) || slices.Contains(pl.Tables[:i], t) {
			return fmt.Errorf("%w: %d", ErrTableAlreadyAt, t)
		}
	}
	p.s.Joined = append(p.s.Joined, pl.Tables...)
	return nil
}

func (p *projector) partTables(pl TablesParted) error {
	if len(pl.Tables) == 0 {
		return invalid("no tables to part")
	}
	kept := slices.Clone(p.s.Joined)
	for _, t := range pl.Tables {
		i := slices.Index(kept, t)
		if i < 0 {
			return fmt.Errorf("%w: %d", ErrTableNotJoined, t)
		}
		kept = slices.Delete(kept, i, i+1)
	}
	if len(kept) == 0 {
		kept = nil
	}
	p.s.Joined = kept
	return nil
}

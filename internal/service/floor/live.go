package floor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/table"
	svcorder "github.com/byjoellobo/nofiltr-pos/internal/service/order"
)

// LiveTable is a table in service and where it is in its turn.
type LiveTable struct {
	Table gen.LiveTablesRow
	View  table.View
	// Orders are the public ids of the orders open at it, oldest first.
	// Usually one; two when a second ticket was opened at the table.
	Orders []string
	// JoinedTo is the table whose party this one is joined to (ADR-088),
	// 0 when it has a party of its own or none. A joined table shows the
	// party's state; its covers and balance are shown at the party's own.
	JoinedTo int64
}

// Live is the floor as terminals show it in service: the areas in service
// and their tables, each with its status (ADR-086).
type Live struct {
	Areas  []gen.AreasForVenueRow
	Tables []LiveTable
}

// Live reads the floor in service: the plan, and the orders open at each
// table, derived into each table's tile.
func (s *Service) Live(ctx context.Context, venueID int64) (Live, error) {
	if s.orders == nil {
		return Live{}, errors.New("floor: the live floor needs the orders")
	}
	q := s.db.Queries()
	areas, err := q.AreasForVenue(ctx, venueID)
	if err != nil {
		return Live{}, fmt.Errorf("floor: listing areas: %w", err)
	}
	rows, err := q.LiveTables(ctx, venueID)
	if err != nil {
		return Live{}, fmt.Errorf("floor: listing tables: %w", err)
	}
	open, err := s.orders.OpenOrders(ctx, venueID)
	if err != nil {
		return Live{}, fmt.Errorf("floor: reading the open orders: %w", err)
	}

	type atTable struct {
		states   []order.State
		ids      []string
		joinedTo int64
	}
	byTable := map[int64]*atTable{}
	for _, o := range open.Orders {
		for _, id := range o.State.Tables() {
			at := byTable[id]
			if at == nil {
				at = &atTable{}
				byTable[id] = at
			}
			at.states = append(at.states, o.State)
			at.ids = append(at.ids, o.PublicID)
			if id != o.State.TableID {
				at.joinedTo = o.State.TableID
			}
		}
	}

	out := Live{Tables: make([]LiveTable, 0, len(rows))}
	for _, a := range areas {
		if a.Active {
			out.Areas = append(out.Areas, a)
		}
	}
	for _, r := range rows {
		at := byTable[r.ID]
		if at == nil {
			at = &atTable{}
		}
		v, err := table.Derive(table.Turn{Open: at.states, LastClosedAt: r.LastClosedAt.Time, ClearedAt: r.ClearedAt.Time})
		if err != nil {
			return Live{}, fmt.Errorf("floor: table %s: %w", r.Name, err)
		}
		ids := at.ids
		if ids == nil {
			ids = []string{}
		}
		if at.joinedTo != 0 {
			// The party is counted, and owes, at its own table.
			v.Covers, v.BalanceValid = 0, false
		}
		out.Tables = append(out.Tables, LiveTable{Table: r, View: v, Orders: ids, JoinedTo: at.joinedTo})
	}
	return out, nil
}

// Clear marks a dirty table cleared, so it is free to seat. A table already
// free is cleared again without complaint; one with an order open at it is
// refused with table.ErrOccupied.
func (s *Service) Clear(ctx context.Context, venueID, id int64) error {
	return s.write(ctx, venueID, func(tx *gen.Queries) error {
		t, err := tx.TableTurn(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (t.VenueID != venueID || !t.Active)) {
			return fmt.Errorf("%w: %d", ErrTableNotFound, id)
		}
		if err != nil {
			return fmt.Errorf("floor: reading table %d: %w", id, err)
		}
		if err := table.Clear(t.Unclosed, t.LastClosedAt.Time, t.ClearedAt.Time); err != nil {
			return err
		}
		n, err := tx.ClearTable(ctx, gen.ClearTableParams{Now: sql.NullTime{Time: s.clock.Now().UTC(), Valid: true}, ID: id, VenueID: venueID})
		return one(n, err, ErrTableNotFound)
	})
}

// Release lets the guests at a table go before they ordered anything: it
// closes every order open at the table, each of which must be empty
// (table.Leavable), then clears every table those orders sat at, so they
// are free rather than dirty.
// Retried, it finds the orders closed and clears again: no harm. The
// closes are the order service's own commands, on their own events.
func (s *Service) Release(ctx context.Context, meta svcorder.Meta, id int64) error {
	if s.orders == nil {
		return errors.New("floor: letting guests go needs the orders")
	}
	if _, err := tableByID(ctx, s.db.Queries(), meta.VenueID, id); err != nil {
		return err
	}
	open, err := s.orders.OpenOrders(ctx, meta.VenueID)
	if err != nil {
		return fmt.Errorf("floor: reading the open orders: %w", err)
	}
	var at []svcorder.Result
	for _, o := range open.Orders {
		if !slices.Contains(o.State.Tables(), id) {
			continue
		}
		if !table.Leavable(o.State) {
			return fmt.Errorf("%w: %s", table.ErrNotEmpty, o.PublicID)
		}
		at = append(at, o)
	}
	// One request closes several orders, so none of them may carry its
	// idempotency key: the key would match the first close and refuse the
	// second as a different request.
	each := meta
	each.IdempotencyKey = ""
	// Every table the closed tickets sat at is left, not just the one
	// tapped: a party joined over two tables got up from both.
	left := []int64{id}
	for _, o := range at {
		if _, err := s.orders.Close(ctx, each, o.OrderID); err != nil {
			return fmt.Errorf("floor: closing %s: %w", o.PublicID, err)
		}
		for _, t := range o.State.Tables() {
			if !slices.Contains(left, t) {
				left = append(left, t)
			}
		}
	}
	for _, t := range left {
		if err := s.Clear(ctx, meta.VenueID, t); err != nil {
			return err
		}
	}
	return nil
}

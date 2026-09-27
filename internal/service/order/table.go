package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/byjoellobo/nofiltr-pos/internal/db"
	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/table"
)

// ErrTableTaken means a party was moved or joined to a table that is not
// free: seated, being cleared, or someone else's (ADR-088).
var ErrTableTaken = errors.New("that table is not free")

// MoveTable moves a ticket to a free table. From is the table the till saw
// it at, so a move from a stale screen is refused.
type MoveTable struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// JoinTables puts free tables with a ticket, for a party that has grown.
type JoinTables struct {
	Tables []int64 `json:"tables"`
}

// PartTables lets joined tables go again.
type PartTables struct {
	Tables []int64 `json:"tables"`
}

// MoveTable moves an order to another table, which must be free.
func (s *Service) MoveTable(ctx context.Context, meta Meta, orderID int64, cmd MoveTable) (Result, error) {
	return s.run(ctx, meta, orderID, "move_table", cmd, func(ctx context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		if err := tableFree(ctx, tx.Queries(), meta.VenueID, cmd.To); err != nil {
			return nil, err
		}
		return []order.Payload{order.OrderTableChanged(cmd)}, nil
	})
}

// JoinTables joins free tables to an order.
func (s *Service) JoinTables(ctx context.Context, meta Meta, orderID int64, cmd JoinTables) (Result, error) {
	return s.run(ctx, meta, orderID, "join_tables", cmd, func(ctx context.Context, tx *db.OrderTx, _ venue, _ time.Time) ([]order.Payload, error) {
		for _, t := range cmd.Tables {
			if err := tableFree(ctx, tx.Queries(), meta.VenueID, t); err != nil {
				return nil, err
			}
		}
		return []order.Payload{order.TablesJoined(cmd)}, nil
	})
}

// PartTables lets tables joined to an order go. A parted table goes back
// to where its own turn stands, free unless a bill closed there uncleared:
// parting is for a party that no longer needs it, not one that has left
// (ADR-088).
func (s *Service) PartTables(ctx context.Context, meta Meta, orderID int64, cmd PartTables) (Result, error) {
	return s.run(ctx, meta, orderID, "part_tables", cmd, one(order.TablesParted(cmd)))
}

// tableFree refuses a table to move or join a party to unless it is in
// service at the venue and free: no order at it, and cleared since the last
// one closed. It runs in the command's own transaction, so two tills cannot
// both take the same free table.
func tableFree(ctx context.Context, q *gen.Queries, venueID, tableID int64) error {
	if tableID <= 0 {
		return nil // no table: the domain refuses a move to nowhere in its own words
	}
	t, err := q.TableTurn(ctx, tableID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (t.VenueID != venueID || !t.Active)) {
		return fmt.Errorf("%w: %d", ErrNoSuchTable, tableID)
	}
	if err != nil {
		return fmt.Errorf("reading table %d: %w", tableID, err)
	}
	if t.Unclosed > 0 || table.Settled(t.LastClosedAt.Time, t.ClearedAt.Time) != table.StatusFree {
		return fmt.Errorf("%w: %s", ErrTableTaken, t.Name)
	}
	return nil
}

// tableInService refuses an order opened at a table the venue does not have
// in service (ADR-086). Zero is no table: a counter sale.
func tableInService(ctx context.Context, q *gen.Queries, venueID, tableID int64) error {
	if tableID == 0 {
		return nil
	}
	t, err := q.TableByID(ctx, tableID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (t.VenueID != venueID || !t.Active)) {
		return fmt.Errorf("%w: %d", ErrNoSuchTable, tableID)
	}
	if err != nil {
		return fmt.Errorf("reading table %d: %w", tableID, err)
	}
	return nil
}

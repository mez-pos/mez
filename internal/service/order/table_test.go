package order

import (
	"context"
	"errors"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
	"github.com/byjoellobo/nofiltr-pos/internal/domain/order"
)

// TestAnOrderOpensOnlyAtATableInService: a table the venue has, in service;
// no table at all is a counter sale.
func TestAnOrderOpensOnlyAtATableInService(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	addVenue(t, d, "ven_OTHER001", nil) // its tables are 11 to 20
	if err := d.WriteTx(ctx, func(q *gen.Queries) error {
		_, err := q.SetTableActive(ctx, gen.SetTableActiveParams{Active: false, Now: now, ID: 5, VenueID: 1})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		table   int64
		wantErr error
	}{
		{"a counter sale", 0, nil},
		{"a table in service", 4, nil},
		{"a retired table", 5, ErrNoSuchTable},
		{"another venue's table", 11, ErrNoSuchTable},
		{"a table nobody has", 99, ErrNoSuchTable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: tt.table, Covers: 2})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestMoveJoinAndPart: a ticket moves only to a free table, joins free
// tables, lets one go; the joined tables count as occupied, then dirty
// once the bill closes (ADR-088).
func TestMoveJoinAndPart(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	turn := func(id int64) gen.TableTurnRow {
		t.Helper()
		row, err := d.Queries().TableTurn(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	party := must(t)(s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 4, Covers: 6}))
	other := must(t)(s.Open(ctx, cashier(""), OpenOrder{Channel: order.ChannelCounter, TableID: 5, Covers: 2}))

	refusals := []struct {
		name    string
		cmd     MoveTable
		wantErr error
	}{
		{"onto a table in use", MoveTable{From: 4, To: 5}, ErrTableTaken},
		{"onto no table the venue has", MoveTable{From: 4, To: 99}, ErrNoSuchTable},
		{"from a stale screen", MoveTable{From: 3, To: 6}, order.ErrTableMismatch},
	}
	for _, tt := range refusals {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.MoveTable(ctx, cashier(""), party.OrderID, tt.cmd); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	moved := must(t)(s.MoveTable(ctx, cashier(""), party.OrderID, MoveTable{From: 4, To: 6}))
	if moved.State.TableID != 6 || turn(4).Unclosed != 0 || turn(6).Unclosed != 1 {
		t.Fatalf("after the move: at %d; table 4 has %d, 6 has %d", moved.State.TableID, turn(4).Unclosed, turn(6).Unclosed)
	}
	if _, err := s.JoinTables(ctx, cashier(""), party.OrderID, JoinTables{Tables: []int64{7, 5}}); !errors.Is(err, ErrTableTaken) {
		t.Errorf("joined table 5, which has its own party: %v", err)
	}
	joined := must(t)(s.JoinTables(ctx, cashier(""), party.OrderID, JoinTables{Tables: []int64{7, 8}}))
	if len(joined.State.Joined) != 2 || turn(7).Unclosed != 1 || turn(8).Unclosed != 1 {
		t.Fatalf("joined: %v", joined.State.Joined)
	}
	parted := must(t)(s.PartTables(ctx, cashier(""), party.OrderID, PartTables{Tables: []int64{8}}))
	if len(parted.State.Joined) != 1 || turn(8).Unclosed != 0 {
		t.Errorf("parted: %v, table 8 has %d", parted.State.Joined, turn(8).Unclosed)
	}
	// Table 4, left behind, has no order open and none closed: free again.
	if row := turn(4); row.LastClosedAt.Valid {
		t.Errorf("table 4 has a closed order: %v", row.LastClosedAt)
	}

	// The party's bill closes: 6 and 7 are both left to clear.
	must(t)(s.Close(ctx, cashier(""), party.OrderID))
	for _, id := range []int64{6, 7} {
		if row := turn(id); row.Unclosed != 0 || !row.LastClosedAt.Valid {
			t.Errorf("table %d after the close: %d open, closed at %v", id, row.Unclosed, row.LastClosedAt)
		}
	}
	// The other party is where it was.
	if row := turn(5); row.Unclosed != 1 {
		t.Errorf("table 5 has %d open", row.Unclosed)
	}
	_ = other
}

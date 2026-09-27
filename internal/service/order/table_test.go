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

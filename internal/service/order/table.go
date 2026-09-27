package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/byjoellobo/nofiltr-pos/internal/db/gen"
)

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

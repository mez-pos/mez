package db

import (
	"context"
	"fmt"
)

// PrintVersion is the venue's print version: a counter every change to its
// print jobs moves on (ADR-081). The stream compares it to tell the tills.
func (d *DB) PrintVersion(ctx context.Context, venueID int64) (int64, error) {
	v, err := d.Queries().PrintVersion(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("db: print version of venue %d: %w", venueID, err)
	}
	return v, nil
}

package db

import (
	"context"
	"fmt"
)

// FloorVersion is the venue's floor version: a counter moved on by every
// change to the floor that its orders' events do not carry, the plan edited
// or a table cleared (ADR-086). The stream compares it to tell terminals.
func (d *DB) FloorVersion(ctx context.Context, venueID int64) (int64, error) {
	v, err := d.Queries().FloorVersion(ctx, venueID)
	if err != nil {
		return 0, fmt.Errorf("db: floor version of venue %d: %w", venueID, err)
	}
	return v, nil
}

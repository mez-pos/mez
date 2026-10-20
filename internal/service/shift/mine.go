package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mez-pos/mez/internal/domain/shift"
)

// MyShift is what one person did in the open shift (T08).
type MyShift struct {
	ShiftID  int64
	OpenedAt time.Time
	shift.Mine
}

// Mine is what staffID did in the venue's open shift, or ErrNoOpenShift.
// It reads the orders the shift touched and adds them up on each ask, as
// the tip report does; nothing is stored.
func (s *Service) Mine(ctx context.Context, venueID, staffID int64) (MyShift, error) {
	q := s.db.Queries()
	row, err := q.OpenShiftForVenue(ctx, venueID)
	if errors.Is(err, sql.ErrNoRows) {
		return MyShift{}, ErrNoOpenShift
	}
	if err != nil {
		return MyShift{}, fmt.Errorf("reading the open shift: %w", err)
	}
	v, err := venueOf(ctx, q, venueID)
	if err != nil {
		return MyShift{}, err
	}
	orders, err := stampedOrders(ctx, q, row.ID)
	if err != nil {
		return MyShift{}, err
	}
	mine, err := shift.MineOf(v.currency, row.ID, staffID, orders)
	if err != nil {
		return MyShift{}, fmt.Errorf("adding up staff %d's shift: %w", staffID, err)
	}
	return MyShift{ShiftID: row.ID, OpenedAt: row.OpenedAt.UTC(), Mine: mine}, nil
}

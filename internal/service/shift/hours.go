package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/hours"
)

// SettingOpeningHours is the venue setting for when it is open (ADR-360):
// a week as hours.Parse reads it, one opening and closing time per weekday
// the venue opens. Unset is a venue that has not said.
//
// The till does nothing with it. It is carried in the venue's health
// summary (ADR-361), and mez Cloud's clock alerts read it there: a shift
// still open past closing, the venue not heard from while open.
const SettingOpeningHours = "venue.opening_hours"

// OpeningHours is the venue's week, read through q, or nil when it has
// set none. A stored value that is not a week is ErrVenueConfig, as it is
// for the other shift settings: the setting's check refuses one, so this
// is a value written some other way.
func OpeningHours(ctx context.Context, q *gen.Queries, venueID int64) (hours.Week, error) {
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingOpeningHours})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", SettingOpeningHours, err)
	}
	w, err := hours.Parse(row.Value)
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not a week of opening hours: %v", ErrVenueConfig, SettingOpeningHours, err)
	}
	return w, nil
}

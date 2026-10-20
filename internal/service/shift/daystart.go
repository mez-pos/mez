package shift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/shift"
	svcorder "github.com/mez-pos/mez/internal/service/order"
)

// SettingDayStart is the venue setting for when the trading day begins, local
// time, written "04:00". Unset is shift.DefaultDayStart.
//
// A shift keeps the trading day it was opened under, so a change reaches the
// next shift opened and not the one running. The reports, the audit trail and
// "sold out until tomorrow" read it as it stands when they are asked.
const SettingDayStart = svcorder.SettingDayStart

// DefaultVarianceThreshold is the variance threshold of a venue that has set
// none, in minor units: any variance at all needs a reason.
const DefaultVarianceThreshold int64 = 0

// DayStart is when the venue's trading day begins, read through q. A value
// that is not a time of day is ErrVenueConfig, as it is for the other
// shift settings: a trading day cannot be guessed at.
func DayStart(ctx context.Context, q *gen.Queries, venueID int64) (time.Duration, error) {
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingDayStart})
	if errors.Is(err, sql.ErrNoRows) {
		return shift.DefaultDayStart, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", SettingDayStart, err)
	}
	d, err := shift.ParseDayStart(strings.TrimSpace(row.Value))
	if err != nil {
		return 0, fmt.Errorf("%w: %s is %q, not a time of day like 04:00", ErrVenueConfig, SettingDayStart, row.Value)
	}
	return d, nil
}

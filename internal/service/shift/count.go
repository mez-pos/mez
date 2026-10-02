package shift

import (
	"cmp"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/shift"
)

// The close's refusals (ADR-095).
var (
	ErrAlreadyCounted      = errors.New("the drawer has already been counted")
	ErrNotCounted          = errors.New("the drawer has not been counted")
	ErrDrawerCounted       = errors.New("the drawer is counted: nothing more goes in or out")
	ErrVarianceUnexplained = errors.New("the drawer is over or short past the threshold and needs a reason")
	ErrAlreadyExplained    = errors.New("the variance already has a reason")
)

// SettingVarianceThreshold is the venue setting for how far over or short
// a drawer may be, in minor units, before closing needs a reason. Unset is
// 0: any variance at all needs one.
const SettingVarianceThreshold = "shift.variance_threshold_minor"

// MaxReasonLength bounds a variance reason, as the schema does.
const MaxReasonLength = 500

// Count records the blind closing count and reconciles it. The count goes
// in first; only then is the expected figure worked out, stored with it,
// and returned. The count is taken once: a retry of the same count gets
// the shift back, a different one is ErrAlreadyCounted.
//
// Once counted, the shift takes no more sales or cash movements: the
// drawer it describes is the one in the count.
func (s *Service) Count(ctx context.Context, meta Meta, shiftID int64, counted shift.Count) (Shift, error) {
	if err := meta.validate(); err != nil {
		return Shift{}, err
	}
	if counted.Kind != shift.CountClosing {
		return Shift{}, fmt.Errorf("%w: the drawer is counted with a closing count, not a %s count", ErrInvalidCommand, counted.Kind)
	}
	now := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := venueOf(ctx, q, meta.VenueID)
		if err != nil {
			return err
		}
		if err := counted.Validate(v.currency); err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidCommand, err)
		}
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		sh, err := load(ctx, q, row, v.currency)
		if err != nil {
			return err
		}
		if sh.Closing != nil {
			if sameCount(sh.Closing.Count, counted) && sh.Closing.CountedBy == meta.ActorID {
				return nil
			}
			return fmt.Errorf("%w: %s at %s", ErrAlreadyCounted, sh.PublicID, sh.Closing.CountedAt.Format(time.RFC3339))
		}
		if sh.Status != shift.StatusOpen {
			return fmt.Errorf("%w: %s", ErrShiftClosed, sh.PublicID)
		}
		threshold, err := varianceThreshold(ctx, q, meta.VenueID, v.currency)
		if err != nil {
			return err
		}
		rec, err := shift.Reconcile(sh.Expected, counted, threshold)
		if err != nil {
			return err
		}
		return saveClosingCount(ctx, q, row.ID, counted, rec, meta.ActorID, now)
	})
	if err != nil {
		return Shift{}, fmt.Errorf("counting the drawer: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.ByID(ctx, meta.VenueID, shiftID)
}

// ExplainVariance records why the drawer was over or short. It needs the
// count first, since nobody knows the variance until then, and is taken
// once. A reason may be given for a variance within the threshold too.
func (s *Service) ExplainVariance(ctx context.Context, meta Meta, shiftID int64, reason string) (Shift, error) {
	if err := meta.validate(); err != nil {
		return Shift{}, err
	}
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return Shift{}, fmt.Errorf("%w: a reason is required", ErrInvalidCommand)
	case utf8.RuneCountInString(reason) > MaxReasonLength:
		return Shift{}, fmt.Errorf("%w: the reason is over %d characters", ErrInvalidCommand, MaxReasonLength)
	}
	now := s.clock.Now().UTC()
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		row, err := shiftAt(ctx, q, meta.VenueID, shiftID)
		if err != nil {
			return err
		}
		prior, err := q.VarianceReason(ctx, row.ID)
		switch {
		case err == nil:
			if prior.Reason == reason && prior.ExplainedBy == meta.ActorID {
				return nil
			}
			return fmt.Errorf("%w: %s", ErrAlreadyExplained, row.PublicID)
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("reading the variance reason: %w", err)
		}
		if shift.Status(row.Status) != shift.StatusOpen {
			return fmt.Errorf("%w: %s", ErrShiftClosed, row.PublicID)
		}
		if _, err := closingOf(ctx, q, row.ID); err != nil {
			return err
		}
		return q.CreateVarianceReason(ctx, gen.CreateVarianceReasonParams{
			ShiftID: row.ID, Reason: reason, ExplainedBy: meta.ActorID, ExplainedAt: now,
		})
	})
	if err != nil {
		return Shift{}, fmt.Errorf("explaining the variance: %w", err)
	}
	s.onChange(meta.VenueID)
	return s.ByID(ctx, meta.VenueID, shiftID)
}

// closingOf is the shift's closing count row, or ErrNotCounted.
func closingOf(ctx context.Context, q *gen.Queries, shiftID int64) (gen.DrawerCountsForShiftRow, error) {
	rows, err := q.DrawerCountsForShift(ctx, shiftID)
	if err != nil {
		return gen.DrawerCountsForShiftRow{}, fmt.Errorf("reading shift %d's counts: %w", shiftID, err)
	}
	for _, r := range rows {
		if shift.CountKind(r.Kind) == shift.CountClosing {
			return r, nil
		}
	}
	return gen.DrawerCountsForShiftRow{}, fmt.Errorf("%w: shift %d", ErrNotCounted, shiftID)
}

// varianceThreshold reads the venue's threshold: 0, the strictest, when
// unset.
func varianceThreshold(ctx context.Context, q *gen.Queries, venueID int64, currency string) (money.Amount, error) {
	row, err := q.Setting(ctx, gen.SettingParams{VenueID: venueID, Key: SettingVarianceThreshold})
	if errors.Is(err, sql.ErrNoRows) {
		return money.New(0, currency)
	}
	if err != nil {
		return money.Amount{}, fmt.Errorf("reading %s: %w", SettingVarianceThreshold, err)
	}
	minor, err := strconv.ParseInt(strings.TrimSpace(row.Value), 10, 64)
	if err != nil || minor < 0 {
		return money.Amount{}, fmt.Errorf("%w: %s is %q, not a whole number of minor units from 0", ErrVenueConfig, SettingVarianceThreshold, row.Value)
	}
	return money.New(minor, currency)
}

func saveClosingCount(ctx context.Context, q *gen.Queries, shiftID int64, c shift.Count, rec shift.Reconciliation, by int64, at time.Time) error {
	body, err := json.Marshal(nonNil(c.Denominations))
	if err != nil {
		return fmt.Errorf("encoding the closing count: %w", err)
	}
	err = q.CreateDrawerCount(ctx, gen.CreateDrawerCountParams{
		ShiftID: shiftID, Kind: string(shift.CountClosing), Denominations: string(body), Total: c.Total.Minor,
		CountedBy: by, CountedAt: at,
		Expected: rec.Expected.Minor, Variance: rec.Variance.Minor, Threshold: rec.Threshold.Minor,
	})
	if err != nil {
		return fmt.Errorf("recording the closing count: %w", err)
	}
	return nil
}

// sameCount reports whether two counts are the same count: a retry.
func sameCount(a, b shift.Count) bool {
	if a.Total != b.Total {
		return false
	}
	da, db := slices.Clone(nonNil(a.Denominations)), slices.Clone(nonNil(b.Denominations))
	sortDenominations(da)
	sortDenominations(db)
	return slices.Equal(da, db)
}

func sortDenominations(d []shift.Denomination) {
	slices.SortFunc(d, func(x, y shift.Denomination) int { return cmp.Compare(y.Face, x.Face) })
}

func nonNil(d []shift.Denomination) []shift.Denomination {
	if d == nil {
		return []shift.Denomination{}
	}
	return d
}

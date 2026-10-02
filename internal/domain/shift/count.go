package shift

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// CountKind says which end of the shift a drawer count is from.
type CountKind string

// The counts a shift has.
const (
	CountOpening CountKind = "opening" // the float the shift starts with
	CountClosing CountKind = "closing" // the blind count at close (Task 2)
)

// CountKinds is every kind, for the generated vocabulary.
func CountKinds() []CountKind { return []CountKind{CountOpening, CountClosing} }

// ErrInvalidCount refuses a count that does not add up or cannot be cash.
var ErrInvalidCount = errors.New("invalid drawer count")

// Denomination is how many of one note or coin are in the drawer. Face is
// the note's value in minor units: 50000 for a ₹500 note.
type Denomination struct {
	Face  int64 `json:"face"`
	Count int64 `json:"count"`
}

// maxPieces bounds any one denomination's count: far past any real drawer,
// well short of anything that could overflow when multiplied by a face.
const maxPieces = 100_000

// Count is cash counted in the drawer. Denominations may be empty when the
// float is entered as one sum; when given, they must add up to Total.
type Count struct {
	Kind          CountKind      `json:"kind"`
	Denominations []Denomination `json:"denominations"`
	Total         money.Amount   `json:"total"`
}

// NewCount builds a count from denominations, working out the total. Each
// face appears once, and is listed largest first, as a drawer is counted.
func NewCount(kind CountKind, currency string, denominations []Denomination) (Count, error) {
	total, err := money.New(0, currency)
	if err != nil {
		return Count{}, fmt.Errorf("%w: %v", ErrInvalidCount, err)
	}
	sorted := slices.Clone(denominations)
	slices.SortFunc(sorted, func(a, b Denomination) int { return cmp.Compare(b.Face, a.Face) })
	for i, d := range sorted {
		if d.Face <= 0 {
			return Count{}, fmt.Errorf("%w: a note or coin of %d", ErrInvalidCount, d.Face)
		}
		if d.Count < 0 || d.Count > maxPieces {
			return Count{}, fmt.Errorf("%w: %d pieces of %d", ErrInvalidCount, d.Count, d.Face)
		}
		if i > 0 && sorted[i-1].Face == d.Face {
			return Count{}, fmt.Errorf("%w: %d is listed twice", ErrInvalidCount, d.Face)
		}
		face, err := money.New(d.Face, currency)
		if err != nil {
			return Count{}, fmt.Errorf("%w: %v", ErrInvalidCount, err)
		}
		value, err := face.Times(d.Count)
		if err != nil {
			return Count{}, fmt.Errorf("%w: %v", ErrInvalidCount, err)
		}
		if total, err = total.Add(value); err != nil {
			return Count{}, fmt.Errorf("%w: %v", ErrInvalidCount, err)
		}
	}
	if !slices.Contains(CountKinds(), kind) {
		return Count{}, fmt.Errorf("%w: kind %q", ErrInvalidCount, kind)
	}
	return Count{Kind: kind, Denominations: sorted, Total: total}, nil
}

// Validate checks a count as stored or received: a known kind, the venue's
// currency, nothing negative, and denominations that add up to the total.
func (c Count) Validate(currency string) error {
	if !slices.Contains(CountKinds(), c.Kind) {
		return fmt.Errorf("%w: kind %q", ErrInvalidCount, c.Kind)
	}
	if c.Total.Currency != currency {
		return fmt.Errorf("%w: counted in %q, the venue sells in %q", ErrInvalidCount, c.Total.Currency, currency)
	}
	if c.Total.Minor < 0 {
		return fmt.Errorf("%w: a drawer cannot hold %s", ErrInvalidCount, c.Total)
	}
	if len(c.Denominations) == 0 {
		return nil
	}
	recount, err := NewCount(c.Kind, currency, c.Denominations)
	if err != nil {
		return err
	}
	if recount.Total.Minor != c.Total.Minor {
		return fmt.Errorf("%w: the notes and coins add up to %s, not %s", ErrInvalidCount, recount.Total, c.Total)
	}
	return nil
}

// LumpSum is a count entered as one amount, with no denominations: an
// opening float someone has counted and types in whole.
func LumpSum(kind CountKind, total money.Amount) (Count, error) {
	c := Count{Kind: kind, Denominations: []Denomination{}, Total: total}
	return c, c.Validate(total.Currency)
}

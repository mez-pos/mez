package invoice

import (
	"fmt"
	"strings"
)

// Series is one invoice series as a venue configures it. A venue may run
// several, such as one for dine-in and one for delivery; each numbers
// independently, and restarts at 1 every financial year.
type Series struct {
	Code      string   // identifies the series within the venue: "NF", "DLV"
	Prefix    string   // written by {prefix}; may be empty
	FY        FYRule   // when the numbering restarts
	Template  Template // how a number is written
	MaxLength int      // longest number allowed; 0 for no limit, 16 for GST
}

// NewSeries builds and checks a series. It formats the first number of a
// sample year to prove the configuration can issue anything at all: a GST
// series whose very first number is 17 characters fails here, at setup, and
// not at the till on the first sale.
func NewSeries(code, prefix string, fy FYRule, template string, maxLength int) (Series, error) {
	if strings.TrimSpace(code) == "" {
		return Series{}, fmt.Errorf("%w: no series code", ErrInvalidSeries)
	}
	if maxLength < 0 {
		return Series{}, fmt.Errorf("%w: max length %d", ErrInvalidSeries, maxLength)
	}
	if err := fy.Validate(); err != nil {
		return Series{}, err
	}
	t, err := ParseTemplate(template)
	if err != nil {
		return Series{}, err
	}
	s := Series{Code: code, Prefix: prefix, FY: fy, Template: t, MaxLength: maxLength}
	if _, err := s.Number(FinancialYear{StartYear: 2026, Rule: fy}, 1); err != nil {
		return Series{}, fmt.Errorf("%w: cannot issue even its first number: %w", ErrInvalidSeries, err)
	}
	return s, nil
}

// Number writes invoice number seq of year fy in this series.
func (s Series) Number(fy FinancialYear, seq int64) (string, error) {
	n, err := s.Template.Format(s.Prefix, fy, seq)
	if err != nil {
		return "", err
	}
	if s.MaxLength > 0 && len(n) > s.MaxLength {
		return "", fmt.Errorf("%w: %q is %d characters, the series allows %d", ErrTooLong, n, len(n), s.MaxLength)
	}
	return n, nil
}

// Key names one gapless sequence: a venue's series in one financial year.
// It is the row an allocator locks.
type Key struct {
	VenueID int64
	Series  string
	FYStart int // FinancialYear.StartYear
}

// KeyFor is the key for this series at a venue in a year.
func (s Series) KeyFor(venueID int64, fy FinancialYear) Key {
	return Key{VenueID: venueID, Series: s.Code, FYStart: fy.StartYear}
}

// Validate refuses a key with no venue, no series or no year.
func (k Key) Validate() error {
	if k.VenueID < 1 || strings.TrimSpace(k.Series) == "" || k.FYStart < 1 {
		return fmt.Errorf("%w: %+v", ErrInvalidKey, k)
	}
	return nil
}

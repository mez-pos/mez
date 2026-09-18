package invoice

import (
	"fmt"
	"time"
)

// FYRule says when a venue's financial year starts. It is a venue setting:
// India starts on 1 April, most of the world on 1 January.
type FYRule struct {
	StartMonth time.Month
	StartDay   int // 1 to 28, so the start exists in every year
}

// The two rules the product ships with.
var (
	IndiaFY    = FYRule{StartMonth: time.April, StartDay: 1}
	CalendarFY = FYRule{StartMonth: time.January, StartDay: 1}
)

// Validate refuses a start that is not a real month, or a day some years
// lack. 29 February, or 31 April, would make the boundary move.
func (r FYRule) Validate() error {
	if r.StartMonth < time.January || r.StartMonth > time.December {
		return fmt.Errorf("%w: month %d", ErrInvalidFYRule, r.StartMonth)
	}
	if r.StartDay < 1 || r.StartDay > 28 {
		return fmt.Errorf("%w: day %d is not 1 to 28", ErrInvalidFYRule, r.StartDay)
	}
	return nil
}

// YearOf is the financial year that at falls in, judged at the venue: the
// boundary is local midnight in loc, not midnight UTC. A sale at 00:30 IST on
// 1 April is 19:00 UTC on 31 March, and it belongs to the new year.
func (r FYRule) YearOf(at time.Time, loc *time.Location) (FinancialYear, error) {
	if err := r.Validate(); err != nil {
		return FinancialYear{}, err
	}
	if loc == nil {
		return FinancialYear{}, ErrNoLocation
	}
	local := at.In(loc)
	year := local.Year()
	if local.Before(r.startIn(year, loc)) {
		year--
	}
	return FinancialYear{StartYear: year, Rule: r}, nil
}

// startIn is local midnight on the rule's start day in the given year.
func (r FYRule) startIn(year int, loc *time.Location) time.Time {
	return time.Date(year, r.StartMonth, r.StartDay, 0, 0, 0, 0, loc)
}

// spansOneYear reports whether the financial year is a calendar year, which
// decides how it is labelled.
func (r FYRule) spansOneYear() bool {
	return r.StartMonth == time.January && r.StartDay == 1
}

// FinancialYear is one year under a rule, named by the calendar year it
// starts in: under IndiaFY, StartYear 2026 runs 1 April 2026 to 31 March
// 2027.
type FinancialYear struct {
	StartYear int
	Rule      FYRule
}

// Label is how the year is written on an invoice and in a report: "2026-27"
// for a year that crosses a calendar year, "2026" for one that does not.
func (fy FinancialYear) Label() string {
	if fy.Rule.spansOneYear() {
		return fmt.Sprintf("%04d", fy.StartYear)
	}
	return fmt.Sprintf("%04d-%02d", fy.StartYear, (fy.StartYear+1)%100)
}

// ShortLabel is Label with two-digit years, "26-27" or "26", for series that
// must fit a length cap such as GST's 16 characters.
func (fy FinancialYear) ShortLabel() string {
	if fy.Rule.spansOneYear() {
		return fmt.Sprintf("%02d", fy.StartYear%100)
	}
	return fmt.Sprintf("%02d-%02d", fy.StartYear%100, (fy.StartYear+1)%100)
}

// Start is the first instant of the year, at the venue.
func (fy FinancialYear) Start(loc *time.Location) time.Time {
	return fy.Rule.startIn(fy.StartYear, loc)
}

// End is the first instant of the next year, at the venue: the year is
// [Start, End).
func (fy FinancialYear) End(loc *time.Location) time.Time {
	return fy.Rule.startIn(fy.StartYear+1, loc)
}

// Next is the year after this one.
func (fy FinancialYear) Next() FinancialYear {
	return FinancialYear{StartYear: fy.StartYear + 1, Rule: fy.Rule}
}

// Package report adds up a range of trading days (phase 10, Task 5a,
// ADR-118): the range's totals, sales over time and an hour by weekday
// heatmap. It counts through the Z-report's own builder, so a day's sales
// here are the same figures its Z-reports add up to.
//
// Where an event falls is decided by when it happened, not by its shift: a
// trading day runs from the venue's day start to the next, in its timezone,
// as the shift and the Z-report count it. A sale is placed by its invoice.
package report

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/shift"
	"github.com/mez-pos/mez/internal/domain/zreport"
)

// Every is how long one point of sales over time covers.
type Every string

// The periods. A week starts on a Monday; a month on its first.
const (
	Day   Every = "day"
	Week  Every = "week"
	Month Every = "month"
)

// Everys is every period, for the generated vocabulary.
func Everys() []Every { return []Every{Day, Week, Month} }

// ErrBadRange refuses a range that cannot be reported on.
var ErrBadRange = errors.New("report: the range cannot be read")

// MaxDays is the longest range one report covers.
const MaxDays = 366

const dayFormat = "2006-01-02"

// Range is the trading days a report covers, From to To inclusive, and how
// its sales over time are cut.
type Range struct {
	From, To string
	Every    Every
	Loc      *time.Location
	DayStart time.Duration
}

// Period is one point of sales over time: the trading days it covers, cut
// to the range (a week that starts before From starts at From).
type Period struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Point is a period's sales, and those sales by how they were served.
type Point struct {
	Period
	Sales     zreport.Sales   `json:"sales"`
	ByService []zreport.Group `json:"by_service"`
}

// Hour is one cell of the heatmap: the bills invoiced on one weekday of
// the range in one hour of the clock. Weekday is 1 for Monday to 7 for
// Sunday, the weekday of the trading day, so 01:00 on a Saturday is
// Friday night's trade. Only cells with a bill are listed.
type Hour struct {
	Weekday int          `json:"weekday"`
	Hour    int          `json:"hour"`
	Bills   int64        `json:"bills"`
	Total   money.Amount `json:"total"`
}

// Result is a range's reports.
type Result struct {
	From   string         `json:"from"`
	To     string         `json:"to"`
	Every  Every          `json:"every"`
	Totals zreport.Report `json:"totals"`
	Series []Point        `json:"series"`
	Hours  []Hour         `json:"hours"`
}

// Input is everything a range's reports are built from.
type Input struct {
	Range    Range
	Currency string
	Taxes    order.Taxes
	// Orders are the whole streams of the orders with any event in the
	// range: a refund in it may be against a payment taken before it.
	Orders [][]shift.StampedEvent
	// Categories are the menu's categories by item ID, as they stand now.
	Categories map[int64]zreport.Category
}

// The keys the reports are built under.
const totalKey = "all"

func periodKey(start string) string { return "p:" + start }
func hourKey(weekday, hour int) string {
	return "h:" + strconv.Itoa(weekday) + ":" + strconv.Itoa(hour)
}

// Build adds up a range.
func Build(in Input) (Result, error) {
	r := in.Range
	periods, err := r.Periods()
	if err != nil {
		return Result{}, err
	}
	want := []string{totalKey}
	for _, p := range periods {
		want = append(want, periodKey(p.Start))
	}
	reports, err := zreport.BuildEach(zreport.Input{
		Currency: in.Currency, Taxes: in.Taxes, Orders: in.Orders, Categories: in.Categories,
	}, want, r.keys)
	if err != nil {
		return Result{}, fmt.Errorf("report: %w", err)
	}

	out := Result{From: r.From, To: r.To, Every: r.Every, Totals: reports[totalKey], Series: make([]Point, 0, len(periods)), Hours: []Hour{}}
	for _, p := range periods {
		period := reports[periodKey(p.Start)]
		out.Series = append(out.Series, Point{Period: p, Sales: period.Sales, ByService: period.ByService})
	}
	for wd := 1; wd <= 7; wd++ {
		for h := range 24 {
			s, ok := reports[hourKey(wd, h)]
			if ok && s.Sales.Bills > 0 {
				out.Hours = append(out.Hours, Hour{Weekday: wd, Hour: h, Bills: s.Sales.Bills, Total: s.Sales.Total})
			}
		}
	}
	return out, nil
}

// keys places an event: in the totals when its trading day is in the range,
// and, for a sale or a credit note, in its period; a sale also in its hour.
// Only those feed the series and the heatmap, which read sales alone.
func (r Range) keys(se shift.StampedEvent) []string {
	// The range was checked in Periods, so the timezone and day start are good.
	day, err := shift.TradingDay(se.Event.At, r.Loc, r.DayStart)
	if err != nil || day < r.From || day > r.To {
		return nil
	}
	switch se.Event.Payload.(type) {
	case order.InvoiceIssued:
		local := se.Event.At.In(r.Loc)
		return []string{totalKey, periodKey(r.periodStart(day)), hourKey(isoWeekday(day), local.Hour())}
	case order.CreditNoteIssued:
		return []string{totalKey, periodKey(r.periodStart(day))}
	}
	return []string{totalKey}
}

// Periods is every period of the range, in order, empty ones included.
func (r Range) Periods() ([]Period, error) {
	if err := r.check(); err != nil {
		return nil, err
	}
	from, _ := time.Parse(dayFormat, r.From)
	to, _ := time.Parse(dayFormat, r.To)
	var out []Period
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		day := d.Format(dayFormat)
		start := r.periodStart(day)
		if len(out) > 0 && out[len(out)-1].Start == start {
			out[len(out)-1].End = day
			continue
		}
		out = append(out, Period{Start: start, End: day})
	}
	return out, nil
}

// periodStart is the first trading day of the period a day falls in, cut
// to the range.
func (r Range) periodStart(day string) string {
	d, _ := time.Parse(dayFormat, day)
	switch r.Every {
	case Week:
		d = d.AddDate(0, 0, -(isoWeekday(day) - 1))
	case Month:
		d = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	start := d.Format(dayFormat)
	if start < r.From {
		return r.From
	}
	return start
}

func (r Range) check() error {
	if r.Loc == nil {
		return fmt.Errorf("%w: no timezone", ErrBadRange)
	}
	if r.DayStart < 0 || r.DayStart >= 24*time.Hour {
		return fmt.Errorf("%w: the day start %s is not within the day", ErrBadRange, r.DayStart)
	}
	switch r.Every {
	case Day, Week, Month:
	default:
		return fmt.Errorf("%w: %q is not day, week or month", ErrBadRange, r.Every)
	}
	from, err := time.Parse(dayFormat, r.From)
	if err != nil {
		return fmt.Errorf("%w: %q is not a day like 2026-10-01", ErrBadRange, r.From)
	}
	to, err := time.Parse(dayFormat, r.To)
	if err != nil {
		return fmt.Errorf("%w: %q is not a day like 2026-10-01", ErrBadRange, r.To)
	}
	if to.Before(from) {
		return fmt.Errorf("%w: the range ends before it starts", ErrBadRange)
	}
	if to.Sub(from) >= MaxDays*24*time.Hour {
		return fmt.Errorf("%w: ask for a year or less at a time", ErrBadRange)
	}
	return nil
}

// isoWeekday is a trading day's weekday, 1 for Monday to 7 for Sunday.
func isoWeekday(day string) int {
	d, _ := time.Parse(dayFormat, day)
	if wd := int(d.Weekday()); wd != 0 {
		return wd
	}
	return 7
}

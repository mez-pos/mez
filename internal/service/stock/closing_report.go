package stock

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

// ErrBadRange is a range of trading days the closing stock cannot be read
// over.
var ErrBadRange = errors.New("the range cannot be read")

// MaxReportDays is the longest range one closing stock report covers.
const MaxReportDays = 366

// ClosingLine is one item's closing count: what the ledger expected, what
// was counted, and the difference in units and at cost.
type ClosingLine struct {
	StockItemID int64      `json:"stock_item_id"`
	Name        string     `json:"name"`
	Unit        stock.Unit `json:"unit"`
	Expected    stock.Qty  `json:"expected"`
	Counted     stock.Qty  `json:"counted"`
	// Variance is counted less expected: below zero is short.
	Variance stock.Qty `json:"variance"`
	// Cost is what a kilogram, a litre or one cost when it was counted;
	// absent when it was not known (ADR-124).
	Cost *money.Amount `json:"cost,omitempty"`
	// Value is the variance at that cost; absent when the cost was not
	// known. Never today's cost.
	Value *money.Amount `json:"value,omitempty"`
}

// ClosingShift is one shift's closing count.
type ClosingShift struct {
	ShiftID    string        `json:"shift_id"` // the shift's public id
	TradingDay string        `json:"trading_day"`
	Lines      []ClosingLine `json:"lines"`
	// Value is the lines' values added up, those with a cost; Uncosted is
	// how many had none, so a total is never read as the whole of it.
	Value    money.Amount `json:"value"`
	Uncosted int64        `json:"uncosted"`
}

// ClosingItemTotal is one item over the range: every close that counted it.
type ClosingItemTotal struct {
	StockItemID int64      `json:"stock_item_id"`
	Name        string     `json:"name"`
	Unit        stock.Unit `json:"unit"`
	Counts      int64      `json:"counts"`
	// Variance is its variances added up, in units.
	Variance stock.Qty    `json:"variance"`
	Value    money.Amount `json:"value"`
	Uncosted int64        `json:"uncosted"`
}

// ClosingReport is the closing stock over a range of trading days (phase
// 10, Task 5f2, ADR-125).
type ClosingReport struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Currency string `json:"currency"`
	// Shifts are the shifts with a closing count, oldest first.
	Shifts []ClosingShift `json:"shifts"`
	// Items are the items counted, the largest loss first.
	Items    []ClosingItemTotal `json:"items"`
	Value    money.Amount       `json:"value"`
	Uncosted int64              `json:"uncosted"`
}

// ClosingReport reads the closing counts of the shifts whose trading day is
// From to To, both inclusive, like "2026-10-01".
func (s *Service) ClosingReport(ctx context.Context, venueID int64, from, to string) (ClosingReport, error) {
	if err := checkDays(from, to); err != nil {
		return ClosingReport{}, err
	}
	q := s.db.Queries()
	currency, err := currencyOf(ctx, q, venueID)
	if err != nil {
		return ClosingReport{}, err
	}
	rows, err := q.ClosingCountsInRange(ctx, gen.ClosingCountsInRangeParams{VenueID: venueID, FromDay: from, ToDay: to})
	if err != nil {
		return ClosingReport{}, fmt.Errorf("stock: reading the closing counts from %s to %s: %w", from, to, err)
	}
	r, err := closingReport(rows, currency)
	if err != nil {
		return ClosingReport{}, err
	}
	r.From, r.To = from, to
	return r, nil
}

// ClosingTable is the report as a table for a CSV download: a row per item
// per shift.
func (s *Service) ClosingTable(ctx context.Context, venueID int64, from, to string) (export.Table, error) {
	r, err := s.ClosingReport(ctx, venueID, from, to)
	if err != nil {
		return export.Table{}, err
	}
	return r.Table(), nil
}

func checkDays(from, to string) error {
	const day = "2006-01-02"
	f, err := time.Parse(day, from)
	if err != nil {
		return fmt.Errorf("%w: %q is not a day like 2026-10-01", ErrBadRange, from)
	}
	t, err := time.Parse(day, to)
	if err != nil {
		return fmt.Errorf("%w: %q is not a day like 2026-10-01", ErrBadRange, to)
	}
	if t.Before(f) {
		return fmt.Errorf("%w: the range ends before it starts", ErrBadRange)
	}
	if t.Sub(f) >= MaxReportDays*24*time.Hour {
		return fmt.Errorf("%w: ask for a year or less at a time", ErrBadRange)
	}
	return nil
}

// closingReport groups the closing movements, which come by trading day,
// then shift, then item name.
func closingReport(rows []gen.ClosingCountsInRangeRow, currency string) (ClosingReport, error) {
	zero := money.Amount{Currency: currency}
	out := ClosingReport{Currency: currency, Shifts: []ClosingShift{}, Items: []ClosingItemTotal{}, Value: zero}
	items := map[int64]*ClosingItemTotal{}
	last := int64(0)
	for _, r := range rows {
		unit := stock.Unit(r.Unit)
		counted, delta := stock.Qty(r.Counted.Int64), stock.Qty(r.Delta)
		line := ClosingLine{
			StockItemID: r.StockItemID, Name: r.Name, Unit: unit,
			Expected: counted - delta, Counted: counted, Variance: delta,
		}
		if r.ShiftID != last || len(out.Shifts) == 0 {
			out.Shifts = append(out.Shifts, ClosingShift{ShiftID: r.ShiftPublicID, TradingDay: r.TradingDay, Lines: []ClosingLine{}, Value: zero})
			last = r.ShiftID
		}
		sh := &out.Shifts[len(out.Shifts)-1]
		it, ok := items[r.StockItemID]
		if !ok {
			it = &ClosingItemTotal{StockItemID: r.StockItemID, Name: r.Name, Unit: unit, Value: zero}
			items[r.StockItemID] = it
		}
		it.Counts++
		it.Variance += delta

		if r.UnitCost > 0 {
			worth, err := stock.Value(delta, unit, r.UnitCost)
			if err != nil {
				return ClosingReport{}, fmt.Errorf("stock: valuing the count of %s in shift %s: %w", r.Name, r.ShiftPublicID, err)
			}
			line.Cost = &money.Amount{Minor: r.UnitCost, Currency: currency}
			line.Value = &money.Amount{Minor: worth, Currency: currency}
			sh.Value.Minor += worth
			it.Value.Minor += worth
			out.Value.Minor += worth
		} else {
			sh.Uncosted++
			it.Uncosted++
			out.Uncosted++
		}
		sh.Lines = append(sh.Lines, line)
	}
	for _, it := range items {
		out.Items = append(out.Items, *it)
	}
	// The largest loss first: the most negative value, then the most
	// negative variance for items with no cost, then by name.
	slices.SortFunc(out.Items, func(a, b ClosingItemTotal) int {
		return cmp.Or(
			cmp.Compare(a.Value.Minor, b.Value.Minor), cmp.Compare(a.Variance, b.Variance),
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.StockItemID, b.StockItemID),
		)
	})
	return out, nil
}

// Table is the report as a table: a row per item per shift, quantities in
// whole units, money in major units with the currency in the header
// (ADR-119). A cost that was not known leaves its cells empty: not 0.00.
func (r ClosingReport) Table() export.Table {
	in := func(name string) string { return name + " (" + r.Currency + ")" }
	t := export.Table{
		Header: []string{"Trading day", "Shift", "Item", "Unit", "Expected", "Counted", "Variance", in("Cost"), "Cost of", in("Variance at cost")},
		Rows:   [][]string{},
	}
	for _, sh := range r.Shifts {
		for _, l := range sh.Lines {
			cost, of, value := "", "", ""
			if l.Cost != nil && l.Value != nil {
				cost, of, value = export.Decimal(*l.Cost), measureWord(l.Unit), export.Decimal(*l.Value)
			}
			t.Rows = append(t.Rows, []string{
				sh.TradingDay, sh.ShiftID, export.Text(l.Name), string(l.Unit),
				qtyText(l.Expected), qtyText(l.Counted), qtyText(l.Variance), cost, of, value,
			})
		}
	}
	return t
}

// measureWord is what a cost is quoted for, by the item's unit.
func measureWord(u stock.Unit) string {
	switch u {
	case stock.Gram:
		return "kg"
	case stock.Millilitre:
		return "l"
	}
	return "each"
}

// qtyText is a quantity in whole units as a plain decimal, "18.5" or
// "-250", from its thousandths: by digits, never a float.
func qtyText(q stock.Qty) string {
	sign := ""
	n := int64(q)
	if n < 0 {
		sign, n = "-", -n
	}
	whole, frac := n/int64(stock.PerUnit), n%int64(stock.PerUnit)
	if frac == 0 {
		return sign + strconv.FormatInt(whole, 10)
	}
	return sign + strconv.FormatInt(whole, 10) + "." + strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
}

package report

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/byjoellobo/mez/internal/domain/export"
	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/zreport"
)

// Sheet is one of a range's reports as a table: what a CSV download holds
// (phase 10, Task 5b, ADR-119).
type Sheet string

// The sheets.
const (
	SheetSummary    Sheet = "summary"    // the range's headline figures
	SheetOverTime   Sheet = "over_time"  // sales by period
	SheetItems      Sheet = "items"      // product mix
	SheetCategories Sheet = "categories" // category performance
	SheetChannels   Sheet = "channels"   // channel mix
	SheetStaff      Sheet = "staff"      // staff performance
	SheetPayments   Sheet = "payments"   // payment mix
	SheetTaxes      Sheet = "taxes"      // tax by component and rate
	SheetHours      Sheet = "hours"      // the hour by weekday heatmap
)

// Sheets is every sheet, for the generated vocabulary.
func Sheets() []Sheet {
	return []Sheet{
		SheetSummary, SheetOverTime, SheetItems, SheetCategories, SheetChannels,
		SheetStaff, SheetPayments, SheetTaxes, SheetHours,
	}
}

// ErrNoSheet refuses a sheet the reports do not have.
var ErrNoSheet = errors.New("report: no such report")

// Check refuses a sheet that is not one, so a caller can say so before
// reading a year of orders.
func (s Sheet) Check() error {
	if slices.Contains(Sheets(), s) {
		return nil
	}
	return fmt.Errorf("%w: %q", ErrNoSheet, string(s))
}

// Table is one of the range's reports as a table. Money is a plain decimal
// in major units, "1285.50", and each money column's header names the
// currency, so no cell does; days are trading days.
func (r Result) Table(s Sheet) (export.Table, error) {
	cur := r.Totals.Currency
	in := func(name string) string { return name + " (" + cur + ")" }
	switch s {
	case SheetSummary:
		return r.summary(in), nil
	case SheetOverTime:
		t := export.Table{Header: []string{
			"From", "To", "Bills", in("Gross"), in("Discounts"), in("Net"), in("Taxable"), in("Tax"), in("Total"), in("Net of credit notes"),
		}, Rows: [][]string{}}
		for _, p := range r.Series {
			ps := p.Sales
			t.Rows = append(t.Rows, []string{
				p.Start, p.End, count(ps.Bills), export.Decimal(ps.Gross), export.Decimal(ps.Discounts), export.Decimal(ps.Net),
				export.Decimal(ps.Taxable), export.Decimal(ps.Tax), export.Decimal(ps.Total), export.Decimal(ps.NetOfCredits),
			})
		}
		return t, nil
	case SheetItems:
		return groups("Item", "Qty", r.Totals.ByItem, in), nil
	case SheetCategories:
		return groups("Category", "Qty", r.Totals.ByCategory, in), nil
	case SheetChannels:
		return groups("Channel", "Bills", r.Totals.ByChannel, in), nil
	case SheetStaff:
		return groups("Opened by", "Bills", r.Totals.ByStaff, in), nil
	case SheetPayments:
		t := export.Table{Header: []string{"Method", "Tenders", in("Taken"), in("Voided"), in("Refunded"), in("Net")}, Rows: [][]string{}}
		for _, p := range r.Totals.Payments {
			t.Rows = append(t.Rows, []string{
				string(p.Method), count(p.Count), export.Decimal(p.Taken), export.Decimal(p.Voided), export.Decimal(p.Refunded), export.Decimal(p.Net),
			})
		}
		return t, nil
	case SheetTaxes:
		t := export.Table{Header: []string{"Tax", "Rate", in("Taxable"), in("Amount")}, Rows: [][]string{}}
		for _, l := range r.Totals.Taxes {
			t.Rows = append(t.Rows, []string{export.Text(l.Name), l.Rate.String(), export.Decimal(l.Taxable), export.Decimal(l.Amount)})
		}
		return t, nil
	case SheetHours:
		t := export.Table{Header: []string{"Weekday", "Hour", "Bills", in("Total")}, Rows: [][]string{}}
		for _, h := range r.Hours {
			// Weekday is 1 for Monday to 7 for Sunday; time.Weekday starts at Sunday.
			day := time.Weekday(h.Weekday % 7).String()
			t.Rows = append(t.Rows, []string{day, fmt.Sprintf("%02d:00", h.Hour), count(h.Bills), export.Decimal(h.Total)})
		}
		return t, nil
	}
	return export.Table{}, s.Check()
}

// summary is the range's headline figures, one a row: a count, an amount,
// or both.
func (r Result) summary(in func(string) string) export.Table {
	t := r.Totals
	amount := func(name string, a money.Amount) []string { return []string{name, "", export.Decimal(a)} }
	tally := func(name string, c zreport.Tally) []string {
		return []string{name, count(c.Count), export.Decimal(c.Amount)}
	}
	return export.Table{
		Header: []string{"Figure", "Count", in("Amount")},
		Rows: [][]string{
			{"Trading days", r.From + " to " + r.To, ""},
			{"Bills", count(t.Sales.Bills), ""},
			amount("Gross sales", t.Sales.Gross),
			amount("Discounts", t.Sales.Discounts),
			amount("Net sales", t.Sales.Net),
			amount("Taxable", t.Sales.Taxable),
			amount("Tax", t.Sales.Tax),
			amount("Total", t.Sales.Total),
			tally("Credit notes", t.CreditNotes),
			amount("Total net of credit notes", t.Sales.NetOfCredits),
			tally("Discounts given", t.Discounts),
			tally("Comps", t.Comps),
			tally("Voids", t.Voids),
			tally("Refunds", t.Refunds),
			amount("Payments", t.Balance.Payments),
			amount("Payments less sales", t.Balance.Difference),
			amount("Tips", t.Tips),
			{"Invoice copies printed", count(t.Reprints), ""},
		},
	}
}

// groups is sales under each heading of one kind.
func groups(heading, qty string, gs []zreport.Group, in func(string) string) export.Table {
	t := export.Table{Header: []string{heading, qty, in("Gross"), in("Discounts"), in("Net"), in("Tax"), in("Total")}, Rows: [][]string{}}
	for _, g := range gs {
		t.Rows = append(t.Rows, []string{
			export.Text(g.Name), count(g.Qty), export.Decimal(g.Gross), export.Decimal(g.Discounts),
			export.Decimal(g.Net), export.Decimal(g.Tax), export.Decimal(g.Total),
		})
	}
	return t
}

func count(n int64) string { return strconv.FormatInt(n, 10) }

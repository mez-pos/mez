package stock

import (
	"fmt"
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/sheet"
)

// Ingredients from a spreadsheet (phase 10, Task 10a, ADR-298): a CSV file
// with a header row and one stock item on each row under it. Reading it
// adds nothing. It answers a plan: the rows to add, the rows already in the
// store, and every problem with the row and column it is in.
//
// The rules are the menu import's (ADR-288): it adds and never changes, one
// bad row refuses the file, and nothing is guessed. A file gives an item no
// stock: stock arrives by Receive, so the ledger says where it came from.

// The columns, by the names the template gives them.
const (
	ColName         = "Name"
	ColUnit         = "Counted in"
	ColLow          = "Low at"
	ColCost         = "Cost"
	ColCountAtClose = "Count at close"
)

// ItemColumns is every column, in the template's order. The first two must
// be in the file.
var ItemColumns = []string{ColName, ColUnit, ColLow, ColCost, ColCountAtClose}

var itemHeadings = sheet.Headings{Columns: ItemColumns, Required: []string{ColName, ColUnit}}

// ItemsVenue is what a file of ingredients is read against.
type ItemsVenue struct {
	Currency string
	// Names are every stock item's name, retired ones included.
	Names []string
}

// ItemRow is one stock item to add. Cost is minor units of the venue's
// currency for a kilogram, a litre or one, by its unit; zero is not known.
type ItemRow struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row          int
	Name         string
	Unit         Unit
	LowLevel     Qty
	Cost         int64
	CountAtClose bool
}

// ItemSkip is a row left out because the store already has an item of that
// name, retired or not. An import adds; it never changes what is there.
type ItemSkip struct {
	Row  int
	Name string
}

// ItemsPlan is what importing a file of ingredients would do.
type ItemsPlan struct {
	Rows    []ItemRow
	Skipped []ItemSkip
	// Examples is how many rows are the template's own examples, left in
	// the file. They are left out (ADR-305).
	Examples int
	Problems []sheet.Problem
}

// OK reports whether the plan can be carried out: something to add and
// nothing wrong.
func (p ItemsPlan) OK() bool { return len(p.Problems) == 0 && len(p.Rows) > 0 }

// ReadItems reads a CSV file against the store and answers what importing
// it would do. It never fails: a file it cannot read is a plan with one
// problem, on row 0.
func ReadItems(data []byte, v ItemsVenue) ItemsPlan {
	file := func(message string) ItemsPlan { return ItemsPlan{Problems: []sheet.Problem{{Message: message}}} }
	records, problem := sheet.Read(data)
	if problem != "" {
		return file(problem)
	}
	columns, problem := itemHeadings.Read(records[0])
	if problem != "" {
		return file(problem)
	}

	r := itemsReader{currency: v.Currency, have: make(map[string]bool, len(v.Names)), firstRow: map[string]int{}}
	for _, name := range v.Names {
		r.have[nameKey(name)] = true
	}
	seen := sheet.Rows(records, columns, r.read)
	if problem := sheet.CountProblem(seen, "Import"); problem != "" {
		return file(problem)
	}
	return r.plan
}

// nameKey is a name as the store compares them: without regard to case, as
// its unique index does.
func nameKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

type itemsReader struct {
	currency string
	plan     ItemsPlan
	have     map[string]bool
	firstRow map[string]int
}

// read reads one row into the plan: an item to add, one already there, or
// its problems, every one of them, so the file is fixed in one pass.
func (r *itemsReader) read(row int, cells map[string]string) {
	var problems []sheet.Problem
	problem := func(column, message string) {
		problems = append(problems, sheet.Problem{Row: row, Column: column, Message: message})
	}

	name := cells[ColName]
	// A template's example is not the venue's ingredient, whatever else it says.
	if sheet.IsExample(name) {
		r.plan.Examples++
		return
	}
	// Which item it is comes first: a row for one already there is left
	// alone whatever else it says.
	if name != "" {
		key := nameKey(name)
		if r.have[key] {
			r.plan.Skipped = append(r.plan.Skipped, ItemSkip{Row: row, Name: name})
			return
		}
		if first, twice := r.firstRow[key]; twice {
			problem(ColName, fmt.Sprintf("%s is already in this file, on row %d. Keep one of them.", name, first))
		} else {
			r.firstRow[key] = row
		}
	}

	unit := importUnit(cells[ColUnit], problem)
	out := ItemRow{
		Row: row, Unit: unit,
		LowLevel:     importLow(cells[ColLow], problem),
		Cost:         importCost(cells[ColCost], r.currency, problem),
		CountAtClose: importYes(ColCountAtClose, cells[ColCountAtClose], problem),
	}
	// The name's own rules are the store's, through CleanItem; the unit and
	// the level were read above, and are given as they stand or not at all.
	draft, p := CleanItem(ItemDraft{Name: name, Unit: orGram(unit), LowLevel: out.LowLevel})
	if p != nil && p.Field == "name" {
		// First, where the column is.
		problems = append([]sheet.Problem{{Row: row, Column: ColName, Message: p.Message}}, problems...)
	}
	if len(problems) > 0 {
		r.plan.Problems = append(r.plan.Problems, problems...)
		return
	}
	out.Name = draft.Name
	r.plan.Rows = append(r.plan.Rows, out)
}

// orGram stands in for a unit that could not be read, so the rest of the
// row is still checked. Its own problem is already reported.
func orGram(u Unit) Unit {
	if u == "" {
		return Gram
	}
	return u
}

// importUnit reads what an item is counted in, however it is commonly
// typed. A kilogram and a litre are not units here: an amount in the store
// is in grams or millilitres, and guessing a thousandfold is not a guess to
// make.
func importUnit(cell string, problem sheet.Report) Unit {
	switch sheet.Fold(cell) {
	case "g", "gram", "grams", "gm", "gms":
		return Gram
	case "ml", "millilitre", "millilitres", "milliliter", "milliliters":
		return Millilitre
	case "each", "piece", "pieces", "pc", "pcs", "unit", "units":
		return Each
	case "":
		problem(ColUnit, "Say what it is counted in: g, ml or each.")
	case "kg", "kilogram", "kilograms", "kilo", "kilos":
		problem(ColUnit, "Stock is counted in grams, not kilograms. Type g here, and give the low level in grams.")
	case "l", "litre", "litres", "liter", "liters", "ltr":
		problem(ColUnit, "Stock is counted in millilitres, not litres. Type ml here, and give the low level in millilitres.")
	default:
		problem(ColUnit, fmt.Sprintf("%q is not something stock is counted in. Type g, ml or each.", cell))
	}
	return ""
}

// importLow reads the level an item is low at, in its unit. Empty is none.
func importLow(cell string, problem sheet.Report) Qty {
	if cell == "" {
		return 0
	}
	n, wrong := sheet.Decimal(cell, 3, "amount")
	switch {
	case wrong != "":
		problem(ColLow, wrong)
	case n < 0:
		problem(ColLow, "The low-stock level cannot be below zero.")
	case Qty(n) > MaxQty:
		problem(ColLow, fmt.Sprintf("%q is too large.", cell))
	default:
		return Qty(n)
	}
	return 0
}

// importCost reads what a kilogram, a litre or one of it costs. Empty is
// not known.
func importCost(cell, currency string, problem sheet.Report) int64 {
	if cell == "" {
		return 0
	}
	minor, wrong := sheet.Money(cell, currency, "cost")
	if wrong != "" {
		problem(ColCost, wrong)
		return 0
	}
	if p := CleanCost(minor); p != nil {
		problem(ColCost, p.Message)
		return 0
	}
	return minor
}

func importYes(column, cell string, problem sheet.Report) bool {
	yes, ok := sheet.Yes(cell)
	if !ok {
		problem(column, fmt.Sprintf("%q: type yes or no, or leave it empty.", cell))
	}
	return yes
}

// ItemsTemplate is the example rows of the file a venue fills in, a cell
// for each of ItemColumns, which is the header: one item counted each way,
// so the three units and what a cost is for are on the sheet. The costs
// have decimals only where the currency has them.
func ItemsTemplate(currency string) [][]string {
	cost := func(whole, withDecimals string) string {
		if money.Exponent(currency) == 0 {
			return whole
		}
		return withDecimals
	}
	return [][]string{
		// 900 for a kilogram, low at half a kilogram.
		{sheet.ExamplePrefix + " Coffee beans", "g", "500", cost("900", "900.00"), "no"},
		// 60 for a litre, low at two litres, counted as each shift closes.
		{sheet.ExamplePrefix + " Milk", "ml", "2000", cost("60", "60.00"), "yes"},
		// 7.50 for one.
		{sheet.ExamplePrefix + " Eggs", "each", "12", cost("8", "7.50"), ""},
	}
}

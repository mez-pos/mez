package menu

import (
	"strings"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// The menu as a file (phase 10, Task 8, ADR-293): every item on a row, in
// the import's own columns, so the file a venue downloads is one it can
// give back. ReadImport takes it as it comes and leaves every row alone.

// ColStatus is the one column the export has and the template does not:
// whether the item is on sale. The import accepts it and does not read it.
const ColStatus = "Status"

// What ColStatus says.
const (
	StatusActive  = "Active"
	StatusRetired = "Retired"
)

// ExportColumns is the export's header: the import's columns, then Status.
var ExportColumns = append(append([]string{}, ImportColumns...), ColStatus)

// ExportItem is an item as the export writes it: what it names by name, as
// a row of the import names them.
type ExportItem struct {
	Category    string
	Name        string
	Price       money.Amount
	TaxClass    string
	HSN         string
	Station     string
	Course      string
	Description string
	Diet        Diet
	GlutenFree  bool
	Popular     bool
	// Choices are the names of the groups it offers, in order, retired
	// groups left out: a row could not offer one.
	Choices []string
	Active  bool
}

// ExportRows is a row for each item, a cell for each of ExportColumns, in
// the order given.
func ExportRows(items []ExportItem) [][]string {
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{
			it.Category, it.Name, exportPrice(it.Price), it.TaxClass, it.HSN, it.Station, it.Course,
			it.Description, exportDiet(it.Diet), yesNo(it.GlutenFree), yesNo(it.Popular),
			strings.Join(it.Choices, ChoicesSeparator+" "), exportStatus(it.Active),
		}
	}
	return rows
}

// exportPrice is a price as the import reads one: the major unit with every
// decimal place the currency has, and no code. It is the canonical form
// with the code taken off, so no digit is worked out here.
func exportPrice(a money.Amount) string {
	return strings.TrimSuffix(a.String(), " "+a.Currency)
}

// exportDiet is a mark as an owner would type it, which importDiet reads.
func exportDiet(d Diet) string {
	switch d {
	case DietVeg:
		return "veg"
	case DietNonVeg:
		return "non-veg"
	case DietVegan:
		return "vegan"
	}
	return ""
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func exportStatus(active bool) string {
	if active {
		return StatusActive
	}
	return StatusRetired
}

package menu

import (
	"bytes"
	"encoding/csv"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
)

func exportFile(t *testing.T, items []ExportItem) []byte {
	t.Helper()
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	if err := w.WriteAll(append([][]string{ExportColumns}, ExportRows(items)...)); err != nil {
		t.Fatal(err)
	}
	return append([]byte(bom), b.Bytes()...)
}

func TestExportRows(t *testing.T) {
	rows := ExportRows([]ExportItem{
		{
			Category: "Coffee", Name: "Flat white", Price: inr(18000), TaxClass: "GST 5%", HSN: "2106", Station: "Coffee bar", Course: "Drinks",
			Description: "Double shot", Diet: DietVeg, GlutenFree: true, Popular: true, Choices: []string{"Size", "Milk"}, Active: true,
		},
		{Category: "Coffee", Name: "Mocha", Price: inr(12050), TaxClass: "GST 5%", Station: "Kitchen", Course: "Drinks", Diet: DietNonVeg},
		{Category: "Coffee", Name: "Water", Price: inr(0), Diet: DietVegan, Active: true},
	})
	want := [][]string{
		{"Coffee", "Flat white", "180.00", "GST 5%", "2106", "Coffee bar", "Drinks", "Double shot", "veg", "yes", "yes", "Size; Milk", "Active"},
		{"Coffee", "Mocha", "120.50", "GST 5%", "", "Kitchen", "Drinks", "", "non-veg", "no", "no", "", "Retired"},
		{"Coffee", "Water", "0.00", "", "", "", "", "", "vegan", "no", "no", "", "Active"},
	}
	for i := range want {
		if !slices.Equal(rows[i], want[i]) {
			t.Errorf("row %d\n got %q\nwant %q", i, rows[i], want[i])
		}
	}
	if len(rows[0]) != len(ExportColumns) {
		t.Errorf("a row has %d cells and there are %d columns", len(rows[0]), len(ExportColumns))
	}
}

func TestExportColumnsAreTheImportsAndStatus(t *testing.T) {
	if got := strings.Join(ExportColumns, ","); got != strings.Join(ImportColumns, ",")+",Status" {
		t.Errorf("columns: %s", got)
	}
	// Built from a copy: the export's extra column is not the template's.
	if slices.Contains(ImportColumns, ColStatus) {
		t.Error("the template has gained the Status column")
	}
}

func TestExportPricesInEveryKindOfCurrency(t *testing.T) {
	tests := []struct {
		currency string
		minor    int64
		want     string
	}{
		{"INR", 115000, "1150.00"},
		{"INR", 5, "0.05"},
		{"JPY", 1150, "1150"},
		{"KWD", 1500, "1.500"},
	}
	for _, tt := range tests {
		got := ExportRows([]ExportItem{{Price: money.Amount{Minor: tt.minor, Currency: tt.currency}}})[0][2]
		if got != tt.want {
			t.Errorf("%d %s is written %q, want %q", tt.minor, tt.currency, got, tt.want)
		}
		// And read back as the same amount.
		v := importVenue()
		v.Currency = tt.currency
		plan := ReadImport([]byte("Category,Name,Price,Tax class,HSN,Station,Course\nCoffee,New,"+got+",GST 5%,2106,Kitchen,Drinks\n"), v)
		if len(plan.Rows) != 1 || plan.Rows[0].Price != tt.minor {
			t.Errorf("%q read back: rows %+v, problems %+v", got, plan.Rows, plan.Problems)
		}
	}
}

// TestTheExportGoesBackIn: the menu's own export, given to the import, has
// nothing wrong with it and nothing to add, whatever is on the menu: an item
// in a retired category, one with a retired tax class, two of one name.
func TestTheExportGoesBackIn(t *testing.T) {
	v := importVenue()
	items := []ExportItem{
		{Category: "Coffee", Name: "Espresso", Price: inr(12000), TaxClass: "GST 5%", HSN: "2106", Station: "Kitchen", Course: "Drinks", Active: true},
		// In a retired category, itself retired, naming a retired tax class.
		{Category: "Winter specials", Name: "Mulled apple", Price: inr(22000), TaxClass: "Old rate", HSN: "2106", Station: "Kitchen", Course: "Drinks"},
		// No HSN, at a venue that needs one: it was added before the venue did.
		{Category: "Coffee", Name: `Chai, "cutting"`, Price: inr(4000), TaxClass: "GST 5%", Station: "Kitchen", Course: "Drinks", Diet: DietVeg, Active: true},
		{Category: "Coffee", Name: "Espresso", Price: inr(13000), TaxClass: "GST 5%", HSN: "2106", Station: "Kitchen", Course: "Drinks", Active: true},
	}
	v.Items = nil
	for _, it := range items {
		v.Items = append(v.Items, ImportHave{Category: it.Category, Name: it.Name})
	}

	plan := ReadImport(exportFile(t, items), v)
	if len(plan.Problems) != 0 {
		t.Fatalf("problems: %+v", plan.Problems)
	}
	if len(plan.Rows) != 0 || len(plan.NewCategories) != 0 || len(plan.Skipped) != len(items) {
		t.Errorf("rows %+v, new categories %v, skipped %d of %d", plan.Rows, plan.NewCategories, len(plan.Skipped), len(items))
	}
}

func TestTheExportWithARowAddedAddsOnlyThatRow(t *testing.T) {
	v := importVenue()
	file := string(exportFile(t, []ExportItem{
		{Category: "Coffee", Name: "Espresso", Price: inr(12000), TaxClass: "GST 5%", HSN: "2106", Station: "Kitchen", Course: "Drinks", Active: true},
	}))
	// The Status an owner types on a new row is not read: it is added, on sale.
	file += "Coffee,Cortado,170,GST 5%,2106,Kitchen,Drinks,,,,,,Retired\r\n"
	plan := ReadImport([]byte(file), v)
	if len(plan.Problems) != 0 || len(plan.Skipped) != 1 || len(plan.Rows) != 1 || plan.Rows[0].Name != "Cortado" {
		t.Errorf("rows %+v, skipped %+v, problems %+v", plan.Rows, plan.Skipped, plan.Problems)
	}
}

func TestANewItemStillCannotGoIntoARetiredCategory(t *testing.T) {
	plan := ReadImport([]byte(importHeader+"Winter specials,,180,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	var columns []string
	for _, p := range plan.Problems {
		columns = append(columns, p.Column)
	}
	// The category's problem is first, as its column is.
	if got := strings.Join(columns, "|"); got != "Category|Name" {
		t.Errorf("problems on %s, want Category|Name: %+v", got, plan.Problems)
	}
}

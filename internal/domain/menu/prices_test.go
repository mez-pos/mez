package menu

import (
	"strings"
	"testing"
)

// priced is a menu with prices to change: two coffees, a retired special,
// and two cakes that share a name, which the menu allows.
func priced() PriceVenue {
	return PriceVenue{
		Currency: "INR",
		Items: []PriceItem{
			{ID: 1, Category: "Coffee", Name: "Espresso", Price: 12000},
			{ID: 2, Category: "Coffee", Name: "Flat white", Price: 18000},
			{ID: 3, Category: "Winter specials", Name: "Mulled apple", Price: 22000},
			{ID: 4, Category: "Cakes", Name: "Brownie", Price: 9000},
			{ID: 5, Category: "Cakes", Name: "Brownie", Price: 9500},
			{ID: 6, Category: "Bakes", Name: "Espresso", Price: 5000},
		},
	}
}

const priceHeader = "Category,Name,Price\n"

func TestReadPricesFindsWhatChanged(t *testing.T) {
	plan := ReadPrices([]byte(priceHeader+
		"Coffee,Espresso,130\n"+
		"coffee, FLAT WHITE ,180.00\n"+
		"Winter specials,Mulled apple,\"₹ 1,150.50\"\n"+
		"Bakes,Espresso,0\n"), priced())

	if len(plan.Problems) != 0 || len(plan.Unknown) != 0 {
		t.Fatalf("problems %+v, unknown %+v", plan.Problems, plan.Unknown)
	}
	want := []PriceChange{
		{Row: 2, ItemID: 1, Category: "Coffee", Name: "Espresso", Old: 12000, New: 13000},
		// A retired item's price may be changed: it is the price it comes back at.
		{Row: 4, ItemID: 3, Category: "Winter specials", Name: "Mulled apple", Old: 22000, New: 115050},
		// The same name in another category is another item. Free is a price.
		{Row: 5, ItemID: 6, Category: "Bakes", Name: "Espresso", Old: 5000, New: 0},
	}
	if len(plan.Changes) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(plan.Changes), len(want), plan.Changes)
	}
	for i := range want {
		if plan.Changes[i] != want[i] {
			t.Errorf("change %d\n got %+v\nwant %+v", i, plan.Changes[i], want[i])
		}
	}
	if plan.Unchanged != 1 {
		t.Errorf("unchanged %d, want the flat white", plan.Unchanged)
	}
	if !plan.OK() {
		t.Error("a plan with changes and nothing wrong is not OK")
	}
}

func TestReadPricesTakesTheExportAsItComes(t *testing.T) {
	// Every column of the export, with only one price edited. Nothing but
	// the price is read: the tax class here is not one the venue has.
	file := string(exportFile(t, []ExportItem{
		{Category: "Coffee", Name: "Espresso", Price: inr(12500), TaxClass: "No such class", Station: "Nowhere", Course: "Never", Diet: "keto", Active: true},
		{Category: "Coffee", Name: "Flat white", Price: inr(18000), TaxClass: "GST 5%", Active: true},
	}))
	plan := ReadPrices([]byte(file), priced())
	if len(plan.Problems) != 0 || len(plan.Changes) != 1 || plan.Changes[0].New != 12500 || plan.Unchanged != 1 {
		t.Errorf("changes %+v, unchanged %d, problems %+v", plan.Changes, plan.Unchanged, plan.Problems)
	}
}

func TestReadPricesLeavesOutARowThatIsNotOnTheMenu(t *testing.T) {
	plan := ReadPrices([]byte(priceHeader+
		"Coffee,Espresso,130\n"+
		"Coffee,Cortado,170\n"+
		"Tiffin,Idli,80\n"+
		// Not checked further: its price is not a price, and it is not an item.
		"Coffee,Macchiato,lots\n"), priced())
	if len(plan.Problems) != 0 || len(plan.Changes) != 1 {
		t.Fatalf("changes %+v, problems %+v", plan.Changes, plan.Problems)
	}
	want := []PriceUnknown{{Row: 3, Category: "Coffee", Name: "Cortado"}, {Row: 4, Category: "Tiffin", Name: "Idli"}, {Row: 5, Category: "Coffee", Name: "Macchiato"}}
	if len(plan.Unknown) != len(want) {
		t.Fatalf("unknown: %+v", plan.Unknown)
	}
	for i := range want {
		if plan.Unknown[i] != want[i] {
			t.Errorf("unknown %d: got %+v, want %+v", i, plan.Unknown[i], want[i])
		}
	}
	if !plan.OK() {
		t.Error("rows that are not on the menu do not refuse the file")
	}
}

func TestReadPricesProblems(t *testing.T) {
	tests := []struct {
		name         string
		row          string
		column, says string
	}{
		{"no category", ",Espresso,130", "Category", "Give it a category"},
		{"no name", "Coffee,,130", "Name", "Give it a name"},
		{"no price", "Coffee,Espresso,", "Price", "Give it a price"},
		{"a price in words", "Coffee,Espresso,one thirty", "Price", "like 120 or 120.50"},
		{"a price below zero", "Coffee,Espresso,-1", "Price", "zero or more"},
		{"half a paisa", "Coffee,Espresso,130.005", "Price", "2 decimal places"},
		{"a decimal comma is not guessed at", "Coffee,Espresso,\"12,50\"", "Price", "point"},
		{"two items of one name", "Cakes,Brownie,95", "Name", "2 items"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadPrices([]byte(priceHeader+tt.row+"\n"), priced())
			if len(plan.Changes) != 0 {
				t.Errorf("a row with a problem changes nothing: %+v", plan.Changes)
			}
			if len(plan.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %+v", len(plan.Problems), plan.Problems)
			}
			p := plan.Problems[0]
			if p.Row != 2 || p.Column != tt.column || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, column %q, %q; want row 2, column %q, saying %q", p.Row, p.Column, p.Message, tt.column, tt.says)
			}
			if plan.OK() {
				t.Error("a plan with a problem is OK")
			}
		})
	}
}

func TestReadPricesTheSameItemTwice(t *testing.T) {
	tests := []struct {
		name, second string
	}{
		{"with two prices", "Coffee,espresso,140"},
		// One of them is the price it has. Which did the owner mean?
		{"once unchanged", "Coffee,Espresso,120"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadPrices([]byte(priceHeader+"Coffee,Espresso,130\n"+tt.second+"\n"), priced())
			if len(plan.Problems) != 1 || plan.Problems[0].Row != 3 || plan.Problems[0].Column != "Name" || !strings.Contains(plan.Problems[0].Message, "row 2") {
				t.Errorf("problems: %+v", plan.Problems)
			}
		})
	}
}

func TestReadPricesNothingChanged(t *testing.T) {
	plan := ReadPrices([]byte(priceHeader+"Coffee,Espresso,120\nCoffee,Flat white,180.00\n"), priced())
	if len(plan.Problems) != 0 || len(plan.Changes) != 0 || plan.Unchanged != 2 {
		t.Errorf("changes %+v, unchanged %d, problems %+v", plan.Changes, plan.Unchanged, plan.Problems)
	}
	if plan.OK() {
		t.Error("a plan that changes nothing is OK to apply")
	}
}

func TestReadPricesAWholeUnitCurrency(t *testing.T) {
	v := priced()
	v.Currency = "JPY"
	plan := ReadPrices([]byte(priceHeader+"Coffee,Espresso,13000\nCoffee,Flat white,180.5\n"), v)
	if len(plan.Changes) != 1 || plan.Changes[0].New != 13000 {
		t.Errorf("changes: %+v", plan.Changes)
	}
	if len(plan.Problems) != 1 || !strings.Contains(plan.Problems[0].Message, "no decimal places") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadPricesFilesItCannotRead(t *testing.T) {
	tests := []struct {
		name, data, says string
	}{
		{"empty", "", "empty"},
		{"a header and nothing under it", priceHeader, "no rows"},
		{"no Price column", "Category,Name\nCoffee,Espresso\n", "Price"},
		{"a column it does not know", "Category,Name,Price,New price\nCoffee,Espresso,120,130\n", "New price"},
		{"a spreadsheet, not a CSV", "PK\x03\x04\x14\x00\x06\x00", "CSV UTF-8"},
		{"a quote left open", priceHeader + "Coffee,\"Espresso,130\n", "could not be read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadPrices([]byte(tt.data), priced())
			if len(plan.Changes) != 0 || len(plan.Problems) != 1 {
				t.Fatalf("changes %+v, problems %+v", plan.Changes, plan.Problems)
			}
			if p := plan.Problems[0]; p.Row != 0 || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, %q; want the file's problem, saying %q", p.Row, p.Message, tt.says)
			}
		})
	}
}

func TestReadPricesTooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString(priceHeader)
	for range MaxImportRows + 1 {
		b.WriteString("Tiffin,Idli,80\n")
	}
	plan := ReadPrices([]byte(b.String()), priced())
	if len(plan.Unknown) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Row != 0 || !strings.Contains(plan.Problems[0].Message, "1000") {
		t.Errorf("unknown %d, problems %+v", len(plan.Unknown), plan.Problems)
	}
}

func TestReadPricesCountsRowsAsTheSpreadsheetDoes(t *testing.T) {
	plan := ReadPrices([]byte(priceHeader+",,\nCoffee,Espresso,130\n,,\nCoffee,Flat white,abc\n"), priced())
	if len(plan.Changes) != 1 || plan.Changes[0].Row != 3 {
		t.Errorf("changes: %+v", plan.Changes)
	}
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 5 {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

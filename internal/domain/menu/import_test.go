package menu

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/sheet"
)

// cafe is a venue with enough on it to import against: two of everything a
// row may name, one retired category, and two items already on the menu.
func importVenue() ImportVenue {
	return ImportVenue{
		Currency: "INR",
		NeedsHSN: true,
		Categories: []ImportChoice{
			{ID: 1, Name: "Coffee", Active: true},
			{ID: 2, Name: "Winter specials", Active: false},
		},
		TaxClasses: []ImportChoice{
			{ID: 10, Name: "GST 5%", Active: true},
			{ID: 11, Name: "GST 18%", Active: true},
			{ID: 12, Name: "Old rate", Active: false},
		},
		Stations: []ImportChoice{
			{ID: 20, Name: "Kitchen", Also: "kitchen", Active: true},
			{ID: 21, Name: "Coffee bar", Also: "bar", Active: true},
		},
		Courses: []ImportChoice{
			{ID: 30, Name: "Drinks", Also: "1", Active: true},
			{ID: 31, Name: "Mains", Also: "2", Active: true},
		},
		Groups: []ImportChoice{
			{ID: 40, Name: "Size", Active: true},
			{ID: 41, Name: "Milk", Active: true},
			{ID: 42, Name: "Old extras", Active: false},
			{ID: 43, Name: "Toppings", Active: true},
			{ID: 44, Name: "toppings", Active: true},
		},
		Items: []ImportHave{
			{Category: "Coffee", Name: "Espresso"},
			{Category: "Winter specials", Name: "Mulled apple"},
		},
	}
}

// bom is the byte order mark a spreadsheet puts at the start of a UTF-8 file.
const bom = string(rune(0xFEFF))

const importHeader = "Category,Name,Price,Tax class,HSN,Station,Course,Description,Diet,Gluten free,Popular\n"

func TestReadImportAddsRows(t *testing.T) {
	plan := ReadImport([]byte(importHeader+
		"Coffee,Latte,180,GST 5%,2106,Coffee bar,Drinks,Steamed milk,veg,yes,no\n"+
		"Bakes,Banana bread,120.50,GST 5%,1905,Kitchen,Mains,,Vegan,,Y\n"+
		"bakes,Lemon tart,\"1,150\",gst 18%,1905,KITCHEN,2,,,,\n"), importVenue())

	if len(plan.Problems) != 0 {
		t.Fatalf("problems: %+v", plan.Problems)
	}
	if len(plan.Rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(plan.Rows))
	}
	latte := plan.Rows[0]
	want := ImportRow{
		Row: 2, Category: "Coffee", CategoryID: 1, Name: "Latte", Price: 18000, HSN: "2106",
		TaxClassID: 10, StationID: 21, CourseID: 30, Description: "Steamed milk", Diet: DietVeg, GlutenFree: true,
	}
	if !reflect.DeepEqual(latte, want) {
		t.Errorf("latte\n got %+v\nwant %+v", latte, want)
	}
	bread := plan.Rows[1]
	if bread.Price != 12050 || bread.Diet != DietVegan || !bread.Popular || bread.CategoryID != 0 || bread.Category != "Bakes" {
		t.Errorf("banana bread: %+v", bread)
	}
	tart := plan.Rows[2]
	// The second spelling of a new category is the first's; the course is found by its number.
	if tart.Category != "Bakes" || tart.Price != 115000 || tart.TaxClassID != 11 || tart.StationID != 20 || tart.CourseID != 31 {
		t.Errorf("lemon tart: %+v", tart)
	}
	if got := strings.Join(plan.NewCategories, "|"); got != "Bakes" {
		t.Errorf("new categories %q, want Bakes once", got)
	}
}

func TestReadImportProblems(t *testing.T) {
	tests := []struct {
		name string
		row  string
		// The column the problem is on and a piece of what it says.
		column, says string
	}{
		{"no category", ",Latte,180,GST 5%,2106,Kitchen,Drinks,,,,", "Category", "Give it a category"},
		{"no name", "Coffee,,180,GST 5%,2106,Kitchen,Drinks,,,,", "Name", "Give it a name"},
		{"name too long", "Coffee," + strings.Repeat("x", MaxNameLength+1) + ",180,GST 5%,2106,Kitchen,Drinks,,,,", "Name", "60 characters"},
		{"name over two lines", "Coffee,\"Latte\nlarge\",180,GST 5%,2106,Kitchen,Drinks,,,,", "Name", "one line"},
		{"retired category", "Winter specials,Hot toddy,180,GST 5%,2106,Kitchen,Drinks,,,,", "Category", "retired"},
		{"no price", "Coffee,Latte,,GST 5%,2106,Kitchen,Drinks,,,,", "Price", "Give it a price"},
		{"price in words", "Coffee,Latte,one eighty,GST 5%,2106,Kitchen,Drinks,,,,", "Price", "like 120 or 120.50"},
		{"negative price", "Coffee,Latte,-5,GST 5%,2106,Kitchen,Drinks,,,,", "Price", "zero or more"},
		{"half a paisa", "Coffee,Latte,180.505,GST 5%,2106,Kitchen,Drinks,,,,", "Price", "2 decimal places"},
		{"a decimal comma is not guessed at", "Coffee,Latte,\"12,50\",GST 5%,2106,Kitchen,Drinks,,,,", "Price", "point"},
		{"two decimal commas", "Coffee,Latte,\"1,2,50\",GST 5%,2106,Kitchen,Drinks,,,,", "Price", "point"},
		{"a price too large to hold", "Coffee,Latte,99999999999999999999,GST 5%,2106,Kitchen,Drinks,,,,", "Price", "like 120 or 120.50"},
		{"unknown tax class", "Coffee,Latte,180,VAT,2106,Kitchen,Drinks,,,,", "Tax class", "GST 5%, GST 18%"},
		{"retired tax class", "Coffee,Latte,180,Old rate,2106,Kitchen,Drinks,,,,", "Tax class", "retired"},
		{"tax class left out with two to pick from", "Coffee,Latte,180,,2106,Kitchen,Drinks,,,,", "Tax class", "Say which"},
		{"unknown station", "Coffee,Latte,180,GST 5%,2106,Grill,Drinks,,,,", "Station", "Kitchen, Coffee bar"},
		{"unknown course", "Coffee,Latte,180,GST 5%,2106,Kitchen,Dessert,,,,", "Course", "Drinks, Mains"},
		{"no HSN where invoices show one", "Coffee,Latte,180,GST 5%,,Kitchen,Drinks,,,,", "HSN", "4, 6 or 8 digits"},
		{"a malformed HSN", "Coffee,Latte,180,GST 5%,21,Kitchen,Drinks,,,,", "HSN", "4, 6 or 8 digits"},
		{"description too long", "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks," + strings.Repeat("d", MaxDescriptionLength+1) + ",,,", "Description", "140 characters"},
		{"description over two lines", "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"hot\nmilky\",,,", "Description", "one line"},
		{"unknown mark", "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,halal,,", "Diet", "veg, non-veg or vegan"},
		{"gluten free is yes or no", "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,maybe,", "Gluten free", "yes or no"},
		{"popular is yes or no", "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,sometimes", "Popular", "yes or no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadImport([]byte(importHeader+tt.row+"\n"), importVenue())
			if len(plan.Rows) != 0 {
				t.Errorf("a row with a problem is not a row to add: %+v", plan.Rows)
			}
			if len(plan.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %+v", len(plan.Problems), plan.Problems)
			}
			p := plan.Problems[0]
			if p.Row != 2 || p.Column != tt.column || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, column %q, %q; want row 2, column %q, saying %q", p.Row, p.Column, p.Message, tt.column, tt.says)
			}
		})
	}
}

func TestReadImportReportsEveryProblemOnARow(t *testing.T) {
	plan := ReadImport([]byte(importHeader+",,abc,VAT,,Grill,Dessert,,,,\n"), importVenue())
	var columns []string
	for _, p := range plan.Problems {
		columns = append(columns, p.Column)
	}
	if got, want := strings.Join(columns, "|"), "Category|Name|Price|Tax class|HSN|Station|Course"; got != want {
		t.Errorf("problems on %s, want %s", got, want)
	}
}

func TestReadImportPrices(t *testing.T) {
	tests := []struct {
		cell string
		want int64
	}{
		{"180", 18000},
		{"180.5", 18050},
		{"180.50", 18050},
		{"0", 0},
		{" 180 ", 18000},
		{"₹180", 18000},
		{"₹ 1,150.00", 115000},
		{"Rs. 180", 18000},
		{"Rs 180", 18000},
		{"INR 180", 18000},
		{"180 INR", 18000},
		{"1,00,000", 10000000},
	}
	for _, tt := range tests {
		t.Run(tt.cell, func(t *testing.T) {
			plan := ReadImport([]byte("Category,Name,Price,Tax class,HSN,Station,Course\nCoffee,Latte,\""+tt.cell+"\",GST 5%,2106,Kitchen,Drinks\n"), importVenue())
			if len(plan.Problems) != 0 || len(plan.Rows) != 1 {
				t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
			if got := plan.Rows[0].Price; got != tt.want {
				t.Errorf("got %d minor units, want %d", got, tt.want)
			}
		})
	}
}

func TestReadImportAWholeUnitCurrency(t *testing.T) {
	v := importVenue()
	v.Currency = "JPY"
	plan := ReadImport([]byte(importHeader+"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\nCoffee,Mocha,180.5,GST 5%,2106,Kitchen,Drinks,,,,\n"), v)
	if len(plan.Rows) != 1 || plan.Rows[0].Price != 180 {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Problems) != 1 || !strings.Contains(plan.Problems[0].Message, "no decimal places") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadImportLeavesWhatIsThereAlone(t *testing.T) {
	plan := ReadImport([]byte(importHeader+
		"coffee, ESPRESSO ,999,GST 5%,2106,Kitchen,Drinks,,,,\n"+
		"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	if len(plan.Problems) != 0 {
		t.Fatalf("problems: %+v", plan.Problems)
	}
	if len(plan.Rows) != 1 || plan.Rows[0].Name != "Latte" {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0] != (ImportSkip{Row: 2, Category: "Coffee", Name: "ESPRESSO"}) {
		t.Errorf("skipped: %+v", plan.Skipped)
	}
}

func TestReadImportARowAlreadyThereIsNotChecked(t *testing.T) {
	// Its price is not a price, but nothing will be done with it.
	plan := ReadImport([]byte(importHeader+"Coffee,Espresso,free,,,,,,,,\n"), importVenue())
	if len(plan.Problems) != 0 || len(plan.Skipped) != 1 {
		t.Errorf("skipped %+v, problems %+v", plan.Skipped, plan.Problems)
	}
}

func TestReadImportTheSameItemTwice(t *testing.T) {
	plan := ReadImport([]byte(importHeader+
		"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n"+
		"Coffee,latte,200,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	if len(plan.Rows) != 1 {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 3 || plan.Problems[0].Column != "Name" || !strings.Contains(plan.Problems[0].Message, "row 2") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadImportOneChoiceNeedsNoCell(t *testing.T) {
	v := importVenue()
	v.NeedsHSN = false
	v.TaxClasses = v.TaxClasses[:1]
	v.Stations = v.Stations[:1]
	v.Courses = v.Courses[:1]
	plan := ReadImport([]byte("Name,Price,Category\nLatte,180,Coffee\n"), v)
	if len(plan.Problems) != 0 || len(plan.Rows) != 1 {
		t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
	}
	if r := plan.Rows[0]; r.TaxClassID != 10 || r.StationID != 20 || r.CourseID != 30 || r.HSN != "" {
		t.Errorf("row: %+v", r)
	}
}

func TestReadImportNothingToChooseFrom(t *testing.T) {
	v := importVenue()
	v.Stations = nil
	plan := ReadImport([]byte(importHeader+"Coffee,Latte,180,GST 5%,2106,,Drinks,,,,\n"), v)
	if len(plan.Problems) != 1 || plan.Problems[0].Column != "Station" || !strings.Contains(plan.Problems[0].Message, "Add one") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadImportAnHSNIsOptionalWhereInvoicesDoNotShowIt(t *testing.T) {
	v := importVenue()
	v.NeedsHSN = false
	plan := ReadImport([]byte(importHeader+
		"Coffee,Latte,180,GST 5%,,Kitchen,Drinks,,,,\n"+
		"Coffee,Mocha,180,GST 5%,2106,Kitchen,Drinks,,,,\n"+
		"Coffee,Cortado,180,GST 5%,21,Kitchen,Drinks,,,,\n"), v)
	if len(plan.Rows) != 2 {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 4 || plan.Problems[0].Column != "HSN" {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadImportMarks(t *testing.T) {
	tests := []struct {
		cell string
		want Diet
	}{
		{"", DietNone}, {"none", DietNone}, {"veg", DietVeg}, {"Vegetarian", DietVeg},
		{"non-veg", DietNonVeg}, {"Non Veg", DietNonVeg}, {"non_veg", DietNonVeg}, {"NONVEG", DietNonVeg},
		{"vegan", DietVegan},
	}
	for _, tt := range tests {
		t.Run(tt.cell, func(t *testing.T) {
			plan := ReadImport([]byte(importHeader+"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,"+tt.cell+",,\n"), importVenue())
			if len(plan.Rows) != 1 || plan.Rows[0].Diet != tt.want {
				t.Errorf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
		})
	}
}

func TestReadImportAsASpreadsheetSavesIt(t *testing.T) {
	const row = "Coffee;Latte;180;GST 5%;2106;Kitchen;Drinks;Steamed; with care;veg;yes;no"
	tests := []struct {
		name string
		data string
	}{
		{"a byte order mark", bom + importHeader + "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"Steamed; with care\",veg,yes,no\n"},
		{"semicolons", strings.ReplaceAll(importHeader, ",", ";") + strings.Replace(row, "Steamed; with care", "\"Steamed; with care\"", 1) + "\n"},
		{"tabs", strings.ReplaceAll(importHeader, ",", "\t") + "Coffee\tLatte\t180\tGST 5%\t2106\tKitchen\tDrinks\tSteamed; with care\tveg\tyes\tno\n"},
		{"Windows line ends", strings.ReplaceAll(importHeader+"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"Steamed; with care\",veg,yes,no\n", "\n", "\r\n")},
		{"headers in another case and order", "NAME, price ,category,TAX CLASS,hsn,station,course,description,diet,gluten_free,popular\nLatte,180,Coffee,GST 5%,2106,Kitchen,Drinks,\"Steamed; with care\",veg,yes,no\n"},
		{"empty rows between and after", importHeader + ",,,,,,,,,,\nCoffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"Steamed; with care\",veg,yes,no\n,,,,,,,,,,\n\n"},
		{"a short row", importHeader + "Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"Steamed; with care\",veg,yes\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadImport([]byte(tt.data), importVenue())
			if len(plan.Problems) != 0 || len(plan.Rows) != 1 {
				t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
			r := plan.Rows[0]
			if r.Name != "Latte" || r.Price != 18000 || r.CategoryID != 1 || r.Description != "Steamed; with care" || r.Diet != DietVeg || !r.GlutenFree {
				t.Errorf("row: %+v", r)
			}
		})
	}
}

func TestReadImportCountsRowsAsTheSpreadsheetDoes(t *testing.T) {
	plan := ReadImport([]byte(importHeader+
		",,,,,,,,,,\n"+
		"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n"+
		"Coffee,Mocha,abc,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	if len(plan.Rows) != 1 || plan.Rows[0].Row != 3 {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 4 {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

func TestReadImportFilesItCannotRead(t *testing.T) {
	tests := []struct {
		name string
		data string
		says string
	}{
		{"empty", "", "empty"},
		{"only spaces", " \r\n \n", "empty"},
		{"a importHeader and nothing under it", importHeader, "no rows"},
		{"a importHeader and only empty rows", importHeader + ",,,\n,,,\n", "no rows"},
		{"no Name column", "Category,Price\nCoffee,180\n", "Name"},
		{"no Price or Category column", "Name\nLatte\n", "Category and Price"},
		{"a column it does not know", importHeader[:len(importHeader)-1] + ",Notes\nCoffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,,x\n", "Notes"},
		{"a column twice", "Category,Name,Price,Name\nCoffee,Latte,180,Mocha\n", "twice"},
		{"saved in an old encoding", importHeader + "Coffee,Caf\xe9 au lait,180,GST 5%,2106,Kitchen,Drinks,,,,\n", "CSV UTF-8"},
		{"saved as Unicode text", "\xff\xfeC\x00a\x00t\x00", "CSV UTF-8"},
		{"a spreadsheet, not a CSV", "PK\x03\x04\x14\x00\x06\x00", "CSV UTF-8"},
		{"a quote left open", importHeader + "Coffee,\"Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n", "could not be read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadImport([]byte(tt.data), importVenue())
			if len(plan.Rows) != 0 {
				t.Errorf("rows from a file that cannot be read: %+v", plan.Rows)
			}
			if len(plan.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %+v", len(plan.Problems), plan.Problems)
			}
			p := plan.Problems[0]
			if p.Row != 0 || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, %q; want the file's problem, saying %q", p.Row, p.Message, tt.says)
			}
		})
	}
}

func TestReadImportTooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("Category,Name,Price,Tax class,HSN,Station,Course\n")
	for i := range MaxImportRows + 1 {
		b.WriteString("Coffee,Item ")
		b.WriteString(strings.Repeat("i", i%7+1))
		b.WriteString(",1,GST 5%,2106,Kitchen,Drinks\n")
	}
	plan := ReadImport([]byte(b.String()), importVenue())
	if len(plan.Rows) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Row != 0 || !strings.Contains(plan.Problems[0].Message, "1000") {
		t.Errorf("rows %d, problems %+v", len(plan.Rows), plan.Problems)
	}
}

func TestImportPlanOK(t *testing.T) {
	good := ReadImport([]byte(importHeader+"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	bad := ReadImport([]byte(importHeader+"Coffee,Latte,abc,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	onlyThere := ReadImport([]byte(importHeader+"Coffee,Espresso,180,GST 5%,2106,Kitchen,Drinks,,,,\n"), importVenue())
	if !good.OK() || bad.OK() || onlyThere.OK() {
		t.Errorf("good %v, bad %v, nothing to add %v; want true, false, false", good.OK(), bad.OK(), onlyThere.OK())
	}
}

// templateFile is the template as the back office downloads it: the columns
// and the example rows, as CSV.
func templateFile(t *testing.T, v ImportVenue) []byte {
	t.Helper()
	return csvFile(t, ImportColumns, renamed(ImportTemplate(v), 1))
}

// renamed is a template's rows as an owner makes them theirs: the cell at
// name without the example's mark, so the row is read and not left out.
func renamed(rows [][]string, name int) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = slices.Clone(row)
		out[i][name] = strings.TrimSpace(strings.TrimPrefix(row[name], sheet.ExamplePrefix))
	}
	return out
}

// csvFile is rows under a header as a spreadsheet saves them.
func csvFile(t *testing.T, header []string, rows [][]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	if err := w.WriteAll(append([][]string{header}, rows...)); err != nil {
		t.Fatal(err)
	}
	return append([]byte(bom), b.Bytes()...)
}

func TestImportTemplateImportsAsItComes(t *testing.T) {
	empty := ImportVenue{Currency: "INR"}
	yen := importVenue()
	yen.Currency = "JPY"
	dinar := importVenue()
	dinar.Currency = "KWD"
	plain := importVenue()
	plain.NeedsHSN = false
	awkward := importVenue()
	awkward.Categories = []ImportChoice{{ID: 1, Name: `Chai, "cutting"`, Active: true}}
	awkward.TaxClasses = []ImportChoice{{ID: 10, Name: "GST 5%, food", Active: true}}
	retiredFirst := importVenue()
	retiredFirst.Stations[0].Active = false

	tests := []struct {
		name  string
		venue ImportVenue
		// What the rows name; 0 where the venue has nothing for them to name.
		category, taxClass, station, course int64
		problems                            int
	}{
		{"a venue with a menu", importVenue(), 1, 10, 20, 30, 0},
		{"a venue whose invoices show no HSN", plain, 1, 10, 20, 30, 0},
		{"a currency with no decimal places", yen, 1, 10, 20, 30, 0},
		{"a currency with three", dinar, 1, 10, 20, 30, 0},
		{"names with commas and quotes in them", awkward, 1, 10, 20, 30, 0},
		{"the first station retired", retiredFirst, 1, 10, 21, 30, 0},
		// Nothing to name: a tax class, a station and a course each, on each row.
		{"a venue with nothing set up", empty, 0, 0, 0, 0, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadImport(templateFile(t, tt.venue), tt.venue)
			if len(plan.Problems) != tt.problems {
				t.Fatalf("got %d problems, want %d: %+v", len(plan.Problems), tt.problems, plan.Problems)
			}
			if tt.problems > 0 {
				return
			}
			if len(plan.Rows) != 2 {
				t.Fatalf("got %d rows, want the 2 examples: %+v", len(plan.Rows), plan.Rows)
			}
			for _, r := range plan.Rows {
				if r.CategoryID != tt.category || r.TaxClassID != tt.taxClass || r.StationID != tt.station || r.CourseID != tt.course {
					t.Errorf("row %d names category %d, tax class %d, station %d, course %d; want %d, %d, %d, %d",
						r.Row, r.CategoryID, r.TaxClassID, r.StationID, r.CourseID, tt.category, tt.taxClass, tt.station, tt.course)
				}
				if r.Price <= 0 || r.Name == "" {
					t.Errorf("row %d: %+v", r.Row, r)
				}
			}
		})
	}
}

func TestImportTemplateShowsEveryColumn(t *testing.T) {
	rows := ImportTemplate(importVenue())
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	for i, row := range rows {
		if len(row) != len(ImportColumns) {
			t.Errorf("row %d has %d cells, want one for each of the %d columns", i, len(row), len(ImportColumns))
		}
	}
	// The first row fills every cell, so the owner sees what each takes; the
	// second leaves out what may be left out.
	for i, cell := range rows[0] {
		if cell == "" {
			t.Errorf("the first example leaves %s empty", ImportColumns[i])
		}
	}
	if rows[1][7] != "" || rows[1][8] != "" {
		t.Errorf("the second example fills the optional cells: %q", rows[1])
	}
}

func TestImportTemplateNamesACategoryForAnEmptyMenu(t *testing.T) {
	v := importVenue()
	v.Categories = []ImportChoice{{ID: 2, Name: "Winter specials", Active: false}}
	plan := ReadImport(templateFile(t, v), v)
	if len(plan.Problems) != 0 || len(plan.NewCategories) != 1 || plan.NewCategories[0] != "Food" {
		t.Errorf("new categories %v, problems %+v", plan.NewCategories, plan.Problems)
	}
}

// TestExampleRowsAreLeftOut: the template's own rows, left in a file, are
// not the venue's items. They are counted and nothing else about them is
// read, so they are never a problem and never on the menu.
func TestExampleRowsAreLeftOut(t *testing.T) {
	v := importVenue()
	asItComes := ReadImport(csvFile(t, ImportColumns, ImportTemplate(v)), v)
	if asItComes.Examples != 2 || len(asItComes.Rows) != 0 || len(asItComes.Problems) != 0 || len(asItComes.NewCategories) != 0 {
		t.Errorf("the template as it comes: %+v", asItComes)
	}
	if asItComes.OK() {
		t.Error("a file of only examples is OK to import")
	}

	// Among an owner's own rows, and whatever else an example says.
	plan := ReadImport([]byte(importHeader+
		"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,,,,\n"+
		"Nowhere,EXAMPLE: anything,not a price,No tax,,,,,,,\n"+
		", example:Masala dosa ,,,,,,,,,\n"+
		// An item that only mentions the word is the venue's own.
		"Coffee,Good example cake,90,GST 5%,2106,Kitchen,Drinks,,,,\n"), v)
	if plan.Examples != 2 || len(plan.Problems) != 0 || len(plan.Rows) != 2 || plan.Rows[1].Name != "Good example cake" {
		t.Errorf("examples %d, rows %+v, problems %+v", plan.Examples, plan.Rows, plan.Problems)
	}
}

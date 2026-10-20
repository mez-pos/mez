package stock

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/sheet"
)

func store() ItemsVenue {
	return ItemsVenue{Currency: "INR", Names: []string{"Coffee beans", "Old syrup"}}
}

const itemsHeader = "Name,Counted in,Low at,Cost,Count at close\n"

func TestReadItemsAddsRows(t *testing.T) {
	plan := ReadItems([]byte(itemsHeader+
		"Milk,ml,2000,60,yes\n"+
		" Eggs ,EACH,12,7.50,\n"+
		"Sugar,grams,\"1,500.5\",\"₹ 45\",no\n"+
		"Salt,g,,,\n"), store())
	if len(plan.Problems) != 0 || len(plan.Skipped) != 0 {
		t.Fatalf("problems %+v, skipped %+v", plan.Problems, plan.Skipped)
	}
	want := []ItemRow{
		{Row: 2, Name: "Milk", Unit: Millilitre, LowLevel: 2000 * PerUnit, Cost: 6000, CountAtClose: true},
		{Row: 3, Name: "Eggs", Unit: Each, LowLevel: 12 * PerUnit, Cost: 750},
		{Row: 4, Name: "Sugar", Unit: Gram, LowLevel: 1500500, Cost: 4500},
		// No level and no cost: never low, and not costed.
		{Row: 5, Name: "Salt", Unit: Gram},
	}
	if len(plan.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(plan.Rows), len(want), plan.Rows)
	}
	for i := range want {
		if plan.Rows[i] != want[i] {
			t.Errorf("row %d\n got %+v\nwant %+v", i, plan.Rows[i], want[i])
		}
	}
	if !plan.OK() {
		t.Error("a plan with rows and nothing wrong is not OK")
	}
}

func TestReadItemsProblems(t *testing.T) {
	tests := []struct {
		name, row    string
		column, says string
	}{
		{"no name", ",g,500,900,", "Name", "Give it a name"},
		{"name too long", strings.Repeat("x", MaxNameLength+1) + ",g,500,900,", "Name", "60 characters"},
		{"no unit", "Milk,,500,900,", "Counted in", "g, ml or each"},
		{"a unit it does not know", "Milk,cups,500,900,", "Counted in", "not something stock is counted in"},
		{"kilograms are not guessed at", "Rice,kg,5,90,", "Counted in", "grams, not kilograms"},
		{"litres are not guessed at", "Milk,Litres,5,60,", "Counted in", "millilitres, not litres"},
		{"a level in words", "Milk,ml,plenty,60,", "Low at", "not an amount"},
		{"a level below zero", "Milk,ml,-5,60,", "Low at", "below zero"},
		{"a level finer than a thousandth", "Milk,ml,0.0005,60,", "Low at", "3 decimal places"},
		{"a level with a decimal comma", "Milk,ml,\"1,5\",60,", "Low at", "point"},
		{"a level too large to keep", "Milk,ml,99999999999999,60,", "Low at", "too large"},
		{"a cost in words", "Milk,ml,500,cheap,", "Cost", "not a cost"},
		{"a cost below zero", "Milk,ml,500,-1,", "Cost", "below zero"},
		{"half a paisa", "Milk,ml,500,60.005,", "Cost", "2 decimal places"},
		{"a cost with a decimal comma", "Milk,ml,500,\"12,50\",", "Cost", "point"},
		{"a cost too large to keep", "Milk,ml,500,99999999999,", "Cost", "too large"},
		{"count at close is yes or no", "Milk,ml,500,60,sometimes", "Count at close", "yes or no"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadItems([]byte(itemsHeader+tt.row+"\n"), store())
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

func TestReadItemsReportsEveryProblemOnARowInColumnOrder(t *testing.T) {
	plan := ReadItems([]byte(itemsHeader+",cups,plenty,cheap,sometimes\n"), store())
	var columns []string
	for _, p := range plan.Problems {
		columns = append(columns, p.Column)
	}
	if got, want := strings.Join(columns, "|"), "Name|Counted in|Low at|Cost|Count at close"; got != want {
		t.Errorf("problems on %s, want %s", got, want)
	}
}

func TestReadItemsLeavesWhatIsThereAlone(t *testing.T) {
	// Without regard to case, as the index on the names is; a retired item
	// is there too. Neither row is checked further.
	plan := ReadItems([]byte(itemsHeader+"COFFEE BEANS,cups,,,\nold syrup,ml,,,\nMilk,ml,,,\n"), store())
	if len(plan.Problems) != 0 || len(plan.Rows) != 1 || plan.Rows[0].Name != "Milk" {
		t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
	}
	if len(plan.Skipped) != 2 || plan.Skipped[0] != (ItemSkip{Row: 2, Name: "COFFEE BEANS"}) || plan.Skipped[1].Row != 3 {
		t.Errorf("skipped: %+v", plan.Skipped)
	}
}

func TestReadItemsTheSameItemTwice(t *testing.T) {
	plan := ReadItems([]byte(itemsHeader+"Milk,ml,,,\nmilk,ml,,,\n"), store())
	if len(plan.Rows) != 1 || len(plan.Problems) != 1 || plan.Problems[0].Row != 3 || plan.Problems[0].Column != "Name" || !strings.Contains(plan.Problems[0].Message, "row 2") {
		t.Errorf("rows %+v, problems %+v", plan.Rows, plan.Problems)
	}
}

func TestReadItemsNeedsOnlyANameAndAUnit(t *testing.T) {
	plan := ReadItems([]byte("counted_in;NAME\r\nml;Milk\r\n"), store())
	if len(plan.Problems) != 0 || len(plan.Rows) != 1 || plan.Rows[0] != (ItemRow{Row: 2, Name: "Milk", Unit: Millilitre}) {
		t.Errorf("rows %+v, problems %+v", plan.Rows, plan.Problems)
	}
}

func TestReadItemsFilesItCannotRead(t *testing.T) {
	tests := []struct{ name, data, says string }{
		{"empty", "", "empty"},
		{"a header and nothing under it", itemsHeader, "no rows"},
		{"no unit column", "Name,Cost\nMilk,60\n", "Counted in"},
		{"a column it does not know", "Name,Counted in,In stock\nMilk,ml,5000\n", "In stock"},
		{"a spreadsheet, not a CSV", "PK\x03\x04\x14\x00", "CSV UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadItems([]byte(tt.data), store())
			if len(plan.Rows) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Row != 0 || !strings.Contains(plan.Problems[0].Message, tt.says) {
				t.Errorf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
			if plan.OK() {
				t.Error("a file that cannot be read is OK")
			}
		})
	}
}

func TestReadItemsACurrencyWithNoDecimals(t *testing.T) {
	v := store()
	v.Currency = "JPY"
	plan := ReadItems([]byte(itemsHeader+"Milk,ml,,180,\nRice,g,,180.5,\n"), v)
	if len(plan.Rows) != 1 || plan.Rows[0].Cost != 180 {
		t.Errorf("rows: %+v", plan.Rows)
	}
	if len(plan.Problems) != 1 || !strings.Contains(plan.Problems[0].Message, "no decimal places") {
		t.Errorf("problems: %+v", plan.Problems)
	}
}

// TestItemsTemplateImportsAsItComes: the template, given straight back, has
// nothing wrong with it and adds its examples, one counted each way.
func TestItemsTemplateImportsAsItComes(t *testing.T) {
	for _, currency := range []string{"INR", "JPY", "KWD"} {
		t.Run(currency, func(t *testing.T) {
			var b bytes.Buffer
			w := csv.NewWriter(&b)
			w.UseCRLF = true
			// As an owner makes it theirs: the names without the example's mark.
			if err := w.WriteAll(append([][]string{ItemColumns}, named(ItemsTemplate(currency))...)); err != nil {
				t.Fatal(err)
			}
			plan := ReadItems(append([]byte("\xef\xbb\xbf"), b.Bytes()...), ItemsVenue{Currency: currency})
			if len(plan.Problems) != 0 || len(plan.Rows) != 3 {
				t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
			if plan.Rows[0].Unit != Gram || plan.Rows[1].Unit != Millilitre || plan.Rows[2].Unit != Each {
				t.Errorf("units: %v %v %v", plan.Rows[0].Unit, plan.Rows[1].Unit, plan.Rows[2].Unit)
			}
			for _, r := range plan.Rows {
				if r.Cost <= 0 || r.LowLevel <= 0 || r.Name == "" {
					t.Errorf("row: %+v", r)
				}
			}
			for _, row := range ItemsTemplate(currency) {
				if len(row) != len(ItemColumns) {
					t.Errorf("a row has %d cells and there are %d columns", len(row), len(ItemColumns))
				}
			}
		})
	}
}

// named is the template's rows with real names: the example's mark taken off.
func named(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = append([]string{strings.TrimSpace(strings.TrimPrefix(row[0], sheet.ExamplePrefix))}, row[1:]...)
	}
	return out
}

// TestExampleIngredientsAreLeftOut: the template's own rows, left in a
// file, are counted and not added, whatever else they say.
func TestExampleIngredientsAreLeftOut(t *testing.T) {
	var b bytes.Buffer
	if err := csv.NewWriter(&b).WriteAll(append([][]string{ItemColumns}, ItemsTemplate("INR")...)); err != nil {
		t.Fatal(err)
	}
	asItComes := ReadItems(b.Bytes(), store())
	if asItComes.Examples != 3 || len(asItComes.Rows) != 0 || len(asItComes.Problems) != 0 || asItComes.OK() {
		t.Errorf("the template as it comes: %+v", asItComes)
	}
	plan := ReadItems([]byte(itemsHeader+"Milk,ml,,,\nEXAMPLE: Rice,kg,lots,cheap,maybe\n"), store())
	if plan.Examples != 1 || len(plan.Problems) != 0 || len(plan.Rows) != 1 {
		t.Errorf("examples %d, rows %+v, problems %+v", plan.Examples, plan.Rows, plan.Problems)
	}
}

package menu

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
)

func choosing() ChoicesVenue {
	return ChoicesVenue{Currency: "INR", Groups: []string{"Milk", "Old extras"}}
}

const choicesHeader = "Group,Fewest,Most,Choice,Price,Picked to start\n"

func TestReadChoicesAddsGroups(t *testing.T) {
	plan := ReadChoices([]byte(choicesHeader+
		"Size,1,1,Regular,0,yes\n"+
		"Extras,0,2, Extra shot ,40,\n"+
		// A group's rows need not be together, and say its bounds once.
		"size,,,Large,30.50,no\n"+
		"Extras,,,No cheese,-20,\n"+
		"Extras,0,2,Syrup,,\n"), choosing())
	if len(plan.Problems) != 0 || len(plan.Skipped) != 0 {
		t.Fatalf("problems %+v, skipped %+v", plan.Problems, plan.Skipped)
	}
	want := []GroupImport{
		{Row: 2, Group: GroupDraft{Name: "Size", MinSelect: 1, MaxSelect: 1, Choices: []ChoiceDraft{
			{Name: "Regular", Price: 0, IsDefault: true}, {Name: "Large", Price: 3050},
		}}},
		{Row: 3, Group: GroupDraft{Name: "Extras", MinSelect: 0, MaxSelect: 2, Choices: []ChoiceDraft{
			// Below zero takes off; an empty price is nothing.
			{Name: "Extra shot", Price: 4000}, {Name: "No cheese", Price: -2000}, {Name: "Syrup", Price: 0},
		}}},
	}
	if !reflect.DeepEqual(plan.Groups, want) {
		t.Errorf("groups\n got %+v\nwant %+v", plan.Groups, want)
	}
	if !plan.OK() {
		t.Error("a plan with groups and nothing wrong is not OK")
	}
}

func TestReadChoicesProblems(t *testing.T) {
	tests := []struct {
		name string
		rows string
		// The row and column of the one problem, and a piece of what it says.
		row          int
		column, says string
	}{
		{"no group", ",1,1,Regular,0,", 2, "Group", "Say which group"},
		{"no choice", "Size,1,1,,0,", 2, "Choice", "Give the choice a name"},
		{"a group name too long", strings.Repeat("x", MaxNameLength+1) + ",1,1,Regular,0,", 2, "Group", "60 characters"},
		{"a choice name too long", "Size,1,1," + strings.Repeat("x", MaxNameLength+1) + ",0,", 2, "Choice", "60 characters"},
		{"a price in words", "Size,1,1,Large,extra,", 2, "Price", "not a price"},
		{"a price with a decimal comma", "Size,1,1,Large,\"12,50\",", 2, "Price", "point"},
		{"picked to start is yes or no", "Size,1,1,Large,30,maybe", 2, "Picked to start", "yes or no"},
		{"fewest is a whole number", "Size,one,1,Large,30,", 2, "Fewest", "whole number"},
		{"most is a whole number", "Size,1,1.5,Large,30,", 2, "Most", "whole number"},
		{"fewest below zero", "Size,-1,1,Large,30,", 2, "Fewest", "whole number"},
		{"no fewest on any row", "Size,,1,Regular,0,\nSize,,,Large,30,", 2, "Fewest", "fewest of Size"},
		{"no most on any row", "Size,1,,Regular,0,\nSize,,,Large,30,", 2, "Most", "most of Size"},
		{"two rows that disagree", "Size,1,1,Regular,0,\nSize,1,2,Large,30,", 3, "Most", "Row 2 says 1"},
		{"most of none", "Size,0,0,Regular,0,", 2, "Most", "at least one"},
		{"most below fewest", "Size,2,1,Regular,0,\nSize,,,Large,30,", 2, "Most", "fewer than the fewest"},
		{"asks for more than it offers", "Size,2,2,Regular,0,", 2, "Fewest", "offers only 1"},
		{"two choices of one name", "Size,1,1,Regular,0,\nSize,,,regular,30,", 3, "Choice", "Two choices"},
		{"more picked to start than may be picked", "Size,1,1,Regular,0,yes\nSize,,,Large,30,yes", 2, "Picked to start", "at most 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadChoices([]byte(choicesHeader+tt.rows+"\n"), choosing())
			if len(plan.Groups) != 0 {
				t.Errorf("a group with a problem is not a group to add: %+v", plan.Groups)
			}
			if len(plan.Problems) != 1 {
				t.Fatalf("got %d problems, want 1: %+v", len(plan.Problems), plan.Problems)
			}
			p := plan.Problems[0]
			if p.Row != tt.row || p.Column != tt.column || !strings.Contains(p.Message, tt.says) {
				t.Errorf("got row %d, column %q, %q; want row %d, column %q, saying %q", p.Row, p.Column, p.Message, tt.row, tt.column, tt.says)
			}
		})
	}
}

func TestReadChoicesPartOfAGroupIsNotAGroup(t *testing.T) {
	// One row of Size is wrong: its good row is not added as a group of one.
	plan := ReadChoices([]byte(choicesHeader+"Size,1,1,Regular,0,\nSize,,,Large,lots,\nExtras,0,1,Syrup,30,\n"), choosing())
	if len(plan.Problems) != 1 || plan.Problems[0].Row != 3 {
		t.Fatalf("problems: %+v", plan.Problems)
	}
	if len(plan.Groups) != 1 || plan.Groups[0].Group.Name != "Extras" {
		t.Errorf("groups: %+v", plan.Groups)
	}
	if plan.OK() {
		t.Error("a plan with a problem is OK")
	}
}

func TestReadChoicesLeavesAGroupThatIsThereAlone(t *testing.T) {
	// Without regard to case, a retired one too, and not checked further.
	plan := ReadChoices([]byte(choicesHeader+"MILK,x,y,,lots,maybe\nold extras,0,1,Syrup,30,\nSize,1,1,Regular,0,\n"), choosing())
	if len(plan.Problems) != 0 || len(plan.Groups) != 1 || plan.Groups[0].Group.Name != "Size" {
		t.Fatalf("groups %+v, problems %+v", plan.Groups, plan.Problems)
	}
	if len(plan.Skipped) != 2 || plan.Skipped[0] != (GroupSkip{Row: 2, Group: "MILK"}) || plan.Skipped[1].Row != 3 {
		t.Errorf("skipped: %+v", plan.Skipped)
	}
}

func TestReadChoicesFilesItCannotRead(t *testing.T) {
	tests := []struct{ name, data, says string }{
		{"empty", "", "empty"},
		{"a header and nothing under it", choicesHeader, "no rows"},
		{"no Most column", "Group,Fewest,Choice\nSize,1,Regular\n", "Most"},
		{"a column it does not know", "Group,Fewest,Most,Choice,Default\nSize,1,1,Regular,yes\n", "Default"},
		{"a spreadsheet, not a CSV", "PK\x03\x04\x14\x00", "CSV UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadChoices([]byte(tt.data), choosing())
			if len(plan.Groups) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Row != 0 || !strings.Contains(plan.Problems[0].Message, tt.says) {
				t.Errorf("groups %+v, problems %+v", plan.Groups, plan.Problems)
			}
		})
	}
}

// TestChoicesTemplateImportsAsItComes: the template, given straight back,
// has nothing wrong with it and adds its two example groups.
func TestChoicesTemplateImportsAsItComes(t *testing.T) {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.UseCRLF = true
	if err := w.WriteAll(append([][]string{ChoiceColumns}, ChoicesTemplate()...)); err != nil {
		t.Fatal(err)
	}
	for _, currency := range []string{"INR", "JPY"} {
		plan := ReadChoices(append([]byte(bom), b.Bytes()...), ChoicesVenue{Currency: currency})
		if len(plan.Problems) != 0 || len(plan.Groups) != 2 {
			t.Fatalf("%s: groups %+v, problems %+v", currency, plan.Groups, plan.Problems)
		}
		size, extras := plan.Groups[0].Group, plan.Groups[1].Group
		if size.MinSelect != 1 || size.MaxSelect != 1 || len(size.Choices) != 2 || !size.Choices[0].IsDefault {
			t.Errorf("%s: size %+v", currency, size)
		}
		if extras.MinSelect != 0 || extras.MaxSelect != 2 || len(extras.Choices) != 2 {
			t.Errorf("%s: extras %+v", currency, extras)
		}
	}
	for _, row := range ChoicesTemplate() {
		if len(row) != len(ChoiceColumns) {
			t.Errorf("a row has %d cells and there are %d columns", len(row), len(ChoiceColumns))
		}
	}
}

// The Choices column of the items import: the groups an item offers.

const withChoices = "Category,Name,Price,Tax class,HSN,Station,Course,Choices\n"

func TestReadImportOffersTheGroupsARowNames(t *testing.T) {
	plan := ReadImport([]byte(withChoices+
		"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\"MILK ; size\"\n"+
		"Coffee,Mocha,180,GST 5%,2106,Kitchen,Drinks,Size;\n"+
		"Coffee,Cortado,180,GST 5%,2106,Kitchen,Drinks,\n"), importVenue())
	if len(plan.Problems) != 0 || len(plan.Rows) != 3 {
		t.Fatalf("rows %+v, problems %+v", plan.Rows, plan.Problems)
	}
	// In the order the cell gives them, which is the order the sheet shows them.
	if got := plan.Rows[0].Groups; !reflect.DeepEqual(got, []int64{41, 40}) {
		t.Errorf("latte offers %v, want Milk then Size", got)
	}
	if got := plan.Rows[1].Groups; !reflect.DeepEqual(got, []int64{40}) {
		t.Errorf("mocha offers %v, want Size", got)
	}
	if got := plan.Rows[2].Groups; len(got) != 0 {
		t.Errorf("cortado offers %v, want none", got)
	}
}

func TestReadImportChoicesProblems(t *testing.T) {
	tests := []struct{ name, cell, says string }{
		{"a group the menu does not have", "Sweetness", "Import the choices first"},
		{"a retired group", "Old extras", "retired"},
		{"two groups of one name", "Toppings", "2 groups of choices are called"},
		{"the same group twice", "Size; size", "twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := ReadImport([]byte(withChoices+"Coffee,Latte,180,GST 5%,2106,Kitchen,Drinks,\""+tt.cell+"\"\n"), importVenue())
			if len(plan.Rows) != 0 || len(plan.Problems) != 1 || plan.Problems[0].Column != "Choices" || !strings.Contains(plan.Problems[0].Message, tt.says) {
				t.Errorf("rows %+v, problems %+v", plan.Rows, plan.Problems)
			}
		})
	}
}

func TestImportTemplateNamesAGroupWhereThereIsOne(t *testing.T) {
	rows := ImportTemplate(importVenue())
	if got := rows[0][len(rows[0])-1]; got != "Size" {
		t.Errorf("the first example offers %q, want the first group in use", got)
	}
	none := importVenue()
	none.Groups = nil
	if got := ImportTemplate(none)[0][len(ImportColumns)-1]; got != "" {
		t.Errorf("with no groups the cell is %q, want empty", got)
	}
}

// TestTheExportWithChoicesGoesBackIn: an export that names an item's groups
// is still a file the import leaves alone, and a new row may name them too.
func TestTheExportWithChoicesGoesBackIn(t *testing.T) {
	v := importVenue()
	file := string(exportFile(t, []ExportItem{
		{Category: "Coffee", Name: "Espresso", Price: inr(12000), TaxClass: "GST 5%", HSN: "2106", Station: "Kitchen", Course: "Drinks", Choices: []string{"Size", "Milk"}, Active: true},
	}))
	if !strings.Contains(file, "Size; Milk") {
		t.Fatalf("the export does not name the groups: %s", file)
	}
	file += "Coffee,Cortado,170,GST 5%,2106,Kitchen,Drinks,,,,,Milk; Size,Active\r\n"
	plan := ReadImport([]byte(file), v)
	if len(plan.Problems) != 0 || len(plan.Skipped) != 1 || len(plan.Rows) != 1 || !reflect.DeepEqual(plan.Rows[0].Groups, []int64{41, 40}) {
		t.Errorf("rows %+v, skipped %+v, problems %+v", plan.Rows, plan.Skipped, plan.Problems)
	}
}

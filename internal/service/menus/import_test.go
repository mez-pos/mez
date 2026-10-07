package menus

import (
	"context"
	"encoding/csv"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/menu"
	csvsheet "github.com/byjoellobo/mez/internal/domain/sheet"
)

// sheet is a file as the fixture's venues would write it: one tax class,
// one active station and one course, so those cells may be left empty.
const sheet = "Category,Name,Price,HSN,Station,Course,Description,Diet,Gluten free,Popular\n" +
	"Coffee,Flat white,180,2106,,,Double shot,veg,,yes\n" +
	"Bakes,Banana bread,120.50,1905,kitchen,1,,vegan,yes,\n" +
	"bakes,Lemon tart,150,1905,Kitchen,Mains,,,,\n"

func (w *world) importMenu(t *testing.T, venue int64, data string) Import {
	t.Helper()
	out, err := w.s.ImportMenu(context.Background(), venue, []byte(data))
	if err != nil {
		t.Fatalf("importing: %v (problems %+v)", err, out.Problems)
	}
	return out
}

func TestPreviewImportWritesNothing(t *testing.T) {
	w := newWorld(t)
	w.category(t, plain, "Coffee")
	w.nudged = nil
	before := w.version(t, plain)

	out, err := w.s.PreviewImport(context.Background(), plain, []byte(sheet))
	if err != nil {
		t.Fatal(err)
	}
	if out.Imported || len(out.ToAdd) != 3 || len(out.Problems) != 0 || len(out.Skipped) != 0 {
		t.Errorf("preview: %+v", out)
	}
	if !slices.Equal(out.NewCategories, []string{"Bakes"}) {
		t.Errorf("new categories %v, want Bakes", out.NewCategories)
	}
	if got := out.ToAdd[1]; got.Row != 3 || got.Category != "Bakes" || got.Name != "Banana bread" || got.Price.Minor != 12050 || got.Price.Currency != "INR" {
		t.Errorf("second row: %+v", got)
	}
	if e := w.editor(t, plain); len(e.Items) != 0 || len(e.Categories) != 1 {
		t.Errorf("a preview wrote: %d items, %d categories", len(e.Items), len(e.Categories))
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: a preview is not an edit", got, before, w.nudged)
	}
}

func TestImportMenuAddsEverythingAsOneEdit(t *testing.T) {
	w := newWorld(t)
	coffee := w.category(t, plain, "Coffee")
	w.create(t, plain, w.item(plain, coffee, "Espresso", 12000, ""))
	w.nudged = nil
	before := w.version(t, plain)

	out := w.importMenu(t, plain, sheet)
	if !out.Imported || len(out.ToAdd) != 3 {
		t.Fatalf("import: %+v", out)
	}
	if got := w.version(t, plain); got != before+1 {
		t.Errorf("version %d, want %d: one import is one edit", got, before+1)
	}
	if !slices.Equal(w.nudged, []int64{plain}) {
		t.Errorf("nudged %v, want venue %d once", w.nudged, plain)
	}

	e := w.editor(t, plain)
	if len(e.Categories) != 2 || e.Categories[0].Name != "Coffee" || e.Categories[1].Name != "Bakes" {
		t.Fatalf("categories: %+v", e.Categories)
	}
	bakes := e.Categories[1].ID
	var names []string
	for _, it := range e.Items {
		names = append(names, it.Name)
	}
	// Each category in its own order: what was there, then the file's rows.
	if got := strings.Join(names, "|"); got != "Espresso|Flat white|Banana bread|Lemon tart" {
		t.Errorf("items %s", got)
	}
	r := w.refs[plain]
	white, bread := e.Items[1], e.Items[2]
	if white.CategoryID != coffee || white.Price.Minor != 18000 || white.HSN != "2106" || white.Description != "Double shot" ||
		white.Diet != menu.DietVeg || !white.Popular || white.GlutenFree || !white.Active ||
		white.TaxClassID != r.taxClass || white.StationID != r.kitchen || white.CourseID != r.course {
		t.Errorf("flat white: %+v", white)
	}
	if bread.CategoryID != bakes || bread.Price.Minor != 12050 || bread.Diet != menu.DietVegan || !bread.GlutenFree || bread.StationID != r.kitchen || bread.CourseID != r.course {
		t.Errorf("banana bread: %+v", bread)
	}
	if e.Items[3].CategoryID != bakes {
		t.Errorf("lemon tart is in category %d, want the one Banana bread made (%d)", e.Items[3].CategoryID, bakes)
	}
}

func TestImportMenuRefusesAFileWithAProblem(t *testing.T) {
	w := newWorld(t)
	before := w.version(t, plain)
	bad := sheet + "Bakes,Brownie,a lot,1905,,,,,,\n"

	out, err := w.s.ImportMenu(context.Background(), plain, []byte(bad))
	if !errors.Is(err, ErrImportRefused) {
		t.Fatalf("err = %v, want ErrImportRefused", err)
	}
	if out.Imported || len(out.Problems) != 1 || out.Problems[0].Row != 5 || out.Problems[0].Column != menu.ColPrice {
		t.Errorf("refusal: %+v", out)
	}
	if e := w.editor(t, plain); len(e.Items) != 0 || len(e.Categories) != 0 {
		t.Errorf("a refused import wrote: %d items, %d categories", len(e.Items), len(e.Categories))
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: nothing changed", got, before, w.nudged)
	}
}

func TestImportMenuTwiceAddsOnce(t *testing.T) {
	w := newWorld(t)
	w.importMenu(t, plain, sheet)
	w.nudged = nil
	before := w.version(t, plain)

	again := w.importMenu(t, plain, sheet)
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 3 || len(again.NewCategories) != 0 {
		t.Errorf("second import: %+v", again)
	}
	if e := w.editor(t, plain); len(e.Items) != 3 || len(e.Categories) != 2 {
		t.Errorf("after two imports: %d items, %d categories", len(e.Items), len(e.Categories))
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: nothing was added", got, before, w.nudged)
	}
}

func TestImportMenuAddsOnlyWhatIsNew(t *testing.T) {
	w := newWorld(t)
	coffee := w.category(t, plain, "Coffee")
	w.create(t, plain, w.item(plain, coffee, "Flat white", 99900, ""))

	out := w.importMenu(t, plain, sheet)
	if !out.Imported || len(out.ToAdd) != 2 || len(out.Skipped) != 1 || out.Skipped[0].Name != "Flat white" {
		t.Fatalf("import: %+v", out)
	}
	for _, it := range w.editor(t, plain).Items {
		if it.Name == "Flat white" && it.Price.Minor != 99900 {
			t.Errorf("an import changed the price of an item already there: %d", it.Price.Minor)
		}
	}
}

func TestImportMenuReadsTheVenueItIsFor(t *testing.T) {
	tests := []struct {
		name  string
		venue int64
		row   string
		// The column of the one problem, or empty for none.
		column string
	}{
		{"no HSN where the venue's invoices do not show one", plain, "Coffee,Latte,180,,,,,,,", ""},
		{"no HSN on GST", gst, "Coffee,Latte,180,,,,,,,", menu.ColHSN},
		{"an HSN on GST", gst, "Coffee,Latte,180,2106,,,,,,", ""},
		{"a retired station", plain, "Coffee,Latte,180,,Bar,,,,,", menu.ColStation},
		{"a station by its code", plain, "Coffee,Latte,180,,KITCHEN,,,,,", ""},
		{"a course by its number", plain, "Coffee,Latte,180,,,1,,,,", ""},
		{"a course the venue does not have", plain, "Coffee,Latte,180,,,2,,,,", menu.ColCourse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			data := strings.SplitN(sheet, "\n", 2)[0] + "\n" + tt.row + "\n"
			out, err := w.s.PreviewImport(context.Background(), tt.venue, []byte(data))
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.column == "" && (len(out.Problems) != 0 || len(out.ToAdd) != 1):
				t.Errorf("want one row to add: %+v", out)
			case tt.column != "" && (len(out.Problems) != 1 || out.Problems[0].Column != tt.column):
				t.Errorf("want one problem on %s: %+v", tt.column, out.Problems)
			}
		})
	}
}

func TestImportMenuKeepsToItsVenue(t *testing.T) {
	w := newWorld(t)
	// Venue 2 has a category called Elsewhere; venue 1 does not.
	out := w.importMenu(t, plain, "Category,Name,Price\nElsewhere,Latte,180\n")
	if !slices.Equal(out.NewCategories, []string{"Elsewhere"}) {
		t.Errorf("new categories %v: another venue's category is not this venue's", out.NewCategories)
	}
	e := w.editor(t, plain)
	if len(e.Categories) != 1 || e.Categories[0].ID == w.elseCat || len(e.Items) != 1 || e.Items[0].CategoryID != e.Categories[0].ID {
		t.Errorf("venue 1: %+v %+v", e.Categories, e.Items)
	}
	if other := w.editor(t, gst); len(other.Items) != 0 || len(other.Categories) != 1 {
		t.Errorf("venue 2 was written to: %+v %+v", other.Categories, other.Items)
	}
}

func TestImportMenuRefusesAFileTooLarge(t *testing.T) {
	w := newWorld(t)
	big := []byte("Category,Name,Price\n" + strings.Repeat("x", MaxImportBytes))
	for name, run := range map[string]func() error{
		"preview": func() error { _, err := w.s.PreviewImport(context.Background(), plain, big); return err },
		"import":  func() error { _, err := w.s.ImportMenu(context.Background(), plain, big); return err },
	} {
		if err := run(); fieldOf(err) != "file" {
			t.Errorf("%s: err = %v, want Invalid on file", name, err)
		}
	}
	if len(w.nudged) != 0 {
		t.Errorf("nudged %v", w.nudged)
	}
}

func TestImportListsAreNeverMissing(t *testing.T) {
	w := newWorld(t)
	out, err := w.s.PreviewImport(context.Background(), plain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.ToAdd == nil || out.NewCategories == nil || out.Skipped == nil || len(out.Problems) != 1 || out.Problems[0].Row != 0 {
		t.Errorf("an empty file: %+v", out)
	}
}

// TestImportTemplateImportsAtItsOwnVenue: the template a venue downloads,
// given straight back, has nothing wrong with it and adds its two examples.
func TestImportTemplateImportsAtItsOwnVenue(t *testing.T) {
	w := newWorld(t)
	for _, tt := range []struct {
		name     string
		venue    int64
		category string
	}{
		{"an empty menu", plain, "Food"},
		// On GST, where every item needs an HSN, and with a category to name.
		{"a menu with a category", gst, "Elsewhere"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := w.s.ImportTemplate(context.Background(), tt.venue)
			if err != nil {
				t.Fatal(err)
			}
			var file strings.Builder
			cw := csv.NewWriter(&file)
			if err := cw.WriteAll(append([][]string{menu.ImportColumns}, unmarked(rows, 1)...)); err != nil {
				t.Fatal(err)
			}
			out, err := w.s.PreviewImport(context.Background(), tt.venue, []byte(file.String()))
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Problems) != 0 || len(out.ToAdd) != 2 || out.ToAdd[0].Category != tt.category {
				t.Errorf("the template, previewed: %+v", out)
			}
		})
	}
	if len(w.nudged) != 0 {
		t.Errorf("nudged %v: a template is not an edit", w.nudged)
	}
}

// unmarked is a template's rows as an owner makes them theirs: the cell at
// name without the example's mark, so the row is read and not left out.
func unmarked(rows [][]string, name int) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = slices.Clone(row)
		out[i][name] = strings.TrimSpace(strings.TrimPrefix(row[name], csvsheet.ExamplePrefix))
	}
	return out
}

// TestImportLeavesTheTemplatesExamplesOut: the template given straight back
// adds nothing and is nothing wrong, and says how many rows it left out.
func TestImportLeavesTheTemplatesExamplesOut(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	rows, err := w.s.ImportTemplate(ctx, plain)
	if err != nil {
		t.Fatal(err)
	}
	var file strings.Builder
	if err := csv.NewWriter(&file).WriteAll(append([][]string{menu.ImportColumns}, rows...)); err != nil {
		t.Fatal(err)
	}
	before := w.version(t, plain)
	out, err := w.s.ImportMenu(ctx, plain, []byte(file.String()))
	if err != nil {
		t.Fatal(err)
	}
	if out.Imported || out.Examples != 2 || len(out.ToAdd) != 0 || len(out.Problems) != 0 || len(out.NewCategories) != 0 {
		t.Errorf("the template, imported as it comes: %+v", out)
	}
	if e := w.editor(t, plain); len(e.Items) != 0 || len(e.Categories) != 0 || w.version(t, plain) != before {
		t.Errorf("an example reached the menu: %+v %+v", e.Categories, e.Items)
	}
}

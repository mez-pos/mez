package menus

import (
	"context"
	"encoding/csv"
	"slices"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/menu"
)

func (w *world) export(t *testing.T, venue int64) [][]string {
	t.Helper()
	rows, err := w.s.ExportMenu(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestExportMenuWritesEveryItemInMenuOrder(t *testing.T) {
	w := newWorld(t)
	coffee := w.category(t, plain, "Coffee")
	bakes := w.category(t, plain, "Bakes")
	// Made out of order, so the file's order is the menu's and not the ids'.
	tart := w.item(plain, bakes, "Lemon tart", 15000, "1905")
	tart.Description, tart.Diet, tart.GlutenFree, tart.Popular = "Sharp", menu.DietVeg, true, true
	w.create(t, plain, tart)
	w.create(t, plain, w.item(plain, coffee, "Flat white", 18000, ""))
	retired := w.create(t, plain, w.item(plain, coffee, "Mocha", 20050, ""))
	if err := w.s.SetItemActive(context.Background(), plain, retired, false); err != nil {
		t.Fatal(err)
	}
	w.nudged = nil

	rows := w.export(t, plain)
	want := [][]string{
		{"Coffee", "Flat white", "180.00", "Food 5%", "", "Kitchen", "Mains", "", "", "no", "no", "Active"},
		{"Coffee", "Mocha", "200.50", "Food 5%", "", "Kitchen", "Mains", "", "", "no", "no", "Retired"},
		{"Bakes", "Lemon tart", "150.00", "Food 5%", "1905", "Kitchen", "Mains", "Sharp", "veg", "yes", "yes", "Active"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %q", len(rows), len(want), rows)
	}
	for i := range want {
		if !slices.Equal(rows[i], want[i]) {
			t.Errorf("row %d\n got %q\nwant %q", i, rows[i], want[i])
		}
	}
	if len(w.nudged) != 0 {
		t.Errorf("nudged %v: an export is not an edit", w.nudged)
	}
}

// TestExportMenuGoesBackIn: a venue's export, given to its own import, has
// nothing wrong and nothing to add, a retired category's items included.
func TestExportMenuGoesBackIn(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	coffee := w.category(t, gst, "Coffee")
	old := w.category(t, gst, "Winter specials")
	w.create(t, gst, w.item(gst, coffee, "Flat white", 18000, "2106"))
	w.create(t, gst, w.item(gst, old, `Mulled apple, "hot"`, 22000, "2106"))
	if err := w.s.SetCategoryActive(ctx, gst, old, false); err != nil {
		t.Fatal(err)
	}
	before := w.version(t, gst)

	var file strings.Builder
	cw := csv.NewWriter(&file)
	if err := cw.WriteAll(append([][]string{menu.ExportColumns}, w.export(t, gst)...)); err != nil {
		t.Fatal(err)
	}
	out, err := w.s.ImportMenu(ctx, gst, []byte(file.String()))
	if err != nil {
		t.Fatalf("importing the export: %v (problems %+v)", err, out.Problems)
	}
	if out.Imported || len(out.Problems) != 0 || len(out.ToAdd) != 0 || len(out.Skipped) != 2 {
		t.Errorf("the export, imported: %+v", out)
	}
	if got := w.version(t, gst); got != before {
		t.Errorf("version %d (was %d): nothing was added", got, before)
	}
}

func TestExportMenuKeepsToItsVenue(t *testing.T) {
	w := newWorld(t)
	w.create(t, gst, w.item(gst, w.elseCat, "Theirs", 100, "2106"))
	if rows := w.export(t, plain); len(rows) != 0 {
		t.Errorf("the export of venue 1 has another venue's items: %q", rows)
	}
}

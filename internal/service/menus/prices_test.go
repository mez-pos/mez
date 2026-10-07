package menus

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// pricedWorld is the fixture with a small menu on venue 1: two coffees and
// a retired cake.
func pricedWorld(t *testing.T) (w *world, espresso, white, cake int64) {
	t.Helper()
	w = newWorld(t)
	coffee := w.category(t, plain, "Coffee")
	cakes := w.category(t, plain, "Cakes")
	espresso = w.create(t, plain, w.item(plain, coffee, "Espresso", 12000, ""))
	white = w.create(t, plain, w.item(plain, coffee, "Flat white", 18000, ""))
	cake = w.create(t, plain, w.item(plain, cakes, "Brownie", 9000, ""))
	if err := w.s.SetItemActive(context.Background(), plain, cake, false); err != nil {
		t.Fatal(err)
	}
	w.nudged = nil
	return w, espresso, white, cake
}

func (w *world) prices(t *testing.T, venue int64) map[int64]int64 {
	t.Helper()
	out := map[int64]int64{}
	for _, it := range w.editor(t, venue).Items {
		out[it.ID] = it.Price.Minor
	}
	return out
}

const newPrices = "Category,Name,Price\nCoffee,Espresso,130\nCoffee,Flat white,180\nCakes,Brownie,95.50\nCoffee,Cortado,170\n"

func TestPreviewPricesWritesNothing(t *testing.T) {
	w, espresso, _, cake := pricedWorld(t)
	before := w.version(t, plain)

	out, err := w.s.PreviewPrices(context.Background(), plain, []byte(newPrices))
	if err != nil {
		t.Fatal(err)
	}
	if out.Applied || out.Version != before || len(out.Changes) != 2 || out.Unchanged != 1 || len(out.Unknown) != 1 || len(out.Problems) != 0 {
		t.Fatalf("preview: %+v", out)
	}
	first := out.Changes[0]
	if first.ItemID != espresso || first.Row != 2 || first.Old.Minor != 12000 || first.New.Minor != 13000 || first.Old.Currency != "INR" || first.New.Currency != "INR" {
		t.Errorf("first change: %+v", first)
	}
	if out.Changes[1].ItemID != cake || out.Changes[1].New.Minor != 9550 {
		t.Errorf("second change: %+v", out.Changes[1])
	}
	if out.Unknown[0] != (PriceUnknown{Row: 5, Category: "Coffee", Name: "Cortado"}) {
		t.Errorf("unknown: %+v", out.Unknown)
	}
	if got := w.prices(t, plain); got[espresso] != 12000 || got[cake] != 9000 {
		t.Errorf("a preview changed prices: %v", got)
	}
	if got := w.version(t, plain); got != before || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: a preview is not an edit", got, before, w.nudged)
	}
}

func TestApplyPricesChangesThemAsOneEdit(t *testing.T) {
	w, espresso, white, cake := pricedWorld(t)
	before := w.version(t, plain)
	items := len(w.editor(t, plain).Items)

	out, err := w.s.ApplyPrices(context.Background(), plain, []byte(newPrices), before)
	if err != nil {
		t.Fatalf("applying: %v (problems %+v)", err, out.Problems)
	}
	if !out.Applied || len(out.Changes) != 2 {
		t.Fatalf("applied: %+v", out)
	}
	if got := w.prices(t, plain); got[espresso] != 13000 || got[white] != 18000 || got[cake] != 9550 {
		t.Errorf("prices: %v", got)
	}
	if got := w.version(t, plain); got != before+1 {
		t.Errorf("version %d, want %d: one file is one edit", got, before+1)
	}
	if !slices.Equal(w.nudged, []int64{plain}) {
		t.Errorf("nudged %v, want venue %d once", w.nudged, plain)
	}
	// The row that is no item added nothing, and the retired cake is still retired.
	e := w.editor(t, plain)
	if len(e.Items) != items {
		t.Errorf("%d items, was %d: a file of prices adds nothing", len(e.Items), items)
	}
	for _, it := range e.Items {
		if it.ID == cake && it.Active {
			t.Error("changing a retired item's price brought it back")
		}
	}
}

func TestApplyPricesChangesOnlyThePrice(t *testing.T) {
	w := newWorld(t)
	coffee := w.category(t, plain, "Coffee")
	in := w.item(plain, coffee, "Espresso", 12000, "2106")
	in.Description, in.Popular = "Short and strong", true
	id := w.create(t, plain, in)
	before := w.editor(t, plain).Items[0]

	// The file moves it, renames its tax class and clears everything else.
	file := "Category,Name,Price,Tax class,HSN,Station,Course,Description,Diet,Gluten free,Popular,Status\n" +
		"coffee,ESPRESSO,150,Nothing,9999,Nowhere,Never,,vegan,yes,no,Retired\n"
	if _, err := w.s.ApplyPrices(context.Background(), plain, []byte(file), w.version(t, plain)); err != nil {
		t.Fatal(err)
	}
	after := w.editor(t, plain).Items[0]
	want := before
	want.Price.Minor = 15000
	if after.ID != id || after.Name != want.Name || after.Price != want.Price || after.HSN != want.HSN || after.TaxClassID != want.TaxClassID ||
		after.StationID != want.StationID || after.CourseID != want.CourseID || after.Description != want.Description ||
		after.Diet != want.Diet || after.GlutenFree != want.GlutenFree || after.Popular != want.Popular || after.Active != want.Active ||
		after.CategoryID != want.CategoryID {
		t.Errorf("after\n got %+v\nwant %+v", after, want)
	}
}

func TestApplyPricesRefusals(t *testing.T) {
	w, espresso, _, _ := pricedWorld(t)
	ctx := context.Background()
	version := w.version(t, plain)

	bad := newPrices + "Coffee,Flat white,lots\n"
	out, err := w.s.ApplyPrices(ctx, plain, []byte(bad), version)
	if !errors.Is(err, ErrPricesRefused) || out.Applied || len(out.Problems) == 0 {
		t.Errorf("a file with a problem: %v %+v", err, out)
	}

	if _, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), version-1); !errors.Is(err, ErrPricesStale) {
		t.Errorf("a version behind: %v, want ErrPricesStale", err)
	}
	if _, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), version+1); !errors.Is(err, ErrPricesStale) {
		t.Errorf("a version ahead: %v, want ErrPricesStale", err)
	}

	big := []byte("Category,Name,Price\n" + strings.Repeat("x", MaxImportBytes))
	if _, err := w.s.ApplyPrices(ctx, plain, big, version); fieldOf(err) != "file" {
		t.Errorf("a file too large to apply: %v", err)
	}
	if _, err := w.s.PreviewPrices(ctx, plain, big); fieldOf(err) != "file" {
		t.Errorf("a file too large to check: %v", err)
	}

	if got := w.prices(t, plain); got[espresso] != 12000 {
		t.Errorf("a refusal changed a price: %v", got)
	}
	if got := w.version(t, plain); got != version || len(w.nudged) != 0 {
		t.Errorf("version %d (was %d), nudged %v: nothing changed", got, version, w.nudged)
	}
}

// TestApplyPricesIsRefusedOnceTheMenuMoves: the owner checks a file, someone
// edits the menu, and the file is not applied over what they did not see.
func TestApplyPricesIsRefusedOnceTheMenuMoves(t *testing.T) {
	w, espresso, white, _ := pricedWorld(t)
	ctx := context.Background()
	preview, err := w.s.PreviewPrices(ctx, plain, []byte(newPrices))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.s.SetItemActive(ctx, plain, white, false); err != nil {
		t.Fatal(err)
	}

	if _, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), preview.Version); !errors.Is(err, ErrPricesStale) {
		t.Fatalf("err = %v, want ErrPricesStale", err)
	}
	if got := w.prices(t, plain); got[espresso] != 12000 {
		t.Errorf("a stale file changed a price: %v", got)
	}

	// Checked again, it goes through.
	again, err := w.s.PreviewPrices(ctx, plain, []byte(newPrices))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), again.Version); err != nil || !out.Applied {
		t.Errorf("after checking again: %v %+v", err, out)
	}
}

func TestApplyPricesTwiceChangesOnce(t *testing.T) {
	w, espresso, _, _ := pricedWorld(t)
	ctx := context.Background()
	if _, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), w.version(t, plain)); err != nil {
		t.Fatal(err)
	}
	w.nudged = nil
	version := w.version(t, plain)

	again, err := w.s.ApplyPrices(ctx, plain, []byte(newPrices), version)
	if err != nil {
		t.Fatal(err)
	}
	if again.Applied || len(again.Changes) != 0 || again.Unchanged != 3 {
		t.Errorf("the same file again: %+v", again)
	}
	if got := w.version(t, plain); got != version || len(w.nudged) != 0 || w.prices(t, plain)[espresso] != 13000 {
		t.Errorf("version %d (was %d), nudged %v: nothing was left to change", got, version, w.nudged)
	}
}

func TestApplyPricesKeepsToItsVenue(t *testing.T) {
	w, _, _, _ := pricedWorld(t)
	ctx := context.Background()
	theirs := w.create(t, gst, w.item(gst, w.elseCat, "Espresso", 5000, "2106"))

	// Venue 2 has an Espresso too, in a category venue 1 does not have.
	out, err := w.s.ApplyPrices(ctx, plain, []byte("Category,Name,Price\nElsewhere,Espresso,1\nCoffee,Espresso,130\n"), w.version(t, plain))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Changes) != 1 || len(out.Unknown) != 1 {
		t.Errorf("applied: %+v", out)
	}
	if got := w.prices(t, gst)[theirs]; got != 5000 {
		t.Errorf("another venue's price is %d, was 5000", got)
	}
}

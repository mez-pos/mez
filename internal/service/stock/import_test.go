package stock

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

const ingredients = "Name,Counted in,Low at,Cost,Count at close\n" +
	"Milk,ml,2000,60,yes\n" +
	"Eggs,each,12,7.50,\n" +
	"Coffee beans,g,500,900,no\n"

func (f fixture) names(t *testing.T, venue int64) string {
	t.Helper()
	items, err := f.s.Items(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range items {
		out = append(out, it.Name)
	}
	return strings.Join(out, "|")
}

func TestPreviewItemsWritesNothing(t *testing.T) {
	f := newFixture(t)
	f.item(t, 1, "Coffee beans", stock.Gram, 0)

	out, err := f.s.PreviewItems(context.Background(), 1, []byte(ingredients))
	if err != nil {
		t.Fatal(err)
	}
	if out.Imported || len(out.ToAdd) != 2 || len(out.Skipped) != 1 || len(out.Problems) != 0 {
		t.Fatalf("preview: %+v", out)
	}
	want := ItemImportRow{Row: 2, Name: "Milk", Unit: stock.Millilitre, LowLevel: 2000 * stock.PerUnit, Cost: money.Amount{Minor: 6000, Currency: "INR"}, CountAtClose: true}
	if out.ToAdd[0] != want {
		t.Errorf("first row\n got %+v\nwant %+v", out.ToAdd[0], want)
	}
	if out.Skipped[0] != (ImportSkipped{Row: 4, Name: "Coffee beans"}) {
		t.Errorf("skipped: %+v", out.Skipped)
	}
	if got := f.names(t, 1); got != "Coffee beans" {
		t.Errorf("a preview wrote: %s", got)
	}
}

func TestImportItemsAddsThemWithNothingOnHand(t *testing.T) {
	f := newFixture(t)
	beans := f.item(t, 1, "Coffee beans", stock.Gram, 0)

	out, err := f.s.ImportItems(context.Background(), 1, []byte(ingredients))
	if err != nil {
		t.Fatalf("importing: %v (problems %+v)", err, out.Problems)
	}
	if !out.Imported || len(out.ToAdd) != 2 {
		t.Fatalf("import: %+v", out)
	}
	items, err := f.s.Items(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items: %+v", items)
	}
	for _, it := range items {
		switch it.Name {
		case "Milk":
			if it.Unit != stock.Millilitre || it.LowLevel != 2000*stock.PerUnit || it.Cost.Minor != 6000 || !it.CountAtClose || !it.Active || it.OnHand != 0 {
				t.Errorf("milk: %+v", it)
			}
		case "Eggs":
			if it.Unit != stock.Each || it.LowLevel != 12*stock.PerUnit || it.Cost.Minor != 750 || it.CountAtClose || it.OnHand != 0 {
				t.Errorf("eggs: %+v", it)
			}
		case "Coffee beans":
			// The row for it would have set a level and a cost: it changed nothing.
			if it.ID != beans || it.LowLevel != 0 || it.Cost.Minor != 0 {
				t.Errorf("an import changed an item already there: %+v", it)
			}
		}
	}
}

func TestImportItemsRefusesAFileWithAProblem(t *testing.T) {
	f := newFixture(t)
	out, err := f.s.ImportItems(context.Background(), 1, []byte(ingredients+"Rice,kg,5,90,\n"))
	if !errors.Is(err, ErrImportRefused) {
		t.Fatalf("err = %v, want ErrImportRefused", err)
	}
	if out.Imported || len(out.Problems) != 1 || out.Problems[0].Row != 5 || out.Problems[0].Column != stock.ColUnit {
		t.Errorf("refusal: %+v", out)
	}
	if got := f.names(t, 1); got != "" {
		t.Errorf("a refused import wrote: %s", got)
	}
}

func TestImportItemsTwiceAddsOnce(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.s.ImportItems(ctx, 1, []byte(ingredients)); err != nil {
		t.Fatal(err)
	}
	again, err := f.s.ImportItems(ctx, 1, []byte(ingredients))
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 3 {
		t.Errorf("the same file again: %+v", again)
	}
	if got := f.names(t, 1); strings.Count(got, "|") != 2 {
		t.Errorf("after two imports: %s", got)
	}
}

func TestImportItemsLeavesARetiredItemRetired(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	milk := f.item(t, 1, "Milk", stock.Millilitre, 0)
	if err := f.s.SetItemActive(ctx, 1, milk, false); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.ImportItems(ctx, 1, []byte(ingredients))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Skipped) != 1 || out.Skipped[0].Name != "Milk" {
		t.Errorf("skipped: %+v", out.Skipped)
	}
	items, _ := f.s.Items(ctx, 1)
	for _, it := range items {
		if it.ID == milk && it.Active {
			t.Error("an import brought a retired item back")
		}
	}
}

func TestImportItemsKeepsToItsVenue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.item(t, 2, "Milk", stock.Millilitre, 0)

	// Venue 2 has Milk; venue 1 does not, so it is added there and only there.
	out, err := f.s.ImportItems(ctx, 1, []byte(ingredients))
	if err != nil || len(out.ToAdd) != 3 {
		t.Fatalf("import: %v %+v", err, out)
	}
	if got := f.names(t, 2); got != "Milk" {
		t.Errorf("venue 2 was written to: %s", got)
	}
}

func TestImportItemsRefusesAFileTooLarge(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	big := []byte("Name,Counted in\n" + strings.Repeat("x", MaxImportBytes))
	var invalid *Invalid
	if _, err := f.s.PreviewItems(ctx, 1, big); !errors.As(err, &invalid) || invalid.Field != "file" {
		t.Errorf("preview: %v", err)
	}
	if _, err := f.s.ImportItems(ctx, 1, big); !errors.As(err, &invalid) || invalid.Field != "file" {
		t.Errorf("import: %v", err)
	}
}

// TestItemsTemplateImportsAtItsOwnVenue: the template a venue downloads,
// given straight back, has nothing wrong and adds its three examples.
func TestItemsTemplateImportsAtItsOwnVenue(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	rows, err := f.s.ItemsTemplate(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	var file strings.Builder
	if err := csv.NewWriter(&file).WriteAll(append([][]string{stock.ItemColumns}, rows...)); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.PreviewItems(ctx, 1, []byte(file.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Problems) != 0 || len(out.ToAdd) != 3 {
		t.Errorf("the template, previewed: %+v", out)
	}
}

func TestItemsImportListsAreNeverMissing(t *testing.T) {
	f := newFixture(t)
	out, err := f.s.PreviewItems(context.Background(), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.ToAdd == nil || out.Skipped == nil || len(out.Problems) != 1 || out.Problems[0].Row != 0 {
		t.Errorf("an empty file: %+v", out)
	}
}

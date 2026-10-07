package stock

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"

	"github.com/byjoellobo/mez/internal/db"
	"github.com/byjoellobo/mez/internal/domain/stock"
	"github.com/byjoellobo/mez/internal/domain/tax"
)

// cafeMenu gives a venue a menu to write recipes for: Bakes first with a
// Toast, then Coffee with an Espresso and a Latte, and a Toast in Kids too.
// It answers the items' ids by name; the two Toasts are "Toast" and
// "Toast (Kids)".
func cafeMenu(t *testing.T, f fixture, venue int64) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	item := func(name string) db.ItemSpec {
		return db.ItemSpec{Name: name, Price: 10000, TaxClass: "Food", Station: "bar", Course: 1}
	}
	if _, err := f.d.CreateMenu(ctx, venue, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food", Rate: tax.RateFromBP(500)}},
		Categories: []db.CategorySpec{
			{Name: "Bakes", Items: []db.ItemSpec{item("Toast")}},
			{Name: "Coffee", Items: []db.ItemSpec{item("Espresso"), item("Latte")}},
			{Name: "Kids", Items: []db.ItemSpec{item("Toast")}},
		},
	}, now); err != nil {
		t.Fatal(err)
	}
	m, err := f.d.Menu(ctx, venue)
	if err != nil {
		t.Fatal(err)
	}
	category := map[int64]string{}
	for _, c := range m.Categories {
		category[c.ID] = c.Name
	}
	ids := map[string]int64{}
	for _, it := range m.Items {
		name := it.Name
		if category[it.CategoryID] == "Kids" {
			name += " (Kids)"
		}
		ids[name] = it.ID
	}
	return ids
}

func (f fixture) recipeOf(t *testing.T, venue, item int64) []RecipePart {
	t.Helper()
	book, err := f.s.Recipes(context.Background(), venue)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range book.Items {
		if r.ID == item {
			return r.Parts
		}
	}
	return nil
}

const recipeSheet = "Dish,Ingredient,Amount\nEspresso,Beans,18\nLatte,Beans,18\nLatte,Milk,200.5\n"

func TestPreviewRecipesWritesNothing(t *testing.T) {
	f := newFixture(t)
	ids := cafeMenu(t, f, 1)
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	f.item(t, 1, "Milk", stock.Millilitre, 0)

	out, err := f.s.PreviewRecipes(context.Background(), 1, []byte(recipeSheet))
	if err != nil {
		t.Fatal(err)
	}
	if out.Imported || len(out.ToAdd) != 2 || len(out.Problems) != 0 || len(out.Skipped) != 0 {
		t.Fatalf("preview: %+v", out)
	}
	espresso := out.ToAdd[0]
	if espresso.ItemID != ids["Espresso"] || espresso.Dish != "Espresso" || len(espresso.Parts) != 1 ||
		espresso.Parts[0] != (RecipeImportPart{Row: 2, StockItemID: beans, Ingredient: "Beans", Unit: stock.Gram, Qty: 18 * stock.PerUnit}) {
		t.Errorf("espresso: %+v", espresso)
	}
	if latte := out.ToAdd[1]; len(latte.Parts) != 2 || latte.Parts[1].Qty != 200500 || latte.Parts[1].Unit != stock.Millilitre {
		t.Errorf("latte: %+v", latte)
	}
	if got := f.recipeOf(t, 1, ids["Espresso"]); len(got) != 0 {
		t.Errorf("a preview wrote: %+v", got)
	}
}

func TestImportRecipesGivesEachDishItsRecipe(t *testing.T) {
	f := newFixture(t)
	ids := cafeMenu(t, f, 1)
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	milk := f.item(t, 1, "Milk", stock.Millilitre, 0)

	out, err := f.s.ImportRecipes(context.Background(), 1, []byte(recipeSheet))
	if err != nil {
		t.Fatalf("importing: %v (problems %+v)", err, out.Problems)
	}
	if !out.Imported || len(out.ToAdd) != 2 {
		t.Fatalf("import: %+v", out)
	}
	if got := f.recipeOf(t, 1, ids["Espresso"]); len(got) != 1 || got[0] != (RecipePart{beans, 18 * stock.PerUnit}) {
		t.Errorf("espresso: %+v", got)
	}
	latte := f.recipeOf(t, 1, ids["Latte"])
	if len(latte) != 2 {
		t.Fatalf("latte: %+v", latte)
	}
	for _, p := range latte {
		if (p.StockItemID == beans && p.Qty != 18*stock.PerUnit) || (p.StockItemID == milk && p.Qty != 200500) {
			t.Errorf("latte part: %+v", p)
		}
	}
}

func TestImportRecipesLeavesARecipeThatIsThereAlone(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ids := cafeMenu(t, f, 1)
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	f.item(t, 1, "Milk", stock.Millilitre, 0)
	if err := f.s.SetItemRecipe(ctx, 1, ids["Latte"], []RecipePart{{beans, 9 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}

	out, err := f.s.ImportRecipes(ctx, 1, []byte(recipeSheet))
	if err != nil {
		t.Fatal(err)
	}
	if !out.Imported || len(out.ToAdd) != 1 || out.ToAdd[0].Dish != "Espresso" || len(out.Skipped) != 2 {
		t.Fatalf("import: %+v", out)
	}
	if got := f.recipeOf(t, 1, ids["Latte"]); len(got) != 1 || got[0].Qty != 9*stock.PerUnit {
		t.Errorf("an import changed a recipe that was there: %+v", got)
	}

	// The same file again: every dish in it has a recipe now.
	again, err := f.s.ImportRecipes(ctx, 1, []byte(recipeSheet))
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported || len(again.ToAdd) != 0 || len(again.Skipped) != 3 {
		t.Errorf("the same file again: %+v", again)
	}
}

func TestImportRecipesRefusesAFileWithAProblem(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ids := cafeMenu(t, f, 1)
	f.item(t, 1, "Beans", stock.Gram, 0)
	// No Milk in the store: the latte's second row is wrong.
	out, err := f.s.ImportRecipes(ctx, 1, []byte(recipeSheet))
	if !errors.Is(err, ErrImportRefused) {
		t.Fatalf("err = %v, want ErrImportRefused", err)
	}
	if out.Imported || len(out.Problems) != 1 || out.Problems[0].Row != 4 || out.Problems[0].Column != stock.ColIngredient {
		t.Errorf("refusal: %+v", out)
	}
	// The espresso's recipe was right, and is not written either.
	if got := f.recipeOf(t, 1, ids["Espresso"]); len(got) != 0 {
		t.Errorf("a refused import wrote: %+v", got)
	}
}

func TestImportRecipesReadsTheMenuAndTheStoreItIsFor(t *testing.T) {
	tests := []struct {
		name, row string
		// The column of the one problem, or empty for none.
		column string
	}{
		{"a dish on the menu", "Espresso,Beans,18", ""},
		{"two dishes of one name", "Toast,Beans,5", stock.ColDish},
		{"a retired dish is not on sale", "Mocha,Beans,18", stock.ColDish},
		{"a dish in a retired category is not on sale", "Brownie,Beans,5", stock.ColDish},
		{"a retired ingredient", "Espresso,Old syrup,5", stock.ColIngredient},
		{"another venue's ingredient", "Espresso,Theirs,5", stock.ColIngredient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			cafeMenu(t, f, 1)
			// A retired dish, and a category retired with the dish in it.
			for _, sql := range []string{
				`INSERT INTO categories (venue_id, name, sort_order, active, created_at, updated_at) SELECT 1, 'Old', 9, 0, created_at, updated_at FROM categories LIMIT 1`,
				`INSERT INTO items (venue_id, category_id, name, price, hsn, tax_class_id, station_id, course_id, sort_order, active, created_at, updated_at)
				 SELECT venue_id, category_id, 'Mocha', price, hsn, tax_class_id, station_id, course_id, 9, 0, created_at, updated_at FROM items WHERE name = 'Espresso'`,
				`INSERT INTO items (venue_id, category_id, name, price, hsn, tax_class_id, station_id, course_id, sort_order, active, created_at, updated_at)
				 SELECT venue_id, (SELECT id FROM categories WHERE name = 'Old'), 'Brownie', price, hsn, tax_class_id, station_id, course_id, 0, 1, created_at, updated_at FROM items WHERE name = 'Espresso'`,
			} {
				if _, err := f.d.Writer().ExecContext(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			f.item(t, 1, "Beans", stock.Gram, 0)
			syrup := f.item(t, 1, "Old syrup", stock.Millilitre, 0)
			if err := f.s.SetItemActive(ctx, 1, syrup, false); err != nil {
				t.Fatal(err)
			}
			f.item(t, 2, "Theirs", stock.Gram, 0)

			out, err := f.s.PreviewRecipes(ctx, 1, []byte("Dish,Ingredient,Amount\n"+tt.row+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.column == "" && (len(out.Problems) != 0 || len(out.ToAdd) != 1):
				t.Errorf("want one recipe: %+v", out)
			case tt.column != "" && (len(out.Problems) != 1 || out.Problems[0].Column != tt.column):
				t.Errorf("want one problem on %s: %+v", tt.column, out.Problems)
			}
		})
	}
}

// TestRecipesTemplateIsTheDishesWithNoRecipe: in menu order, without the two
// that share a name, and it reads back as nothing wrong and nothing to do.
func TestRecipesTemplateIsTheDishesWithNoRecipe(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ids := cafeMenu(t, f, 1)
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	if err := f.s.SetItemRecipe(ctx, 1, ids["Latte"], []RecipePart{{beans, 18 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}

	rows, err := f.s.RecipesTemplate(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "Espresso" {
		t.Fatalf("rows: %q", rows)
	}
	var file strings.Builder
	if err := csv.NewWriter(&file).WriteAll(append([][]string{stock.RecipeColumns}, rows...)); err != nil {
		t.Fatal(err)
	}
	out, err := f.s.PreviewRecipes(ctx, 1, []byte(file.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Problems) != 0 || out.Blank != 1 || len(out.ToAdd) != 0 {
		t.Errorf("the template as it comes: %+v", out)
	}
}

func TestImportRecipesRefusesAFileTooLarge(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	big := []byte("Dish,Ingredient,Amount\n" + strings.Repeat("x", MaxImportBytes))
	var invalid *Invalid
	if _, err := f.s.PreviewRecipes(ctx, 1, big); !errors.As(err, &invalid) || invalid.Field != "file" {
		t.Errorf("preview: %v", err)
	}
	if _, err := f.s.ImportRecipes(ctx, 1, big); !errors.As(err, &invalid) || invalid.Field != "file" {
		t.Errorf("import: %v", err)
	}
}

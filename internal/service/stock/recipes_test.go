package stock

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db"
	"github.com/mez-pos/mez/internal/domain/stock"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// menuOf gives a venue one item, Latte, with a Milk group of Dairy: its
// ids, as the menu read names them.
func menuOf(t *testing.T, f fixture, venue int64) (item, modifier int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.d.CreateMenu(ctx, venue, db.MenuSpec{
		Stations:   []db.StationSpec{{Code: "bar", Name: "Bar"}},
		Courses:    []db.CourseSpec{{Number: 1, Name: "Mains"}},
		TaxClasses: []db.TaxClassSpec{{Name: "Food", Rate: tax.RateFromBP(500)}},
		Groups:     []db.ModifierGroupSpec{{Name: "Milk", MaxSelect: 1, Modifiers: []db.ModifierSpec{{Name: "Dairy"}}}},
		Categories: []db.CategorySpec{{Name: "Coffee", Items: []db.ItemSpec{
			{Name: "Latte", Price: 18000, TaxClass: "Food", Station: "bar", Course: 1, Groups: []string{"Milk"}},
		}}},
	}, now); err != nil {
		t.Fatal(err)
	}
	m, err := f.d.Menu(ctx, venue)
	if err != nil {
		t.Fatal(err)
	}
	return m.Items[0].ID, m.Groups[0].Modifiers[0].ID
}

func TestRecipes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	latte, dairy := menuOf(t, f, 1)
	otherLatte, otherDairy := menuOf(t, f, 2)
	beans := f.item(t, 1, "Beans", stock.Gram, 0)
	milk := f.item(t, 1, "Milk", stock.Millilitre, 0)
	elsewhere := f.item(t, 2, "Beans", stock.Gram, 0)

	if err := f.s.SetItemRecipe(ctx, 1, latte, []RecipePart{{beans, 18 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}
	if err := f.s.SetModifierRecipe(ctx, 1, dairy, []RecipePart{{milk, 200 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}
	book, err := f.s.Recipes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(book.Items) != 1 || book.Items[0].ID != latte || book.Items[0].Parts[0] != (RecipePart{beans, 18000}) {
		t.Errorf("items = %+v", book.Items)
	}
	if len(book.Modifiers) != 1 || book.Modifiers[0].ID != dairy || book.Modifiers[0].Parts[0].Qty != 200000 {
		t.Errorf("modifiers = %+v", book.Modifiers)
	}

	// Replacing is whole: the new list is the recipe.
	if err := f.s.SetItemRecipe(ctx, 1, latte, []RecipePart{{beans, 20 * stock.PerUnit}, {milk, 50 * stock.PerUnit}}); err != nil {
		t.Fatal(err)
	}
	if book, _ := f.s.Recipes(ctx, 1); len(book.Items[0].Parts) != 2 || book.Items[0].Parts[0].Qty != 20000 {
		t.Errorf("replaced = %+v", book.Items)
	}
	if err := f.s.SetModifierRecipe(ctx, 1, dairy, nil); err != nil {
		t.Fatal(err)
	}
	if book, _ := f.s.Recipes(ctx, 1); len(book.Modifiers) != 0 {
		t.Errorf("an empty recipe clears it: %+v", book.Modifiers)
	}

	if err := f.s.SetItemActive(ctx, 1, milk, false); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		err   error
		field string
		is    error
	}{
		{"another venue's stock item", f.s.SetItemRecipe(ctx, 1, latte, []RecipePart{{elsewhere, 1}}), "parts.0.stock_item_id", nil},
		{"a retired stock item", f.s.SetItemRecipe(ctx, 1, latte, []RecipePart{{beans, 1}, {milk, 1}}), "parts.1.stock_item_id", nil},
		{"one stock item twice", f.s.SetItemRecipe(ctx, 1, latte, []RecipePart{{beans, 1}, {beans, 2}}), "parts.1.stock_item_id", nil},
		{"none of it", f.s.SetModifierRecipe(ctx, 1, dairy, []RecipePart{{beans, 0}}), "parts.0.qty", nil},
		{"another venue's menu item", f.s.SetItemRecipe(ctx, 1, otherLatte, []RecipePart{{beans, 1}}), "", ErrMenuItemNotFound},
		{"another venue's modifier", f.s.SetModifierRecipe(ctx, 1, otherDairy, []RecipePart{{beans, 1}}), "", ErrModifierNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.is != nil {
				if !errors.Is(tt.err, tt.is) {
					t.Errorf("err = %v, want %v", tt.err, tt.is)
				}
				return
			}
			if fieldOf(tt.err) != tt.field {
				t.Errorf("err = %v, want a refusal on %q", tt.err, tt.field)
			}
		})
	}
	if book, _ := f.s.Recipes(ctx, 1); len(book.Items[0].Parts) != 2 {
		t.Errorf("refused recipes must change nothing: %+v", book.Items)
	}
}

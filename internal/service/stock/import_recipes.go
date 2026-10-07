package stock

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/byjoellobo/mez/internal/db/gen"
	"github.com/byjoellobo/mez/internal/domain/stock"
)

// Recipes from a spreadsheet (phase 10, Task 10c, ADR-300).
// stock.ReadRecipes reads the file; this reads the menu and the store for
// it to be read against, and writes the recipes it answers. The shape is
// the ingredients import's: a preview that writes nothing, and an import
// that reads the file again inside the write.

// RecipesImport is the recipes a file would give, or has given.
type RecipesImport struct {
	ToAdd []RecipeImportDish `json:"to_add"`
	// Skipped are the rows left out because their dish already has a recipe.
	Skipped []RecipeImportSkipped `json:"skipped"`
	// Blank is how many rows name a dish and nothing it takes.
	Blank    int             `json:"blank"`
	Problems []ImportProblem `json:"problems"`
	// Imported: the dishes in ToAdd have these recipes now. False for a
	// preview, a refusal, and a file that gave none.
	Imported bool `json:"imported"`
}

// RecipeImportDish is a dish and the recipe a file gives it.
type RecipeImportDish struct {
	ItemID int64              `json:"item_id"`
	Dish   string             `json:"dish"`
	Parts  []RecipeImportPart `json:"parts"`
}

// RecipeImportPart is one ingredient of a recipe a file gives.
type RecipeImportPart struct {
	// Row is its row in the spreadsheet; the header is row 1.
	Row         int        `json:"row"`
	StockItemID int64      `json:"stock_item_id"`
	Ingredient  string     `json:"ingredient"`
	Unit        stock.Unit `json:"unit"`
	Qty         stock.Qty  `json:"qty"`
}

// RecipeImportSkipped is a row whose dish already has a recipe.
type RecipeImportSkipped struct {
	Row  int    `json:"row"`
	Dish string `json:"dish"`
}

// PreviewRecipes reads a file of recipes and answers what importing it
// would do. It writes nothing.
func (s *Service) PreviewRecipes(ctx context.Context, venueID int64, data []byte) (RecipesImport, error) {
	if err := checkFileSize(data); err != nil {
		return RecipesImport{}, err
	}
	v, err := recipesVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return RecipesImport{}, err
	}
	return recipesImportOf(stock.ReadRecipes(data, v)), nil
}

// ImportRecipes gives each dish in a file its recipe, in one transaction:
// all of them or none. A dish that already has a recipe is left as it is. A
// file with any problem is refused with ErrImportRefused and the problems.
// It changes what later closes take from stock, not what earlier sales
// took. Sending the same file twice gives nothing the second time.
func (s *Service) ImportRecipes(ctx context.Context, venueID int64, data []byte) (RecipesImport, error) {
	if err := checkFileSize(data); err != nil {
		return RecipesImport{}, err
	}
	var out RecipesImport
	err := s.db.WriteTx(ctx, func(q *gen.Queries) error {
		v, err := recipesVenue(ctx, q, venueID)
		if err != nil {
			return err
		}
		plan := stock.ReadRecipes(data, v)
		out = recipesImportOf(plan)
		switch {
		case len(plan.Problems) > 0:
			return ErrImportRefused
		case len(plan.Recipes) == 0:
			return errNothingToImport
		}
		for _, recipe := range plan.Recipes {
			for _, p := range recipe.Parts {
				if err := q.AddItemRecipePart(ctx, gen.AddItemRecipePartParams{
					VenueID: venueID, ItemID: recipe.ItemID, StockItemID: p.StockItemID, Qty: int64(p.Qty),
				}); err != nil {
					return fmt.Errorf("stock: importing row %d (the recipe of %s): %w", p.Row, recipe.Dish, err)
				}
			}
		}
		return nil
	})
	switch {
	case errors.Is(err, errNothingToImport):
		return out, nil
	case errors.Is(err, ErrImportRefused):
		return out, err
	case err != nil:
		return RecipesImport{}, err
	}
	out.Imported = true
	return out, nil
}

// RecipesTemplate is the rows of the file a venue fills in, under
// stock.RecipeColumns: a row for every dish on sale that has no recipe.
func (s *Service) RecipesTemplate(ctx context.Context, venueID int64) ([][]string, error) {
	v, err := recipesVenue(ctx, s.db.Queries(), venueID)
	if err != nil {
		return nil, err
	}
	return stock.RecipesTemplate(v), nil
}

// recipesVenue is the menu and the store as a file of recipes is read
// against them: the dishes on sale, in menu order, and every stock item.
// A dish is on sale when it and its category are in use, which is what the
// Recipes screen lists.
func recipesVenue(ctx context.Context, q *gen.Queries, venueID int64) (stock.RecipesVenue, error) {
	categories, err := q.EditorCategories(ctx, venueID)
	if err != nil {
		return stock.RecipesVenue{}, fmt.Errorf("stock: reading the menu's categories: %w", err)
	}
	type category struct {
		name   string
		place  int
		active bool
	}
	byID := make(map[int64]category, len(categories))
	for i, c := range categories {
		byID[c.ID] = category{name: c.Name, place: i, active: c.Active}
	}
	items, err := q.EditorItems(ctx, venueID)
	if err != nil {
		return stock.RecipesVenue{}, fmt.Errorf("stock: reading the menu's items: %w", err)
	}
	recipes, err := readRecipes(ctx, q, venueID)
	if err != nil {
		return stock.RecipesVenue{}, err
	}
	// Stable: within a category the items' own order is kept.
	slices.SortStableFunc(items, func(a, b gen.EditorItemsRow) int { return byID[a.CategoryID].place - byID[b.CategoryID].place })

	var v stock.RecipesVenue
	for _, it := range items {
		c := byID[it.CategoryID]
		if it.Active && c.active {
			v.Dishes = append(v.Dishes, stock.RecipeDish{ID: it.ID, Name: it.Name, Category: c.name, HasRecipe: len(recipes.Items[it.ID]) > 0})
		}
	}
	rows, err := q.StockItemsWithOnHand(ctx, venueID)
	if err != nil {
		return stock.RecipesVenue{}, fmt.Errorf("stock: reading the stock of venue %d: %w", venueID, err)
	}
	for _, r := range rows {
		v.Ingredients = append(v.Ingredients, stock.RecipeIngredient{ID: r.ID, Name: r.Name, Unit: stock.Unit(r.Unit), Active: r.Active})
	}
	return v, nil
}

// recipesImportOf is a plan as the back office reads it. Every list is
// there, if empty.
func recipesImportOf(plan stock.RecipesPlan) RecipesImport {
	out := RecipesImport{
		ToAdd:    make([]RecipeImportDish, len(plan.Recipes)),
		Skipped:  make([]RecipeImportSkipped, len(plan.Skipped)),
		Blank:    plan.Blank,
		Problems: problemsOf(plan.Problems),
	}
	for i, r := range plan.Recipes {
		dish := RecipeImportDish{ItemID: r.ItemID, Dish: r.Dish, Parts: make([]RecipeImportPart, len(r.Parts))}
		for j, p := range r.Parts {
			dish.Parts[j] = RecipeImportPart(p)
		}
		out.ToAdd[i] = dish
	}
	for i, sk := range plan.Skipped {
		out.Skipped[i] = RecipeImportSkipped(sk)
	}
	return out
}

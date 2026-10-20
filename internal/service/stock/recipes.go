package stock

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/stock"
)

// The recipe refusals.
var (
	ErrMenuItemNotFound = errors.New("no such menu item at this venue")
	ErrModifierNotFound = errors.New("no such modifier at this venue")
)

// RecipePart is one stock item a recipe uses, per unit sold.
type RecipePart struct {
	StockItemID int64     `json:"stock_item_id"`
	Qty         stock.Qty `json:"qty"`
}

// Recipe is what one menu item, or one modifier, takes from stock.
type Recipe struct {
	ID    int64        `json:"id"`
	Parts []RecipePart `json:"parts"`
}

// RecipeBook is every recipe a venue has.
type RecipeBook struct {
	Items     []Recipe `json:"items"`
	Modifiers []Recipe `json:"modifiers"`
}

// Recipes reads every recipe at the venue.
func (s *Service) Recipes(ctx context.Context, venueID int64) (RecipeBook, error) {
	r, err := readRecipes(ctx, s.db.Queries(), venueID)
	if err != nil {
		return RecipeBook{}, err
	}
	return RecipeBook{Items: book(r.Items), Modifiers: book(r.Modifiers)}, nil
}

// SetItemRecipe replaces a menu item's recipe; an empty one clears it. It
// changes what later closes take, not what earlier sales took.
func (s *Service) SetItemRecipe(ctx context.Context, venueID, itemID int64, parts []RecipePart) error {
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.MenuItemForRecipe(ctx, gen.MenuItemForRecipeParams{VenueID: venueID, ID: itemID}); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrMenuItemNotFound, itemID)
		} else if err != nil {
			return fmt.Errorf("stock: reading menu item %d: %w", itemID, err)
		}
		if err := checkParts(ctx, q, venueID, parts); err != nil {
			return err
		}
		if err := q.ClearItemRecipe(ctx, gen.ClearItemRecipeParams{VenueID: venueID, ItemID: itemID}); err != nil {
			return fmt.Errorf("stock: clearing the recipe of item %d: %w", itemID, err)
		}
		for _, p := range parts {
			if err := q.AddItemRecipePart(ctx, gen.AddItemRecipePartParams{
				VenueID: venueID, ItemID: itemID, StockItemID: p.StockItemID, Qty: int64(p.Qty),
			}); err != nil {
				return fmt.Errorf("stock: writing the recipe of item %d: %w", itemID, err)
			}
		}
		return nil
	})
}

// SetModifierRecipe replaces a modifier's recipe; an empty one clears it.
func (s *Service) SetModifierRecipe(ctx context.Context, venueID, modifierID int64, parts []RecipePart) error {
	return s.db.WriteTx(ctx, func(q *gen.Queries) error {
		if _, err := q.ModifierForRecipe(ctx, gen.ModifierForRecipeParams{VenueID: venueID, ID: modifierID}); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %d", ErrModifierNotFound, modifierID)
		} else if err != nil {
			return fmt.Errorf("stock: reading modifier %d: %w", modifierID, err)
		}
		if err := checkParts(ctx, q, venueID, parts); err != nil {
			return err
		}
		if err := q.ClearModifierRecipe(ctx, gen.ClearModifierRecipeParams{VenueID: venueID, ModifierID: modifierID}); err != nil {
			return fmt.Errorf("stock: clearing the recipe of modifier %d: %w", modifierID, err)
		}
		for _, p := range parts {
			if err := q.AddModifierRecipePart(ctx, gen.AddModifierRecipePartParams{
				VenueID: venueID, ModifierID: modifierID, StockItemID: p.StockItemID, Qty: int64(p.Qty),
			}); err != nil {
				return fmt.Errorf("stock: writing the recipe of modifier %d: %w", modifierID, err)
			}
		}
		return nil
	})
}

// checkParts checks a recipe's rules, and that every stock item it names
// is this venue's and in use.
func checkParts(ctx context.Context, q *gen.Queries, venueID int64, parts []RecipePart) error {
	domain := make([]stock.Part, len(parts))
	for i, p := range parts {
		domain[i] = stock.Part{StockItemID: p.StockItemID, Qty: p.Qty}
	}
	if p := stock.RecipeProblem(domain); p != nil {
		return &Invalid{p.Field, p.Message}
	}
	for i, p := range parts {
		it, err := q.StockItemForEdit(ctx, gen.StockItemForEditParams{VenueID: venueID, ID: p.StockItemID})
		field := fmt.Sprintf("parts.%d.stock_item_id", i)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return &Invalid{field, "Choose a stock item."}
		case err != nil:
			return fmt.Errorf("stock: reading stock item %d: %w", p.StockItemID, err)
		case !it.Active:
			return &Invalid{field, "That stock item is retired."}
		}
	}
	return nil
}

// Sale is an order being closed, as DeductSale needs it.
type Sale struct {
	VenueID int64
	OrderID int64
	// StaffID is who closed it: the sold movements are theirs.
	StaffID int64
	// Lines are the lines that used stock: every one not voided.
	Lines []stock.SoldLine
	At    time.Time
}

// DeductSale writes the sold movements that make an order's deductions
// equal its usage, inside q's transaction: the order service calls it as
// the order closes, so the stock goes with the close or not at all
// (ADR-112). It reads the recipes as they are now. Closing again with the
// same lines writes nothing; a line voided after a reopen is put back. Each
// movement keeps the dish its stock went into (ADR-189).
func DeductSale(ctx context.Context, q *gen.Queries, sale Sale) error {
	recipes, err := readRecipes(ctx, q, sale.VenueID)
	if err != nil {
		return err
	}
	rows, err := q.SoldForOrder(ctx, gen.SoldForOrderParams{VenueID: sale.VenueID, OrderID: sale.OrderID})
	if err != nil {
		return fmt.Errorf("stock: reading what order %d has taken: %w", sale.OrderID, err)
	}
	taken := make(map[stock.Use]stock.Qty, len(rows))
	for _, r := range rows {
		taken[stock.Use{StockItemID: r.StockItemID, ItemID: r.MenuItemID}] = stock.Qty(r.Taken)
	}
	if err := recordDishUnits(ctx, q, sale, recipes); err != nil {
		return err
	}
	deltas := stock.SaleDeltas(stock.Usage(sale.Lines, recipes), taken)
	if len(deltas) == 0 {
		return nil
	}
	shiftID, err := openShift(ctx, q, sale.VenueID)
	if err != nil {
		return err
	}
	for _, d := range deltas {
		if _, err := q.InsertStockMovement(ctx, gen.InsertStockMovementParams{
			VenueID: sale.VenueID, StockItemID: d.StockItemID, Kind: string(stock.Sold), Delta: int64(d.Delta),
			StaffID: sale.StaffID, ShiftID: shiftID, OrderID: sale.OrderID, CreatedAt: sale.At.UTC(), MenuItemID: d.ItemID,
		}); err != nil {
			return fmt.Errorf("stock: recording what order %d used of %d: %w", sale.OrderID, d.StockItemID, err)
		}
	}
	return nil
}

// recordDishUnits writes how many of each dish this close accounts for,
// less what the order's earlier closes recorded (ADR-189): the units a
// dish's cost in the ledger covers.
func recordDishUnits(ctx context.Context, q *gen.Queries, sale Sale, recipes stock.Recipes) error {
	rows, err := q.DishUnitsForOrder(ctx, gen.DishUnitsForOrderParams{VenueID: sale.VenueID, OrderID: sale.OrderID})
	if err != nil {
		return fmt.Errorf("stock: reading the dish units of order %d: %w", sale.OrderID, err)
	}
	recorded := make(map[int64]int64, len(rows))
	for _, r := range rows {
		recorded[r.MenuItemID] = r.Units
	}
	for _, d := range stock.UnitDeltas(stock.DishUnits(sale.Lines, recipes), recorded) {
		if err := q.InsertDishSale(ctx, gen.InsertDishSaleParams{
			VenueID: sale.VenueID, OrderID: sale.OrderID, MenuItemID: d.ItemID, Qty: d.Delta, CreatedAt: sale.At.UTC(),
		}); err != nil {
			return fmt.Errorf("stock: recording %d of dish %d for order %d: %w", d.Delta, d.ItemID, sale.OrderID, err)
		}
	}
	return nil
}

func readRecipes(ctx context.Context, q *gen.Queries, venueID int64) (stock.Recipes, error) {
	items, err := q.ItemRecipesForVenue(ctx, venueID)
	if err != nil {
		return stock.Recipes{}, fmt.Errorf("stock: reading item recipes: %w", err)
	}
	mods, err := q.ModifierRecipesForVenue(ctx, venueID)
	if err != nil {
		return stock.Recipes{}, fmt.Errorf("stock: reading modifier recipes: %w", err)
	}
	r := stock.Recipes{Items: map[int64][]stock.Part{}, Modifiers: map[int64][]stock.Part{}}
	for _, row := range items {
		r.Items[row.ItemID] = append(r.Items[row.ItemID], stock.Part{StockItemID: row.StockItemID, Qty: stock.Qty(row.Qty)})
	}
	for _, row := range mods {
		r.Modifiers[row.ModifierID] = append(r.Modifiers[row.ModifierID], stock.Part{StockItemID: row.StockItemID, Qty: stock.Qty(row.Qty)})
	}
	return r, nil
}

// book lists recipes by id, in id order.
func book(m map[int64][]stock.Part) []Recipe {
	out := make([]Recipe, 0, len(m))
	for id, parts := range m {
		r := Recipe{ID: id, Parts: make([]RecipePart, len(parts))}
		for i, p := range parts {
			r.Parts[i] = RecipePart{StockItemID: p.StockItemID, Qty: p.Qty}
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Recipe) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

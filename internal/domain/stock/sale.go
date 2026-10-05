package stock

import (
	"cmp"
	"slices"
	"strconv"
)

// Part is one stock item a recipe uses, per unit sold: 18 g of beans in a
// latte is {beans, 18000}.
type Part struct {
	StockItemID int64
	Qty         Qty
}

// SoldLine is what a sale used: a menu item, the modifiers chosen on it,
// and how many. A voided line is not a SoldLine; a comped one is, because
// it was made.
type SoldLine struct {
	ItemID      int64
	ModifierIDs []int64
	Qty         int64
}

// Recipes is every recipe a venue has: by menu item, and by modifier. A
// modifier's recipe is added to its item's, so a substitution is modelled
// on the modifiers: Dairy uses 200 ml of milk and Oat 200 ml of oat milk,
// and the latte's own recipe holds only what every latte uses.
type Recipes struct {
	Items     map[int64][]Part
	Modifiers map[int64][]Part
}

// Use is a stock item used for a dish. ItemID is the menu item the stock
// went into; a modifier's stock is its line's item's. Zero is "no dish":
// stock taken before the ledger kept the dish (rebuild Task 16d3, ADR-189).
type Use struct {
	StockItemID int64
	ItemID      int64
}

// Usage is how much of each stock item the lines used, and for which dish.
// An item or modifier with no recipe uses nothing.
func Usage(lines []SoldLine, r Recipes) map[Use]Qty {
	out := map[Use]Qty{}
	for _, l := range lines {
		if l.Qty <= 0 {
			continue
		}
		add := func(parts []Part) {
			for _, p := range parts {
				out[Use{StockItemID: p.StockItemID, ItemID: l.ItemID}] += p.Qty * Qty(l.Qty)
			}
		}
		add(r.Items[l.ItemID])
		for _, m := range l.ModifierIDs {
			add(r.Modifiers[m])
		}
	}
	for u, q := range out {
		if q == 0 {
			delete(out, u)
		}
	}
	return out
}

// SaleDelta is one sold movement to write: the ledger delta for a stock
// item on account of a dish, negative when the sale used more.
type SaleDelta struct {
	StockItemID int64
	ItemID      int64
	Delta       Qty
}

// SaleDeltas is what to write so an order's sold movements add up to its
// usage: usage less what was already deducted for it, per stock item and
// dish, in stock item order and then by dish. deducted is the stock already
// taken by the order's earlier sold movements, as a positive amount.
//
// It makes closing an order idempotent: a second close with the same lines
// writes nothing, and a close after a reopen that voided a line writes a
// positive delta that puts its stock back.
func SaleDeltas(usage, deducted map[Use]Qty) []SaleDelta {
	uses := make([]Use, 0, len(usage)+len(deducted))
	for u := range usage {
		uses = append(uses, u)
	}
	for u := range deducted {
		if _, ok := usage[u]; !ok {
			uses = append(uses, u)
		}
	}
	slices.SortFunc(uses, func(a, b Use) int {
		return cmp.Or(cmp.Compare(a.StockItemID, b.StockItemID), cmp.Compare(a.ItemID, b.ItemID))
	})
	var out []SaleDelta
	for _, u := range uses {
		if d := deducted[u] - usage[u]; d != 0 {
			out = append(out, SaleDelta{StockItemID: u.StockItemID, ItemID: u.ItemID, Delta: d})
		}
	}
	return out
}

// DishUnits is how many of each dish the lines sold that used stock: the
// units a dish's cost in the ledger covers (ADR-189). A dish counts when its
// own recipe, or the recipe of a modifier chosen on it, uses anything. A
// dish with no recipe is left out: nothing costed it.
func DishUnits(lines []SoldLine, r Recipes) map[int64]int64 {
	out := map[int64]int64{}
	for _, l := range lines {
		if l.Qty <= 0 {
			continue
		}
		used := len(r.Items[l.ItemID]) > 0
		for _, m := range l.ModifierIDs {
			used = used || len(r.Modifiers[m]) > 0
		}
		if used {
			out[l.ItemID] += l.Qty
		}
	}
	return out
}

// UnitDelta is a change to how many of a dish an order's closes have
// accounted for.
type UnitDelta struct {
	ItemID int64
	Delta  int64
}

// UnitDeltas is what to record so an order's dish units add up to now:
// now less what its earlier closes recorded, per dish, in dish order. Like
// SaleDeltas it makes a close idempotent.
func UnitDeltas(now, recorded map[int64]int64) []UnitDelta {
	ids := make([]int64, 0, len(now)+len(recorded))
	for id := range now {
		ids = append(ids, id)
	}
	for id := range recorded {
		if _, ok := now[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	var out []UnitDelta
	for _, id := range ids {
		if d := now[id] - recorded[id]; d != 0 {
			out = append(out, UnitDelta{ItemID: id, Delta: d})
		}
	}
	return out
}

// RecipeProblem checks a recipe as the back office would save it: each
// stock item once, each amount above zero and in range. Field names the
// row, as "parts.2.qty".
func RecipeProblem(parts []Part) *Problem {
	seen := make(map[int64]bool, len(parts))
	for i, p := range parts {
		switch {
		case seen[p.StockItemID]:
			return &Problem{partField(i, "stock_item_id"), "That stock item is already in this recipe."}
		case p.Qty <= 0:
			return &Problem{partField(i, "qty"), "Give how much one uses."}
		case p.Qty > MaxQty:
			return &Problem{partField(i, "qty"), "That amount is too large."}
		}
		seen[p.StockItemID] = true
	}
	return nil
}

func partField(i int, field string) string {
	return "parts." + strconv.Itoa(i) + "." + field
}

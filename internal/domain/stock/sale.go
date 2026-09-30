package stock

import (
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

// Usage is how much of each stock item the lines used, by stock item id.
// An item or modifier with no recipe uses nothing.
func Usage(lines []SoldLine, r Recipes) map[int64]Qty {
	out := map[int64]Qty{}
	add := func(parts []Part, n int64) {
		for _, p := range parts {
			out[p.StockItemID] += p.Qty * Qty(n)
		}
	}
	for _, l := range lines {
		if l.Qty <= 0 {
			continue
		}
		add(r.Items[l.ItemID], l.Qty)
		for _, m := range l.ModifierIDs {
			add(r.Modifiers[m], l.Qty)
		}
	}
	for id, q := range out {
		if q == 0 {
			delete(out, id)
		}
	}
	return out
}

// SaleDelta is one sold movement to write: the ledger delta for a stock
// item, negative when the sale used more.
type SaleDelta struct {
	StockItemID int64
	Delta       Qty
}

// SaleDeltas is what to write so an order's sold movements add up to its
// usage: usage less what was already deducted for it, per stock item,
// in stock item order. deducted is the stock already taken by the order's
// earlier sold movements, as a positive amount.
//
// It makes closing an order idempotent: a second close with the same lines
// writes nothing, and a close after a reopen that voided a line writes a
// positive delta that puts its stock back.
func SaleDeltas(usage, deducted map[int64]Qty) []SaleDelta {
	ids := make([]int64, 0, len(usage)+len(deducted))
	for id := range usage {
		ids = append(ids, id)
	}
	for id := range deducted {
		if _, ok := usage[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	var out []SaleDelta
	for _, id := range ids {
		if d := deducted[id] - usage[id]; d != 0 {
			out = append(out, SaleDelta{StockItemID: id, Delta: d})
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

package report

import (
	"math/big"

	"github.com/byjoellobo/mez/internal/domain/money"
	"github.com/byjoellobo/mez/internal/domain/zreport"
)

// Costing is how much of a dish's cost the ledger knows over a range.
type Costing string

const (
	// CostFull: every movement its sales made had a cost. Its margin is given.
	CostFull Costing = "full"
	// CostPart: some of it is known. A stock item it uses has no cost, the
	// ledger costed other than the units it sold (it sold some before it had
	// a recipe), stock was put back for it after a reopen, or the range
	// reaches back before the ledger kept dishes.
	CostPart Costing = "part"
	// CostNone: its sales took no stock under its name: it has no recipe,
	// or it sold only before the ledger kept dishes.
	CostNone Costing = "none"
)

// Costings is every value, for the generated vocabulary.
func Costings() []Costing { return []Costing{CostFull, CostPart, CostNone} }

// DishCost is what a dish's sales took from stock over a range, at the cost
// each movement carried (ADR-124, ADR-189).
type DishCost struct {
	Cost      money.Amount
	Movements int64
	// Uncosted is how many of the movements had no cost and are not in Cost.
	Uncosted int64
	// Units is how many of the dish the ledger accounted for: what Cost covers.
	Units int64
	// PutBack is how many of the movements put stock back.
	PutBack int64
}

// ItemMargin is one menu item over a range: what it sold, what it cost to
// make, and what was left.
type ItemMargin struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Qty  int64  `json:"qty"`
	// Net is its sales after discounts and before tax: tax is not the
	// venue's money, so the margin is against this.
	Net     money.Amount `json:"net"`
	Costing Costing      `json:"costing"`
	// Cost is what the ledger knows of its cost; absent when it knows none.
	// With CostPart it is short of the whole. CostedQty is how many units it
	// covers: fewer than Qty when the dish also sold before it had a recipe.
	Cost      *money.Amount `json:"cost,omitempty"`
	CostedQty int64         `json:"costed_qty"`
	// Margin is Net less Cost, and MarginBP that as hundredths of a percent
	// of Net. Both only with CostFull; MarginBP only when Net is above zero.
	Margin   *money.Amount `json:"margin,omitempty"`
	MarginBP *int64        `json:"margin_bp,omitempty"`
}

// Margins sets each item sold beside what it cost to make. items are the
// report's, in its order; costs is by the item's key. short says the range
// has sales the ledger did not keep by dish, so no cost in it can be taken
// as whole.
func Margins(items []zreport.Group, costs map[string]DishCost, short bool) []ItemMargin {
	out := make([]ItemMargin, 0, len(items))
	for _, g := range items {
		m := ItemMargin{Key: g.Key, Name: g.Name, Qty: g.Qty, Net: g.Net, Costing: CostNone}
		c, ok := costs[g.Key]
		if !ok || c.Movements == 0 {
			out = append(out, m)
			continue
		}
		cost := c.Cost
		m.Cost = &cost
		m.CostedQty = c.Units
		// A cost that covers other than the units sold is not this row's cost:
		// the dish sold some before it had a recipe, or closed on another day.
		// Nor is one with stock put back: that is valued at the cost of the day
		// it went back, which need not be the cost it was taken at.
		if short || c.Uncosted > 0 || c.Units != g.Qty || c.PutBack > 0 {
			m.Costing = CostPart
			out = append(out, m)
			continue
		}
		m.Costing = CostFull
		margin := money.Amount{Minor: g.Net.Minor - cost.Minor, Currency: g.Net.Currency}
		m.Margin = &margin
		if g.Net.Minor > 0 {
			bp := basisPoints(margin.Minor, g.Net.Minor)
			m.MarginBP = &bp
		}
		out = append(out, m)
	}
	return out
}

// basisPoints is part over whole in hundredths of a percent, rounded to the
// nearest with a half going away from zero. whole is above zero. Whole
// numbers throughout.
func basisPoints(part, whole int64) int64 {
	n := new(big.Int).Mul(big.NewInt(part), big.NewInt(20000))
	w := big.NewInt(whole)
	if n.Sign() < 0 {
		n.Sub(n, w)
	} else {
		n.Add(n, w)
	}
	n.Quo(n, new(big.Int).Mul(w, big.NewInt(2)))
	return n.Int64()
}

package stock

import (
	"errors"
	"math/big"
)

// A stock item's cost is in minor units of the venue's currency for one
// costing measure of it: a kilogram for an item counted in grams, a litre
// for one in millilitres, one for one counted by the piece. That is how an
// invoice from a supplier prices it, so it is what an owner can type
// (phase 10, Task 5f1, ADR-124). Zero is "not known".

// MaxCost bounds a cost: ten million in major units of a two-decimal
// currency for a kilogram of anything.
const MaxCost int64 = 1_000_000_000

// ErrValue is a quantity whose worth cannot be worked out.
var ErrValue = errors.New("stock: the value cannot be worked out")

// CostMeasure is the quantity a cost is quoted for: a kilogram, a litre, or
// one of something.
func CostMeasure(u Unit) Qty {
	if u == Each {
		return PerUnit
	}
	return 1000 * PerUnit
}

// CleanCost checks a cost in minor units.
func CleanCost(cost int64) *Problem {
	switch {
	case cost < 0:
		return &Problem{"cost", "The cost cannot be below zero."}
	case cost > MaxCost:
		return &Problem{"cost", "That cost is too large to keep."}
	}
	return nil
}

// Value is what a quantity is worth at a cost, in minor units, rounded to
// the nearest with a half going away from zero. The quantity may be
// negative: a count that found less is worth a loss. Whole numbers
// throughout, as money and stock both are.
func Value(q Qty, u Unit, cost int64) (int64, error) {
	if !validUnit(u) || cost < 0 {
		return 0, ErrValue
	}
	measure := big.NewInt(int64(CostMeasure(u)))
	n := new(big.Int).Mul(big.NewInt(int64(q)), big.NewInt(cost))
	// Twice the product plus or minus the divisor, over twice the divisor:
	// a quotient that truncates toward zero then rounds half away from it.
	n.Mul(n, big.NewInt(2))
	if n.Sign() < 0 {
		n.Sub(n, measure)
	} else {
		n.Add(n, measure)
	}
	n.Quo(n, new(big.Int).Mul(measure, big.NewInt(2)))
	if !n.IsInt64() {
		return 0, ErrValue
	}
	return n.Int64(), nil
}

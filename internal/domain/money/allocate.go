package money

import (
	"cmp"
	"fmt"
	"math/bits"
	"slices"
)

// Allocate divides a non-negative amount across parts in proportion to their
// weights, by the largest remainder method, so the parts sum to exactly the
// total.
//
// It is how an order-level discount is spread over the lines before tax: a
// 50.00 discount on lines worth 300.00 and 100.00 is 37.50 and 12.50. Each
// part is total × weight / sum of weights, rounded down; the minor units
// left over go one each to the parts with the largest remainders, earliest
// first on a tie, so the answer is deterministic.
//
// Weights must not be negative. When they sum to zero there is nothing to
// weigh by, so a zero total is all zeros and anything else is ErrInvalidSplit.
// The arithmetic is 128-bit, so no total or weight can overflow it.
func Allocate(total Amount, weights []int64) ([]Amount, error) {
	if total.Minor < 0 {
		return nil, fmt.Errorf("money: allocating %d: %w", total.Minor, ErrInvalidSplit)
	}
	var sum uint64
	for i, w := range weights {
		if w < 0 {
			return nil, fmt.Errorf("money: weight %d is %d: %w", i, w, ErrInvalidSplit)
		}
		var carry uint64
		if sum, carry = bits.Add64(sum, uint64(w), 0); carry != 0 {
			return nil, fmt.Errorf("money: weights sum past 2^64: %w", ErrOverflow)
		}
	}

	parts := make([]Amount, len(weights))
	for i := range parts {
		parts[i] = Amount{Minor: 0, Currency: total.Currency}
	}
	if sum == 0 {
		if total.Minor != 0 {
			return nil, fmt.Errorf("money: allocating %d over zero weight: %w", total.Minor, ErrInvalidSplit)
		}
		return parts, nil
	}

	type share struct {
		i         int
		remainder uint64
	}
	shares := make([]share, len(weights))
	var given int64
	for i, w := range weights {
		// total × w / sum fits in 64 bits because w <= sum, so Div64 cannot
		// overflow.
		hi, lo := bits.Mul64(uint64(total.Minor), uint64(w))
		q, r := bits.Div64(hi, lo, sum)
		parts[i].Minor = int64(q)
		given += int64(q)
		shares[i] = share{i: i, remainder: r}
	}

	slices.SortStableFunc(shares, func(a, b share) int { return cmp.Compare(b.remainder, a.remainder) })
	for k := range total.Minor - given {
		parts[shares[k].i].Minor++
	}
	return parts, nil
}

package money

import "fmt"

// RoundToNearest rounds the amount to the nearest whole multiple of step minor
// units, half to even. A step of 50 on an INR amount settles a bill to the
// nearest fifty paise, which is what a till with no one-paisa coins needs; a
// step of 1 changes nothing.
//
// This is the venue's cash-rounding rule, and per docs/02-domain.md it is
// applied once, at the end. Rounding a subtotal and then rounding the total
// again is how a Z-report stops balancing.
func (a Amount) RoundToNearest(step int64) (Amount, error) {
	if step <= 0 {
		return Amount{}, fmt.Errorf("money: step of %d minor units: %w", step, ErrInvalidStep)
	}

	steps, err := divRoundHalfEven(a.Minor, step)
	if err != nil {
		return Amount{}, fmt.Errorf("money: rounding %d to a step of %d: %w", a.Minor, step, err)
	}

	minor, ok := mul64(steps, step)
	if !ok {
		return Amount{}, fmt.Errorf("money: rounding %d to a step of %d: %w", a.Minor, step, ErrOverflow)
	}
	return Amount{Minor: minor, Currency: a.Currency}, nil
}

// divRoundHalfEven divides n by d and rounds the result to a whole number,
// half to even. d must be positive.
//
// Half to even, banker's rounding, is the rule docs/02-domain.md settles on
// because it has no directional bias: over a day of bills the halves that go
// up and the halves that go down cancel, where always-up quietly overcharges.
//
// The whole calculation is integer arithmetic. "Is the remainder exactly half"
// is a question about integers, so no float, and no big.Rat, is needed to
// answer it: compare the remainder with half the divisor, and note that an odd
// divisor can never produce an exact half.
func divRoundHalfEven(n, d int64) (int64, error) {
	if d <= 0 {
		return 0, fmt.Errorf("money: dividing by %d: %w", d, ErrInvalidStep)
	}

	quotient := n / d // Go truncates toward zero.
	remainder := n % d
	if remainder == 0 {
		return quotient, nil
	}

	// Work with the size of the remainder and decide the direction from the
	// sign of n, so the rule is symmetric around zero.
	size := remainder
	if size < 0 {
		size = -size
	}
	half := d / 2

	away := func() int64 {
		if n > 0 {
			return quotient + 1
		}
		return quotient - 1
	}

	switch {
	case size > half:
		// More than half a unit left over: round away from zero.
		return away(), nil
	case size == half && d%2 == 0:
		// Exactly half. An odd divisor cannot land here, because twice a
		// remainder is even. Move only if the truncated quotient is odd, so
		// the answer is always the even neighbour.
		if quotient%2 != 0 {
			return away(), nil
		}
		return quotient, nil
	default:
		// Less than half: truncation already gave the nearest whole number.
		return quotient, nil
	}
}

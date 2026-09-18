package money

import "fmt"

// SplitEvenly divides an amount into n parts that sum to exactly the original,
// by the largest remainder method.
//
// A thousand rupees three ways is 333.34 / 333.33 / 333.33, never three times
// 333.33 with a paisa lost somewhere. Because every part carries the same
// weight, every remainder is equal, so the tie is broken by position: the
// earliest parts carry the extra minor unit. That makes the answer
// deterministic, which is what lets a split payment be replayed from the event
// stream and land on the same numbers.
//
// A negative total splits symmetrically, so reversing a split payment returns
// each payer exactly what they put in.
func SplitEvenly(total Amount, n int) ([]Amount, error) {
	if n < 1 {
		return nil, fmt.Errorf("money: splitting into %d parts: %w", n, ErrInvalidSplit)
	}

	parts := make([]Amount, n)

	// Go truncates toward zero, so base and remainder carry the sign of the
	// total and the arithmetic below needs no special case for a refund.
	base := total.Minor / int64(n)
	remainder := total.Minor % int64(n)

	extra := int64(1)
	if remainder < 0 {
		extra = -1
		remainder = -remainder
	}

	for i := range parts {
		minor := base
		if int64(i) < remainder {
			minor += extra
		}
		parts[i] = Amount{Minor: minor, Currency: total.Currency}
	}

	return parts, nil
}

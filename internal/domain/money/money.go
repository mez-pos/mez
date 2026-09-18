package money

import (
	"fmt"
	"math"
)

// basisPointsPerWhole is the denominator of a basis point rate: 10000 basis
// points is 100%, so 1850 is 18.50%.
const basisPointsPerWhole = 10000

// Amount is a quantity of money: a whole number of a currency's minor units,
// and the currency it is in. 123400 INR is one thousand two hundred and thirty
// four rupees, held as paise.
//
// The zero value, Amount{}, is not a usable amount. It has no currency, so it
// fails every operation that needs one. That is deliberate: see doc.go.
type Amount struct {
	Minor    int64  `json:"minor"`
	Currency string `json:"currency"`
}

// New builds an Amount, checking the currency and rejecting a minor value that
// cannot be negated. Use it at every boundary where money arrives from outside
// this package: a request body, a file, a database row.
func New(minor int64, currency string) (Amount, error) {
	if err := checkCurrency(currency); err != nil {
		return Amount{}, err
	}
	if !representable(minor) {
		return Amount{}, fmt.Errorf("money: %d minor units: %w", minor, ErrOverflow)
	}
	return Amount{Minor: minor, Currency: currency}, nil
}

// IsZero reports whether the amount is zero, whatever its currency. Nothing
// costs zero in a currency it is not priced in, so this needs no currency
// check.
func (a Amount) IsZero() bool { return a.Minor == 0 }

// Add returns a + b. The currencies must match exactly.
func (a Amount) Add(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, mismatch(a.Currency, b.Currency)
	}
	sum, ok := add64(a.Minor, b.Minor)
	if !ok {
		return Amount{}, fmt.Errorf("money: %d + %d minor units: %w", a.Minor, b.Minor, ErrOverflow)
	}
	return Amount{Minor: sum, Currency: a.Currency}, nil
}

// Sub returns a - b. The currencies must match exactly.
func (a Amount) Sub(b Amount) (Amount, error) {
	if a.Currency != b.Currency {
		return Amount{}, mismatch(a.Currency, b.Currency)
	}
	diff, ok := sub64(a.Minor, b.Minor)
	if !ok {
		return Amount{}, fmt.Errorf("money: %d - %d minor units: %w", a.Minor, b.Minor, ErrOverflow)
	}
	return Amount{Minor: diff, Currency: a.Currency}, nil
}

// Neg returns the amount with its sign flipped, which is how a refund is
// expressed against a payment.
//
// It returns an error rather than an Amount alone so that no operation in this
// package can quietly produce a wrong number: math.MinInt64 has no positive
// counterpart. New refuses that value, so this only fails for an Amount built
// by struct literal.
func (a Amount) Neg() (Amount, error) {
	if a.Minor == math.MinInt64 {
		return Amount{}, fmt.Errorf("money: negating %d minor units: %w", a.Minor, ErrOverflow)
	}
	return Amount{Minor: -a.Minor, Currency: a.Currency}, nil
}

// Compare returns -1 if a is less than b, 0 if they are equal, and 1 if a is
// greater. The currencies must match: there is no order between rupees and
// dollars.
func (a Amount) Compare(b Amount) (int, error) {
	if a.Currency != b.Currency {
		return 0, mismatch(a.Currency, b.Currency)
	}
	switch {
	case a.Minor < b.Minor:
		return -1, nil
	case a.Minor > b.Minor:
		return 1, nil
	default:
		return 0, nil
	}
}

// Mul applies a rate given in basis points: 1850 is 18.50%, 10000 is 100%, and
// a negative rate is a deduction. The result is rounded once, half to even.
//
// Rates are basis points and not floats on purpose. 18.5% of a bill is a
// question about integers, and asking it in floating point is how a tax line
// ends up a paisa out from the tax register.
func (a Amount) Mul(basisPoints int32) (Amount, error) {
	product, ok := mul64(a.Minor, int64(basisPoints))
	if !ok {
		return Amount{}, fmt.Errorf("money: %d minor units at %d bp: %w", a.Minor, basisPoints, ErrOverflow)
	}
	minor, err := divRoundHalfEven(product, basisPointsPerWhole)
	if err != nil {
		return Amount{}, fmt.Errorf("money: %d minor units at %d bp: %w", a.Minor, basisPoints, err)
	}
	return Amount{Minor: minor, Currency: a.Currency}, nil
}

// Times returns the amount multiplied by a whole count: a unit price times a
// line's quantity. No rounding is involved, so none happens; a result that
// does not fit in an int64 is ErrOverflow.
func (a Amount) Times(n int64) (Amount, error) {
	product, ok := mul64(a.Minor, n)
	if !ok {
		return Amount{}, fmt.Errorf("money: %d minor units times %d: %w", a.Minor, n, ErrOverflow)
	}
	return Amount{Minor: product, Currency: a.Currency}, nil
}

// Sum adds amounts that must all be in the named currency. An empty list sums
// to zero in that currency, which is the answer an order with no lines needs.
//
// The currency is a parameter rather than being read off the first amount so
// that summing nothing still produces a usable total, and so that a list which
// happens to be entirely in the wrong currency fails instead of agreeing with
// itself.
func Sum(currency string, amounts ...Amount) (Amount, error) {
	if err := checkCurrency(currency); err != nil {
		return Amount{}, err
	}

	total := Amount{Minor: 0, Currency: currency}
	for i, a := range amounts {
		if a.Currency != currency {
			return Amount{}, fmt.Errorf("money: amount %d of %d: %w", i+1, len(amounts), mismatch(currency, a.Currency))
		}
		sum, ok := add64(total.Minor, a.Minor)
		if !ok {
			return Amount{}, fmt.Errorf("money: summing amount %d of %d: %w", i+1, len(amounts), ErrOverflow)
		}
		total.Minor = sum
	}
	return total, nil
}

// The three helpers below report false rather than returning a wrapped result,
// and they also refuse math.MinInt64 even though it fits. That value has no
// positive counterpart, so an Amount holding it could not be negated for a
// refund. Refusing it here is what makes the guarantee in Neg true: no amount
// this package produces can fail to negate.

// add64 adds two int64 values, reporting false if the result would wrap or
// would be a value an Amount may not hold.
func add64(a, b int64) (int64, bool) {
	sum := a + b
	if (b > 0 && sum < a) || (b < 0 && sum > a) {
		return 0, false
	}
	return sum, representable(sum)
}

// sub64 subtracts two int64 values, reporting false if the result would wrap
// or would be a value an Amount may not hold.
func sub64(a, b int64) (int64, bool) {
	diff := a - b
	if (b < 0 && diff < a) || (b > 0 && diff > a) {
		return 0, false
	}
	return diff, representable(diff)
}

// mul64 multiplies two int64 values, reporting false if the result would wrap
// or would be a value an Amount may not hold.
func mul64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == math.MinInt64 && b == -1) || (b == math.MinInt64 && a == -1) {
		return 0, false
	}
	product := a * b
	if product/b != a {
		return 0, false
	}
	return product, representable(product)
}

// representable reports whether a number of minor units is one an Amount is
// allowed to hold.
func representable(minor int64) bool { return minor != math.MinInt64 }

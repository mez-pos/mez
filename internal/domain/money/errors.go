package money

import (
	"errors"
	"fmt"
)

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrInvalidCurrency means the currency is not three uppercase ASCII
	// letters. The code is not checked against a list of real currencies:
	// see doc.go for why.
	ErrInvalidCurrency = errors.New("currency must be three uppercase letters")

	// ErrCurrencyMismatch means two amounts in different currencies met in an
	// operation that requires one currency. Every mismatch error in this
	// package unwraps to this, and also carries both sides as a
	// *CurrencyMismatchError.
	ErrCurrencyMismatch = errors.New("currency mismatch")

	// ErrOverflow means the result does not fit in an int64 of minor units.
	// The package returns this rather than wrapping around, because a total
	// that silently wraps is worse than a total that fails.
	ErrOverflow = errors.New("amount out of range")

	// ErrSyntax means a string was not a recognisable amount.
	ErrSyntax = errors.New("malformed amount")

	// ErrPrecision means a string carried more decimal places than the
	// currency has minor units, so honouring it would mean inventing or
	// discarding money.
	ErrPrecision = errors.New("more decimal places than the currency has")

	// ErrInvalidSplit means a split was asked for fewer than one part.
	ErrInvalidSplit = errors.New("split needs at least one part")

	// ErrInvalidStep means a rounding step was not a positive number of minor
	// units.
	ErrInvalidStep = errors.New("rounding step must be positive")
)

// CurrencyMismatchError reports which two currencies met. It exists so that a
// handler can tell the user what actually went wrong instead of "currency
// mismatch", which is useless on a busy counter.
type CurrencyMismatchError struct {
	Want string // the currency the operation was working in
	Got  string // the currency that turned up instead
}

func (e *CurrencyMismatchError) Error() string {
	return fmt.Sprintf("currency mismatch: want %q, got %q", e.Want, e.Got)
}

// Unwrap lets errors.Is(err, ErrCurrencyMismatch) succeed while errors.As
// still recovers both currencies.
func (e *CurrencyMismatchError) Unwrap() error { return ErrCurrencyMismatch }

// mismatch builds the error for two currencies that should have been equal.
func mismatch(want, got string) error {
	return &CurrencyMismatchError{Want: want, Got: got}
}

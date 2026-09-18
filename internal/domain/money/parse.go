package money

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// String renders the amount as "1234.00 INR": the major unit, a decimal point,
// as many decimal places as the currency has, then the code. It is a canonical
// form, not a display form. Parse reads back anything String writes.
//
// The customer-facing rendering, with the right symbol, grouping and placement
// for the venue's locale, belongs in the UI layer. This package knows nothing
// about symbols.
func (a Amount) String() string {
	exponent := Exponent(a.Currency)

	// Format the whole int64 and slice it, rather than taking an absolute
	// value first: math.MinInt64 has no positive counterpart and would
	// overflow on the way.
	digits := strconv.FormatInt(a.Minor, 10)
	sign := ""
	if rest, found := strings.CutPrefix(digits, "-"); found {
		sign, digits = "-", rest
	}

	if exponent == 0 {
		return sign + digits + " " + a.Currency
	}

	for len(digits) <= exponent {
		digits = "0" + digits
	}
	split := len(digits) - exponent

	return sign + digits[:split] + "." + digits[split:] + " " + a.Currency
}

// Parse reads an amount written as "1234.00 INR" or "INR 1234.00". Digit
// grouping is tolerated on the whole part, so "1,00,000.00 INR" works, as does
// a leading + or -.
//
// Fewer decimal places than the currency has are padded: "12.3 INR" is 1230
// paise. More are refused rather than rounded, because rounding at the parsing
// boundary is rounding at the wrong moment, and silently dropping a digit from
// a file a venue imported is how money goes missing.
func Parse(s string) (Amount, error) {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return Amount{}, fmt.Errorf("money: %q is not an amount and a currency: %w", s, ErrSyntax)
	}

	// The number is whichever field starts like one; the other is the code.
	number, currency := fields[0], fields[1]
	if !looksNumeric(number) {
		number, currency = fields[1], fields[0]
	}

	whole, fraction, err := scanDecimal(number)
	if err != nil {
		return Amount{}, fmt.Errorf("money: %q: %w", s, err)
	}
	if err := checkCurrency(currency); err != nil {
		return Amount{}, fmt.Errorf("money: %q: %w", s, err)
	}

	exponent := Exponent(currency)
	if len(fraction) > exponent {
		return Amount{}, fmt.Errorf("money: %q has %d decimal places but %s has %d: %w",
			s, len(fraction), currency, exponent, ErrPrecision)
	}

	// Shift the decimal point by padding, so the conversion below is a plain
	// integer parse and no division ever happens.
	minor, err := strconv.ParseInt(whole+fraction+strings.Repeat("0", exponent-len(fraction)), 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return Amount{}, fmt.Errorf("money: %q: %w", s, ErrOverflow)
		}
		return Amount{}, fmt.Errorf("money: %q: %w", s, ErrSyntax)
	}

	return New(minor, currency)
}

// looksNumeric reports whether a field is where the number is, so that Parse
// accepts the code on either side without guessing wrong about "abc INR".
func looksNumeric(s string) bool {
	if s == "" {
		return false
	}
	switch c := s[0]; {
	case c >= '0' && c <= '9', c == '+', c == '-', c == '.':
		return true
	default:
		return false
	}
}

// scanDecimal splits a signed decimal string into the digits before the point
// and the digits after it, keeping the sign on the whole part. It accepts
// group separators before the point and nothing else: no spaces, no currency
// symbols, and no exponent notation, which is not a way to write money.
func scanDecimal(s string) (whole, fraction string, err error) {
	var before, after strings.Builder

	sign := ""
	if rest, found := strings.CutPrefix(s, "-"); found {
		sign, s = "-", rest
	} else if rest, found := strings.CutPrefix(s, "+"); found {
		s = rest
	}

	seenPoint := false
	for i := range len(s) {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			if seenPoint {
				after.WriteByte(c)
			} else {
				before.WriteByte(c)
			}
		case c == '.':
			if seenPoint {
				return "", "", fmt.Errorf("two decimal points: %w", ErrSyntax)
			}
			seenPoint = true
		case c == ',' && !seenPoint:
			// A digit group separator. Its placement is not checked: this is
			// a tolerance for human input, not a locale validator.
		default:
			return "", "", fmt.Errorf("unexpected %q: %w", string(c), ErrSyntax)
		}
	}

	if before.Len() == 0 && after.Len() == 0 {
		return "", "", fmt.Errorf("no digits: %w", ErrSyntax)
	}

	return sign + before.String(), after.String(), nil
}

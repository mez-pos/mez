package sheet

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// Money reads a cell as an amount of the currency, in minor units. It reads
// digits, never a float: "120.5" is 12050 paise because the point is moved,
// not because anything was multiplied.
//
// A symbol, "Rs" or the currency's code around the number is dropped. A
// comma is taken only where it groups digits ("1,150", "1,00,000"): "12,50"
// is twelve and a half the way much of the world writes it, and money.Parse,
// which reads any comma as a grouping, would make it 1250. An amount below
// zero is read as it is; whether that is allowed is the caller's.
//
// The string is what is wrong with the cell, or empty. what is the thing
// the cell is, as "price" or "cost".
func Money(cell, currency, what string) (int64, string) {
	number := bareNumber(cell, currency)
	if !groupedLikeMoney(number) {
		return 0, fmt.Sprintf("%q: write the %s with a point for the decimals, like 120.50, and no comma.", cell, what)
	}
	amount, err := money.Parse(number + " " + currency)
	switch {
	case errors.Is(err, money.ErrPrecision):
		return 0, fmt.Sprintf("%q: %s", cell, decimalPlaces(currency, what))
	case err != nil:
		return 0, fmt.Sprintf("%q is not a %s. Type a number like 120 or 120.50.", cell, what)
	}
	return amount.Minor, ""
}

// decimalPlaces says how many decimal places an amount in the currency may have.
func decimalPlaces(currency, what string) string {
	n := money.Exponent(currency)
	if n == 0 {
		return fmt.Sprintf("%ss in %s have no decimal places.", what, currency)
	}
	return fmt.Sprintf("%ss in %s have at most %d decimal places.", what, currency, n)
}

// bareNumber is a cell without what a spreadsheet's currency format puts
// around the number: a symbol, "Rs" or the currency's code, on either side.
func bareNumber(cell, currency string) string {
	s := strings.TrimFunc(cell, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Sc, r) })
	lower := strings.ToLower(s)
	for _, word := range []string{strings.ToLower(currency), "rs.", "rs"} {
		if strings.HasPrefix(lower, word) {
			s = s[len(word):]
			break
		}
	}
	if strings.HasSuffix(strings.ToLower(s), strings.ToLower(currency)) {
		s = s[:len(s)-len(currency)]
	}
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Sc, r) })
}

// groupedLikeMoney reports whether every comma in a number is one that
// groups digits.
func groupedLikeMoney(number string) bool {
	whole, _, _ := strings.Cut(number, ".")
	groups := strings.Split(whole, ",")
	if len(groups) == 1 {
		return true
	}
	last := len(groups) - 1
	for _, g := range groups[1:last] {
		if len(g) != 2 && len(g) != 3 {
			return false
		}
	}
	return len(groups[last]) == 3
}

// Decimal reads a cell as a number with at most places decimal places, as a
// whole count of its smallest part: with three places "1.5" is 1500 and
// "250" is 250000. It reads digits and never a float, as Money does, and
// takes a comma only where it groups digits. The number may be below zero;
// whether that is allowed is the caller's.
//
// The string is what is wrong with the cell, or empty. what is the thing
// the cell is, as "amount".
func Decimal(cell string, places int, what string) (int64, string) {
	number := strings.TrimSpace(cell)
	if !groupedLikeMoney(number) {
		return 0, fmt.Sprintf("%q: write the %s with a point for the decimals, like 1.5, and no comma.", cell, what)
	}
	number = strings.ReplaceAll(number, ",", "")
	negative := false
	if rest, found := strings.CutPrefix(number, "-"); found {
		negative, number = true, rest
	}
	whole, fraction, _ := strings.Cut(number, ".")
	notANumber := fmt.Sprintf("%q is not an %s. Type a number like 250 or 1.5.", cell, what)
	if whole == "" && fraction == "" || !allDigits(whole) || !allDigits(fraction) {
		return 0, notANumber
	}
	if len(fraction) > places {
		return 0, fmt.Sprintf("%q: an %s has at most %d decimal places.", cell, what, places)
	}
	// Eighteen digits fit an int64 with room to spare; more is not a quantity.
	digits := strings.TrimLeft(whole, "0") + fraction + strings.Repeat("0", places-len(fraction))
	if len(digits) > 18 {
		return 0, fmt.Sprintf("%q is too large.", cell)
	}
	var n int64
	for _, d := range digits {
		n = n*10 + int64(d-'0')
	}
	if negative {
		n = -n
	}
	return n, ""
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

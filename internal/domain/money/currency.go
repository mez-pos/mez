package money

import "fmt"

// defaultExponent is how many decimal places a currency has unless it is named
// below. Most of the world, and every currency this product has a user in
// today, has two.
const defaultExponent = 2

// exponents lists the currencies that do not have two decimal places, from
// ISO 4217. It is deliberately a short table of exceptions rather than a full
// currency list: an unknown code is allowed through with two decimals, but a
// yen must never be printed as 12.34 when it means 1234.
var exponents = map[string]int{
	// No minor unit at all.
	"BIF": 0, "CLP": 0, "DJF": 0, "GNF": 0, "ISK": 0, "JPY": 0, "KMF": 0,
	"KRW": 0, "PYG": 0, "RWF": 0, "UGX": 0, "UYI": 0, "VND": 0, "VUV": 0,
	"XAF": 0, "XOF": 0, "XPF": 0,

	// Three decimal places.
	"BHD": 3, "IQD": 3, "JOD": 3, "KWD": 3, "LYD": 3, "OMR": 3, "TND": 3,

	// Four.
	"CLF": 4, "UYW": 4,
}

// Exponent is how many decimal places separate a currency's major unit from
// its minor one: 2 for INR, so 100 paise make a rupee, and 0 for JPY.
//
// An unrecognised code returns the default of 2. That is a guess, and it is a
// safe one: it is the answer for the overwhelming majority of currencies, and
// the alternative is refusing to render an amount at all.
func Exponent(currency string) int {
	if e, ok := exponents[currency]; ok {
		return e
	}
	return defaultExponent
}

// DefaultExponent is the decimal places of any currency Exponents does not
// list.
const DefaultExponent = defaultExponent

// Exponents is every currency whose exponent is not DefaultExponent. It is a
// copy, so a caller cannot change how amounts are read. The TypeScript
// generator emits it, so the frontends read minor units exactly as the
// server does.
func Exponents() map[string]int {
	out := make(map[string]int, len(exponents))
	for code, e := range exponents {
		out[code] = e
	}
	return out
}

// validCurrency reports whether a currency code is well formed: exactly three
// uppercase ASCII letters.
//
// Lowercase is rejected rather than folded. Folding would be friendlier, but a
// silent transform in a money path is the kind of helpfulness that hides a bug
// in an import file for a month.
func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for i := range currency {
		if currency[i] < 'A' || currency[i] > 'Z' {
			return false
		}
	}
	return true
}

// checkCurrency returns a described error for a currency that is not well
// formed.
func checkCurrency(currency string) error {
	if !validCurrency(currency) {
		return fmt.Errorf("money: %q: %w", currency, ErrInvalidCurrency)
	}
	return nil
}

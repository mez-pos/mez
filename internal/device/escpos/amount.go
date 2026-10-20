package escpos

import (
	"strconv"
	"strings"

	"github.com/mez-pos/mez/internal/domain/money"
)

// symbols are the currency marks a receipt's total carries, in ASCII.
// Columns of amounts carry none: a receipt reads better without "Rs."
// repeated down the right-hand side.
var symbols = map[string]string{"INR": "Rs.", "USD": "$", "GBP": "GBP ", "EUR": "EUR "}

// amountText is an amount for paper: grouped the way its currency's
// customers read numbers (a lakh is 1,00,000), a minus for a refund, and
// the symbol only when asked. It never goes through a float.
func amountText(a money.Amount, symbol bool) string {
	exponent := money.Exponent(a.Currency)
	digits := strconv.FormatInt(a.Minor, 10)
	sign := ""
	if rest, found := strings.CutPrefix(digits, "-"); found {
		sign, digits = "-", rest
	}
	for len(digits) <= exponent {
		digits = "0" + digits
	}
	whole, fraction := digits[:len(digits)-exponent], digits[len(digits)-exponent:]
	out := group(whole, a.Currency == "INR")
	if exponent > 0 {
		out += "." + fraction
	}
	if symbol {
		mark, ok := symbols[a.Currency]
		if !ok {
			mark = a.Currency + " "
		}
		out = mark + out
	}
	return sign + out
}

// group puts commas in a whole number: every three digits, or the Indian
// way, the last three and then every two.
func group(whole string, indian bool) string {
	if len(whole) <= 3 {
		return whole
	}
	head, tail := whole[:len(whole)-3], whole[len(whole)-3:]
	step := 3
	if indian {
		step = 2
	}
	var parts []string
	for len(head) > step {
		parts = append([]string{head[len(head)-step:]}, parts...)
		head = head[:len(head)-step]
	}
	parts = append([]string{head}, parts...)
	return strings.Join(append(parts, tail), ",")
}

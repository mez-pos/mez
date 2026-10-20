package escpos

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/tax"
)

func TestPrintable(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Flat white", "Flat white"},
		{"Crème brûlée", "Creme brulee"},
		{"₹120", "Rs.120"},
		{"Mango – large", "Mango - large"},
		{"मसाला चाय", "????? ???"},
		{"a\x1Bb\nc", "a?b?c"}, // an escape in a name must never reach the printer as a command
		{"☕", "?"},
	}
	for _, tt := range tests {
		if got := Printable(tt.in); got != tt.want {
			t.Errorf("Printable(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  []string
	}{
		{"short", 10, []string{"short"}},
		{"two words here", 9, []string{"two words", "here"}},
		{"abcdefghijkl", 5, []string{"abcde", "fghij", "kl"}},
		{"  spaced   out  ", 20, []string{"spaced out"}},
		{"", 10, []string{""}},
	}
	for _, tt := range tests {
		if got := wrap(tt.text, tt.width); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("wrap(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
		}
	}
}

func TestColumns(t *testing.T) {
	got := columns("2 x Filter coffee with oat milk", "120.00", 24)
	want := []string{"2 x Filter coffee 120.00", "with oat milk"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("columns = %q, want %q", got, want)
	}
	for _, l := range got {
		if len(l) > 24 {
			t.Errorf("%q is over 24", l)
		}
	}
	// An indent stays on every line the left side wraps to.
	got = columns("  Tip, apart from the bill", "50.00", 24)
	if want := []string{"  Tip, apart from  50.00", "  the bill"}; !reflect.DeepEqual(got, want) {
		t.Errorf("indented columns = %q, want %q", got, want)
	}
	// A right side too wide to share goes on its own line.
	got = columns("Taxable", "12,34,56,78,901.00", 20)
	if len(got) != 2 || strings.TrimSpace(got[1]) != "12,34,56,78,901.00" || len(got[1]) != 20 {
		t.Errorf("wide right side = %q", got)
	}
}

func TestAmountText(t *testing.T) {
	tests := []struct {
		minor    int64
		currency string
		symbol   bool
		want     string
	}{
		{12000, "INR", false, "120.00"},
		{12000000, "INR", false, "1,20,000.00"},
		{123456789, "INR", true, "Rs.12,34,567.89"},
		{5, "INR", false, "0.05"},
		{-500, "INR", false, "-5.00"},
		{123456789, "USD", true, "$1,234,567.89"},
		{1234, "JPY", true, "JPY 1,234"},
		{99, "GBP", true, "GBP 0.99"},
	}
	for _, tt := range tests {
		if got := amountText(money.Amount{Minor: tt.minor, Currency: tt.currency}, tt.symbol); got != tt.want {
			t.Errorf("amountText(%d %s) = %q, want %q", tt.minor, tt.currency, got, tt.want)
		}
	}
}

func TestRateText(t *testing.T) {
	for rate, want := range map[tax.Rate]string{5000: "5%", 2500: "2.5%", 8875: "8.875%", 250: "0.25%", 0: "0%"} {
		if got := rateText(rate); got != want {
			t.Errorf("rateText(%d) = %q, want %q", rate, got, want)
		}
	}
}

package order

import (
	"testing"

	"github.com/mez-pos/mez/internal/domain/tax"
)

// A total read before the bill is made says "before tax" only where the bill
// will add some (ADR-386, GST-2, TILL-18).
func TestTaxesAddedOnTop(t *testing.T) {
	tests := []struct {
		name  string
		taxes Taxes
		want  bool
	}{
		{"GST on exclusive prices", gstCafe(tax.Exclusive), true},
		{"GST on inclusive prices", gstCafe(tax.Inclusive), false},
		{"VAT on inclusive prices", Taxes{Profile: tax.VAT{}, Venue: tax.VenueContext{Currency: "GBP", Pricing: tax.Inclusive}}, false},
		{"US sales tax, always added", Taxes{Profile: tax.USSalesTax{}, Venue: tax.VenueContext{Currency: "USD", Pricing: tax.Exclusive}}, true},
		{"an untaxed venue adds none", Taxes{}, false},
		{"an untaxed venue whatever its pricing says", Taxes{Venue: tax.VenueContext{Pricing: tax.Exclusive}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.taxes.AddedOnTop(); got != tc.want {
				t.Errorf("AddedOnTop() = %v, want %v", got, tc.want)
			}
		})
	}
}

package order

import (
	"context"
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/db/gen"
	"github.com/mez-pos/mez/internal/domain/order"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// TestACompositionVenueChargesNoTax (ADR-401): under the GST composition
// scheme the bill is the price, with no tax on it, and the invoice it issues
// names the seller by its GSTIN under the composition code, with no place of
// supply, though the venue's pricing and state settings say otherwise.
func TestACompositionVenueChargesNoTax(t *testing.T) {
	s, d, _ := newService(t)
	ctx := context.Background()
	if _, err := d.Writer().ExecContext(ctx, `UPDATE venues SET tax_profile = ? WHERE id = 1`, tax.CodeGSTComposition); err != nil {
		t.Fatal(err)
	}
	res := open(t, s)
	res = must(t)(s.AddLine(ctx, cashier(""), res.OrderID, coffee(1)))
	totals := res.State.Totals
	if !totals.Tax.IsZero() || totals.Total != totals.Subtotal {
		t.Fatalf("totals %+v: want no tax, and the total the subtotal", totals)
	}
	if res.State.Taxes == nil || res.State.Taxes.Profile != tax.CodeGSTComposition {
		t.Fatalf("the order carries taxes %+v, want the composition code", res.State.Taxes)
	}

	paid := must(t)(s.TenderPayment(ctx, cashier(""), res.OrderID, TenderPayment{Method: order.PaymentCard, Amount: totals.BalanceDue}))
	var issued *order.InvoiceIssued
	for _, e := range paid.Events {
		if p, ok := e.Payload.(order.InvoiceIssued); ok {
			issued = &p
		}
	}
	if issued == nil || issued.Prices == nil {
		t.Fatalf("no invoice issued: %+v", paid.Events)
	}
	seller := issued.Prices.Seller
	if seller.TaxProfile != tax.CodeGSTComposition || seller.Registration != "29ABCDE1234F1Z5" || seller.StateCode != "" {
		t.Errorf("seller %+v: want the composition code, the GSTIN, no place of supply", seller)
	}
	if !issued.Prices.Tax.IsZero() || issued.Prices.Total != totals.Total {
		t.Errorf("invoice tax %v, total %v; want none and %v", issued.Prices.Tax, issued.Prices.Total, totals.Total)
	}
	if n := len(issued.InvoiceNo); n == 0 || n > GSTMaxInvoiceNo {
		t.Errorf("invoice number %q is %d characters, want 1 to %d", issued.InvoiceNo, n, GSTMaxInvoiceNo)
	}
}

// TestTheCompositionSchemeNeedsAGSTIN: a bill of supply without one could
// not be issued, so the venue is refused, as a GST venue without one is.
// Its numbers are held to GST's 16 characters.
func TestTheCompositionSchemeNeedsAGSTIN(t *testing.T) {
	row := gen.Venue{ID: 1, Currency: "INR", Timezone: "Asia/Kolkata", FinancialYearStartMonth: 4, TaxProfile: tax.CodeGSTComposition}
	if _, err := venueFrom(row, map[string]string{}); !errors.Is(err, ErrVenueConfig) {
		t.Errorf("no GSTIN: %v, want ErrVenueConfig", err)
	}
	row.TaxRegistrationID = "29ABCDE1234F1Z5"
	v, err := venueFrom(row, map[string]string{SettingTaxStateCode: "29", SettingTaxPricing: "exclusive"})
	if err != nil {
		t.Fatalf("venueFrom: %v", err)
	}
	if v.Taxes.Profile != nil || v.TaxCode != tax.CodeGSTComposition {
		t.Errorf("taxes %+v under %q: want none computed, under the composition code", v.Taxes, v.TaxCode)
	}
	if v.Series.MaxLength != GSTMaxInvoiceNo || v.Credits.MaxLength != GSTMaxInvoiceNo {
		t.Errorf("series hold numbers to %d and %d, want %d", v.Series.MaxLength, v.Credits.MaxLength, GSTMaxInvoiceNo)
	}
	if v.Seller.Registration != row.TaxRegistrationID || v.Seller.StateCode != "" {
		t.Errorf("seller %+v", v.Seller)
	}
}

// TestABillIsNamedAsItWasTaxed (ADR-401): a ticket keeps the scheme it
// opened under. Paid after the venue moved between GST and the composition
// scheme, its invoice names the seller under the old one: GST's place of
// supply and tax, or the composition scheme's code and none.
func TestABillIsNamedAsItWasTaxed(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		taxed    bool
		state    string
	}{
		{"opened under GST, paid under the composition scheme", tax.CodeGST, tax.CodeGSTComposition, true, "29"},
		{"opened under the composition scheme, paid under GST", tax.CodeGSTComposition, tax.CodeGST, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, d, _ := newService(t)
			ctx := context.Background()
			move := func(code string) {
				t.Helper()
				if _, err := d.Writer().ExecContext(ctx, `UPDATE venues SET tax_profile = ? WHERE id = 1`, code); err != nil {
					t.Fatal(err)
				}
			}
			move(tt.from)
			res := open(t, s)
			res = must(t)(s.AddLine(ctx, cashier(""), res.OrderID, coffee(1)))
			move(tt.to)

			paid := must(t)(s.TenderPayment(ctx, cashier(""), res.OrderID, TenderPayment{Method: order.PaymentCard, Amount: res.State.Totals.BalanceDue}))
			var prices *order.InvoicePrices
			for _, e := range paid.Events {
				if p, ok := e.Payload.(order.InvoiceIssued); ok {
					prices = p.Prices
				}
			}
			if prices == nil {
				t.Fatal("no invoice issued")
			}
			seller := prices.Seller
			if seller.TaxProfile != tt.from || seller.StateCode != tt.state || seller.Registration != "29ABCDE1234F1Z5" {
				t.Errorf("seller %+v: want %s, place of supply %q, the GSTIN", seller, tt.from, tt.state)
			}
			if prices.Tax.IsZero() == tt.taxed {
				t.Errorf("tax %v: want taxed %v", prices.Tax, tt.taxed)
			}
		})
	}
}

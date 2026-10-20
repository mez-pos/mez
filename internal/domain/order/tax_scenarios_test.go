package order

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
	"github.com/mez-pos/mez/internal/domain/payment"
	"github.com/mez-pos/mez/internal/domain/tax"
)

// gstCafe is a Bengaluru cafe (Karnataka, state 29).
func gstCafe(p tax.Pricing) Taxes {
	return Taxes{Profile: tax.GST{}, Venue: tax.VenueContext{
		Currency: "INR", Pricing: p, GSTIN: "29ABCDE1234F1Z5", StateCode: "29",
	}}
}

// dish is a restaurant line at 5% GST under SAC 996331.
func dish(name string, qty, unitMinor int64) LineAdded {
	return LineAdded{Name: name, Qty: qty, UnitPrice: inr(unitMinor), Station: "kitchen", TaxRate: 5000, HSN: "996331"}
}

func projectTaxed(t *testing.T, taxes Taxes, payloads ...Payload) State {
	t.Helper()
	s, err := Project(stream(payloads...), taxes)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	checkDues(t, s)
	return s
}

// taxedFigures is a taxed bill's money in minor units.
type taxedFigures struct {
	subtotal, taxable, tax, total int64
	paid, refunded, balance       int64
}

func checkTaxed(t *testing.T, got Totals, want taxedFigures) {
	t.Helper()
	have := taxedFigures{
		subtotal: got.Subtotal.Minor, taxable: got.Taxable.Minor, tax: got.Tax.Minor, total: got.Total.Minor,
		paid: got.Paid.Minor, refunded: got.Refunded.Minor, balance: got.BalanceDue.Minor,
	}
	if have != want {
		t.Errorf("totals\n got: %+v\nwant: %+v", have, want)
	}
	if got.Taxable.Minor+got.Tax.Minor != got.Total.Minor {
		t.Errorf("taxable %d + tax %d != total %d", got.Taxable.Minor, got.Tax.Minor, got.Total.Minor)
	}
}

func lineTaxes(s State) []int64 {
	out := make([]int64, len(s.Lines))
	for i, l := range s.Lines {
		out[i] = l.Tax.Minor
	}
	return out
}

func TestGSTCafeExclusive(t *testing.T) {
	// The worked example from the tax package, as a whole order: 2 x 180.00,
	// 250.00 and 99.00 at 5%. CGST per line 900, 625 and 247 once the order
	// reconciles 1772.5 down to 1772; SGST the same. Tax 3544 on 70900.
	taxes := gstCafe(tax.Exclusive)
	payloads := []Payload{
		opened(),
		dish("Cappuccino", 2, 18000), // 2
		dish("Paneer sandwich", 1, 25000),
		dish("Choc chip cookie", 1, 9900),
		pay(PaymentUPI, 74444), // 5
	}
	s := projectTaxed(t, taxes, payloads...)
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 70900, taxable: 70900, tax: 3544, total: 74444, paid: 74444})
	if got := lineTaxes(s); !reflect.DeepEqual(got, []int64{1800, 1250, 494}) {
		t.Errorf("line taxes %v, want [1800 1250 494]", got)
	}

	// The snapshot the invoice carries is the projection's TaxDetail.
	snapshot, err := json.Marshal(s.TaxDetail)
	if err != nil {
		t.Fatal(err)
	}
	s = projectTaxed(t, taxes, append(payloads,
		InvoiceIssued{InvoiceNo: "NF/26-27/000001", Series: "NF", TaxSnapshot: snapshot},
		closed,
	)...)
	var back tax.TaxResult
	if err := json.Unmarshal(snapshot, &back); err != nil || !reflect.DeepEqual(back, s.TaxDetail) {
		t.Errorf("the invoice snapshot does not match the closed bill's tax: %v", err)
	}
	if s.Status != StatusClosed || len(s.TaxDetail.Components) != 2 {
		t.Errorf("status %q, components %+v", s.Status, s.TaxDetail.Components)
	}
}

func TestGSTInclusiveOrderDiscountIsSpreadBeforeTax(t *testing.T) {
	// 300.00 and 100.00 inclusive, 50.00 off the order. The discount is
	// spread in proportion: 37.50 and 12.50, leaving 262.50 and 87.50.
	//
	// 26250 / 1.05 = 25000 exactly, CGST 625. 8750 / 1.05 = 8333.33, so
	// 8333, CGST 208.325, so 208. The order, 33333 x 2.5% = 833.325, is 833
	// = 625 + 208. The second line's taxable value absorbs the rounding:
	// 8750 - 416 = 8334. Total 350.00, exactly the discounted menu price.
	s := projectTaxed(t, gstCafe(tax.Inclusive),
		opened(),
		dish("Thali", 1, 30000), // 2
		dish("Lassi", 1, 10000), // 3
		off(ScopeOrder, 0, 5000),
	)
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 35000, taxable: 33334, tax: 1666, total: 35000, balance: 35000})
	if s.Lines[0].OrderDiscount != inr(3750) || s.Lines[1].OrderDiscount != inr(1250) {
		t.Errorf("order discount spread %+v / %+v, want 3750 / 1250", s.Lines[0].OrderDiscount, s.Lines[1].OrderDiscount)
	}
	if got := lineTaxes(s); !reflect.DeepEqual(got, []int64{1250, 416}) {
		t.Errorf("line taxes %v, want [1250 416]", got)
	}
}

func TestGSTVoidAfterPaymentRefundsTheTaxToo(t *testing.T) {
	// 200.00 and 100.00 exclusive at 5%: tax 10.00 and 5.00, total 315.00.
	// Voiding the 100.00 line takes its 5.00 of tax with it, so the guest is
	// owed 105.00, and that is what is refunded.
	s := projectTaxed(t, gstCafe(tax.Exclusive),
		opened(),
		dish("Biryani", 1, 20000), // 2
		dish("Raita", 1, 10000),   // 3
		pay(PaymentCard, 31500),   // 4
		void(3),
		RefundIssued{Amount: inr(10500), AgainstPaymentID: 4, Reason: "raita never came"},
		closed,
	)
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 20000, taxable: 20000, tax: 1000, total: 21000, paid: 31500, refunded: 10500})
	if s.Lines[1].Tax != inr(0) {
		t.Errorf("a voided line still carries tax %+v", s.Lines[1].Tax)
	}
}

func TestEachLineKeepsTheTaxRateItWasAddedAt(t *testing.T) {
	// The owner moves the item from 5% to 18% at 7pm. The line added before
	// keeps 5%; the one after carries 18%. The projection never looks the
	// rate up.
	before := dish("Mocktail", 1, 10000)
	after := before
	after.TaxRate = tax.RateFromBP(1800)
	s := projectTaxed(t, gstCafe(tax.Exclusive), opened(), before, after, LineComped{LineID: 3, ReasonCode: "x", AuthorisedBy: 3}, dish("Water", 1, 2000))
	// Line 3 was comped: no value, no tax. Lines 2 and 5 are 500 and 100.
	if got := lineTaxes(s); !reflect.DeepEqual(got, []int64{500, 0, 100}) {
		t.Errorf("line taxes %v, want [500 0 100]", got)
	}

	s = projectTaxed(t, gstCafe(tax.Exclusive), opened(), before, after)
	if got := lineTaxes(s); !reflect.DeepEqual(got, []int64{500, 1800}) {
		t.Errorf("line taxes %v, want [500 1800]: each at its own snapshot", got)
	}
	if len(s.TaxDetail.Components) != 4 {
		t.Errorf("two rates should give four summary rows (CGST and SGST at each), got %+v", s.TaxDetail.Components)
	}
}

func TestVATCafeInclusive(t *testing.T) {
	// The London worked example: 3.40 and 2.95 at 20%, 3.50 zero-rated.
	// VAT 57 + 49 = 106, net 879, total 9.85 as priced.
	gbp := func(m int64) money.Amount { return money.Amount{Minor: m, Currency: "GBP"} }
	taxes := Taxes{Profile: tax.VAT{}, Venue: tax.VenueContext{Currency: "GBP", Pricing: tax.Inclusive, VATNumber: "GB123456789"}}
	s := projectTaxed(t, taxes,
		OrderOpened{Channel: ChannelCounter, Currency: "GBP", Covers: 1},
		LineAdded{Name: "Flat white", Qty: 1, UnitPrice: gbp(340), TaxRate: 20000},
		LineAdded{Name: "Croissant", Qty: 1, UnitPrice: gbp(295), TaxRate: 20000},
		LineAdded{Name: "Cold sandwich, takeaway", Qty: 1, UnitPrice: gbp(350)},
		PaymentTendered{Method: PaymentCard, Amount: gbp(985), Tip: gbp(0)},
		closed,
	)
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 985, taxable: 879, tax: 106, total: 985, paid: 985})
}

func TestUSSalesTaxNewYork(t *testing.T) {
	// The NYC worked example: 4.50 and 6.25 taxed at 8.875% in three parts,
	// a 5.00 loaf exempt. Tax 95 cents; total 16.70.
	usd := func(m int64) money.Amount { return money.Amount{Minor: m, Currency: "USD"} }
	taxes := Taxes{Profile: tax.USSalesTax{}, Venue: tax.VenueContext{Currency: "USD", Pricing: tax.Exclusive,
		Jurisdiction: []tax.Component{{Name: "NY state", Rate: 4000}, {Name: "New York City", Rate: 4500}, {Name: "MCTD", Rate: 375}}}}
	s := projectTaxed(t, taxes,
		OrderOpened{Channel: ChannelCounter, Currency: "USD", Covers: 1},
		LineAdded{Name: "Bagel", Qty: 1, UnitPrice: usd(450)},
		LineAdded{Name: "Coffee", Qty: 1, UnitPrice: usd(625)},
		LineAdded{Name: "Whole loaf", Qty: 1, UnitPrice: usd(500), TaxExempt: true},
		PaymentTendered{Method: PaymentCard, Amount: usd(1670), Tip: usd(300)},
		closed,
	)
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 1575, taxable: 1575, tax: 95, total: 1670, paid: 1670})
	if s.Totals.Tips != usd(300) {
		t.Errorf("tips %+v: tax must never touch the tip", s.Totals.Tips)
	}
}

func TestTaxChangesWithTheBill(t *testing.T) {
	// Tax is recomputed after every event: a quantity change moves it, and
	// the balance follows.
	s := projectTaxed(t, gstCafe(tax.Exclusive), opened(), dish("Chai", 1, 3000), LineQtyChanged{LineID: 2, From: 1, To: 3})
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 9000, taxable: 9000, tax: 450, total: 9450, balance: 9450})
}

func TestTaxedProjectionRefuses(t *testing.T) {
	tests := []struct {
		name     string
		taxes    Taxes
		payloads []Payload
		wantErr  error
	}{
		{"a GST line with no HSN", gstCafe(tax.Exclusive),
			[]Payload{opened(), dish("Chai", 1, 3000), LineAdded{Name: "Mystery", Qty: 1, UnitPrice: inr(100), TaxRate: 5000}},
			tax.ErrMissingHSN},
		{"a venue taxed in another currency", Taxes{Profile: tax.VAT{}, Venue: tax.VenueContext{Currency: "GBP", Pricing: tax.Inclusive, VATNumber: "GB1"}},
			[]Payload{opened()}, ErrTaxConfig},
		{"a tax rate over 100%", untaxed,
			[]Payload{opened(), LineAdded{Name: "x", Qty: 1, UnitPrice: inr(100), TaxRate: tax.RateScale + 1}}, ErrInvalidPayload},
		{"US sales tax on inclusive prices", Taxes{Profile: tax.USSalesTax{}, Venue: tax.VenueContext{Currency: "INR", Pricing: tax.Inclusive,
			Jurisdiction: []tax.Component{{Name: "x", Rate: 1000}}}},
			[]Payload{opened()}, tax.ErrPricingNotSupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := stream(tt.payloads...)
			_, err := Project(events, tt.taxes)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			var pe *ProjectionError
			if !errors.As(err, &pe) || pe.Seq != int64(len(events)) {
				t.Errorf("failed at %v, want the last event, seq %d", err, len(events))
			}
		})
	}
}

func TestUntaxedVenueKeepsTaxAtZero(t *testing.T) {
	s := mustProject(t, opened(), dish("Chai", 2, 3000))
	checkTaxed(t, s.Totals, taxedFigures{subtotal: 6000, taxable: 6000, tax: 0, total: 6000, balance: 6000})
	if !reflect.DeepEqual(s.TaxDetail, tax.TaxResult{}) {
		t.Errorf("an untaxed venue has a tax snapshot: %+v", s.TaxDetail)
	}
}

func TestASplitByItemSettlesATaxedBill(t *testing.T) {
	// Three friends at a GST cafe pricing exclusive. Asha had the thali,
	// Ben the dosa, and all three shared a plate of pakoras; 10% off the
	// whole order. The split is built from each line's Due, tendered back
	// into the order, and must close it at exactly zero.
	taxes := gstCafe(tax.Exclusive)
	bill := []Payload{
		opened(),
		dish("Thali", 1, 32000),       // 2
		dish("Masala dosa", 1, 18000), // 3
		dish("Pakoras", 1, 15001),     // 4
		pct(ScopeOrder, 0, 1000),
	}
	s := projectTaxed(t, taxes, bill...)

	payers := map[int64][]int{2: {0}, 3: {1}, 4: {0, 1, 2}}
	items := make([]payment.Item, len(s.Lines))
	for i, l := range s.Lines {
		items[i] = payment.Item{LineID: l.ID, Amount: l.Due, Payers: payers[l.ID]}
	}
	split, err := payment.ByItem(s.Totals.Total, 3, items)
	if err != nil {
		t.Fatalf("ByItem: %v", err)
	}
	split, err = split.WithTips([]money.Amount{inr(3000), inr(0), inr(1500)})
	if err != nil {
		t.Fatal(err)
	}

	methods := []PaymentMethod{PaymentCard, PaymentUPI, PaymentCash}
	for i, sh := range split.Shares {
		bill = append(bill, PaymentTendered{Method: methods[i], Amount: sh.Amount, Tip: sh.Tip, TenderedBy: 11})
	}
	s = projectTaxed(t, taxes, append(bill, closed)...)
	if s.Status != StatusClosed || !s.Totals.BalanceDue.IsZero() {
		t.Fatalf("status %q, balance %d: the split did not settle the bill", s.Status, s.Totals.BalanceDue.Minor)
	}
	if s.Totals.Tips != inr(4500) || s.Totals.Paid != s.Totals.Total {
		t.Errorf("tips %d, paid %d, total %d: tips must stay out of the bill", s.Totals.Tips.Minor, s.Totals.Paid.Minor, s.Totals.Total.Minor)
	}
}

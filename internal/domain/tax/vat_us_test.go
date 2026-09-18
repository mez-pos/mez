package tax

import (
	"errors"
	"testing"

	"github.com/byjoellobo/nofiltr-pos/internal/domain/money"
)

func gbp(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "GBP"} }
func usd(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "USD"} }

func TestVATInclusiveUKCafe(t *testing.T) {
	// A London cafe, prices include 20% VAT.
	//
	// Flat white 3.40: 340 / 1.2 = 283.33, so 283 net; VAT 283 x 20% = 56.6,
	// so 57; 283 + 57 = 340. Croissant 2.95: 295 / 1.2 = 245.83, so 246;
	// VAT 49.2, so 49; 246 + 49 = 295. The order, 529 x 20% = 105.8, rounds
	// to 106 = 57 + 49: no reconciliation. A cold takeaway sandwich at 3.50
	// is zero-rated.
	ctx := VenueContext{Currency: "GBP", Pricing: Inclusive, VATNumber: "GB123456789"}
	lines := []Line{
		{ID: 1, Amount: gbp(340), Rate: 20000},
		{ID: 2, Amount: gbp(295), Rate: 20000},
		{ID: 3, Amount: gbp(350), Rate: 0},
	}
	res, err := VAT{}.Compute(lines, ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{{283, []int64{57}, 340}, {246, []int64{49}, 295}, {350, []int64{}, 350}})
	checkTotals(t, res, 879, 106, 985)
	if len(res.Components) != 1 || res.Components[0].Taxable != gbp(529) || res.Components[0].Amount != gbp(106) {
		t.Errorf("summary = %+v", res.Components)
	}
	// The VAT fraction of the taxed lines: 635 / 6 = 105.83, so 106. The
	// line-by-line figures agree with it.
}

func TestVATExclusiveRoundsHalfToEven(t *testing.T) {
	// A German takeaway at 7%, priced exclusive: 10.50 x 7% = 0.735, which
	// is 73.5 cents and rounds half to even to 74. 10.30 x 7% = 72.1: 72.
	ctx := VenueContext{Currency: "EUR", Pricing: Exclusive, VATNumber: "DE123456789"}
	eur := func(m int64) money.Amount { return money.Amount{Minor: m, Currency: "EUR"} }
	res, err := VAT{}.Compute([]Line{{ID: 1, Amount: eur(1050), Rate: 7000}, {ID: 2, Amount: eur(1030), Rate: 7000}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Per line 74 + 72 = 146; the order, 2080 x 7% = 145.6, is 146. Agreed.
	checkLines(t, res, []lineWant{{1050, []int64{74}, 1124}, {1030, []int64{72}, 1102}})
	checkTotals(t, res, 2080, 146, 2226)
}

func TestVATRefuses(t *testing.T) {
	if _, err := (VAT{}).Compute(nil, VenueContext{Currency: "GBP", Pricing: Inclusive}); !errors.Is(err, ErrInvalidContext) {
		t.Errorf("no VAT number: err = %v", err)
	}
	ctx := VenueContext{Currency: "GBP", Pricing: Inclusive, VATNumber: "GB1"}
	if _, err := (VAT{}).Compute([]Line{{ID: 1, Amount: gbp(1), Rate: -1}}, ctx); !errors.Is(err, ErrInvalidRate) {
		t.Errorf("negative rate: err = %v", err)
	}
}

// nyc is New York City: 4% state, 4.5% city, 0.375% Metropolitan Commuter
// Transportation District, 8.875% combined.
var nyc = VenueContext{Currency: "USD", Pricing: Exclusive, Jurisdiction: []Component{
	{Name: "NY state", Rate: 4000},
	{Name: "New York City", Rate: 4500},
	{Name: "MCTD", Rate: 375},
}}

func TestUSSalesTaxNewYorkCity(t *testing.T) {
	// A bagel at 4.50 and a coffee at 6.25, taxed; a whole loaf at 5.00,
	// exempt. Line rates are ignored: the place sets the rate.
	//
	// State 4%: 18.0 and 25.0, so 18 and 25; the order, 1075 x 4% = 43. City
	// 4.5%: 20.25 and 28.125, so 20 and 28; the order, 48.375, is 48. MCTD
	// 0.375%: 1.6875 and 2.34375, so 2 and 2; the order, 4.03125, is 4. Tax
	// 95 cents, which is also 8.875% of 10.75 (95.4) rounded.
	lines := []Line{
		{ID: 1, Amount: usd(450), Rate: 99999},
		{ID: 2, Amount: usd(625)},
		{ID: 3, Amount: usd(500), Exempt: true},
	}
	res, err := USSalesTax{}.Compute(lines, nyc)
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{
		{450, []int64{18, 20, 2}, 490},
		{625, []int64{25, 28, 2}, 680},
		{500, []int64{}, 500},
	})
	checkTotals(t, res, 1575, 95, 1670)
	for i, want := range []int64{43, 48, 4} {
		if c := res.Components[i]; c.Amount != usd(want) || c.Taxable != usd(1075) {
			t.Errorf("%s: %+v, want %d on 1075 taxable", c.Name, c, want)
		}
	}
	if res.Lines[0].Rate != 8875 || res.Lines[0].Rate.String() != "8.875%" {
		t.Errorf("combined rate %d (%s), want 8.875%%", res.Lines[0].Rate, res.Lines[0].Rate)
	}
}

func TestUSSalesTaxRefuses(t *testing.T) {
	with := func(f func(*VenueContext)) VenueContext {
		c := nyc
		c.Jurisdiction = append([]Component(nil), nyc.Jurisdiction...)
		f(&c)
		return c
	}
	tests := []struct {
		name    string
		ctx     VenueContext
		wantErr error
	}{
		{"inclusive pricing", with(func(c *VenueContext) { c.Pricing = Inclusive }), ErrPricingNotSupported},
		{"no jurisdiction", with(func(c *VenueContext) { c.Jurisdiction = nil }), ErrInvalidContext},
		{"an unnamed rate", with(func(c *VenueContext) { c.Jurisdiction[0].Name = " " }), ErrInvalidContext},
		{"a rate over 100%", with(func(c *VenueContext) { c.Jurisdiction[1].Rate = RateScale + 1 }), ErrInvalidRate},
		{"rates adding past 100%", with(func(c *VenueContext) {
			c.Jurisdiction = []Component{{Name: "a", Rate: 60000}, {Name: "b", Rate: 60000}}
		}), ErrInvalidRate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (USSalesTax{}).Compute([]Line{{ID: 1, Amount: usd(100)}}, tt.ctx); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

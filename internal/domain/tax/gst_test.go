package tax

import (
	"errors"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

// bengaluru is a cafe in Karnataka (state 29) pricing exclusive of tax.
func bengaluru(p Pricing) VenueContext {
	return VenueContext{Currency: "INR", Pricing: p, GSTIN: "29ABCDE1234F1Z5", StateCode: "29"}
}

func restaurant(id, minor int64) Line {
	return Line{ID: id, Name: "item", HSN: "996331", Amount: inr(minor), Rate: 5000} // 5%, restaurant service
}

// lineWant is one line's expected figures in paise: taxable, then each
// component in order, then total.
type lineWant struct {
	taxable    int64
	components []int64
	total      int64
}

func checkLines(t *testing.T, res TaxResult, want []lineWant) {
	t.Helper()
	if len(res.Lines) != len(want) {
		t.Fatalf("%d lines, want %d", len(res.Lines), len(want))
	}
	for i, w := range want {
		l := res.Lines[i]
		got := make([]int64, len(l.Components))
		for j, c := range l.Components {
			got[j] = c.Amount.Minor
		}
		if l.Taxable.Minor != w.taxable || l.Total.Minor != w.total || !equal(got, w.components) {
			t.Errorf("line %d: taxable %d, components %v, total %d; want %d, %v, %d",
				l.LineID, l.Taxable.Minor, got, l.Total.Minor, w.taxable, w.components, w.total)
		}
	}
}

func equal(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func checkTotals(t *testing.T, res TaxResult, taxable, tax, total int64) {
	t.Helper()
	if res.Taxable.Minor != taxable || res.Tax.Minor != tax || res.Total.Minor != total {
		t.Errorf("totals: taxable %d, tax %d, total %d; want %d, %d, %d",
			res.Taxable.Minor, res.Tax.Minor, res.Total.Minor, taxable, tax, total)
	}
}

func TestGSTExclusiveIntraStateReconciles(t *testing.T) {
	// Two cappuccinos at 180.00, a sandwich at 250.00, a cookie at 99.00,
	// all at 5%: CGST 2.5% and SGST 2.5%.
	//
	// Per line, CGST is 36000 x 2.5% = 900, 25000 x 2.5% = 625, and
	// 9900 x 2.5% = 247.5, which rounds half to even to 248. That sums to
	// 1773. On the order, 70900 x 2.5% = 1772.5, which rounds to 1772. The
	// lines are a paisa over, so the line rounded furthest up, the cookie,
	// gives it back: 247. SGST is the same, line for line.
	res, err := GST{}.Compute([]Line{restaurant(1, 36000), restaurant(2, 25000), restaurant(3, 9900)}, bengaluru(Exclusive))
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{
		{36000, []int64{900, 900}, 37800},
		{25000, []int64{625, 625}, 26250},
		{9900, []int64{247, 247}, 10394},
	})
	checkTotals(t, res, 70900, 3544, 74444)
	if len(res.Components) != 2 || res.Components[0].Name != "CGST" || res.Components[1].Name != "SGST" ||
		res.Components[0].Amount != inr(1772) || res.Components[0].Rate != 2500 || res.Components[0].Taxable != inr(70900) {
		t.Errorf("summary = %+v", res.Components)
	}
}

func TestGSTInterStateIsIGST(t *testing.T) {
	// Outdoor catering (SAC 996334) at 18% for a client in Maharashtra (27).
	// 100000 x 18% = 18000; 33333 x 18% = 5999.94, rounds to 6000. The
	// order, 133333 x 18% = 23999.94, rounds to 24000: no reconciliation.
	ctx := bengaluru(Exclusive)
	ctx.PlaceOfSupply = "27"
	lines := []Line{
		{ID: 1, HSN: "996334", Amount: inr(100000), Rate: RateFromBP(1800)},
		{ID: 2, HSN: "996334", Amount: inr(33333), Rate: RateFromBP(1800)},
	}
	res, err := GST{}.Compute(lines, ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{{100000, []int64{18000}, 118000}, {33333, []int64{6000}, 39333}})
	checkTotals(t, res, 133333, 24000, 157333)
	if len(res.Components) != 1 || res.Components[0].Name != "IGST" || res.Components[0].Rate != 18000 {
		t.Errorf("summary = %+v", res.Components)
	}
}

func TestGSTInclusiveBacksOutToThePaisa(t *testing.T) {
	tests := []struct {
		name  string
		price int64
		want  lineWant
	}{
		// 10000 / 1.05 = 9523.81, so 9524 taxable. CGST 9524 x 2.5% =
		// 238.1, so 238, and SGST 238. 9524 + 476 = 10000 exactly.
		{"100.00 at 5%", 10000, lineWant{9524, []int64{238, 238}, 10000}},
		// 9900 / 1.05 = 9428.57, so 9429. CGST 9429 x 2.5% = 235.725, so
		// 236, and SGST 236. 9429 + 472 = 9901, a paisa over the menu
		// price, so the taxable value absorbs it: 9428.
		{"99.00 at 5%, where the taxable value absorbs a paisa", 9900, lineWant{9428, []int64{236, 236}, 9900}},
		// 1 paisa: 1 / 1.05 rounds to 1, tax on 1 paisa rounds to 0.
		{"one paisa", 1, lineWant{1, []int64{0, 0}, 1}},
		{"zero", 0, lineWant{0, []int64{0, 0}, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := GST{}.Compute([]Line{restaurant(1, tt.price)}, bengaluru(Inclusive))
			if err != nil {
				t.Fatal(err)
			}
			checkLines(t, res, []lineWant{tt.want})
			if res.Total.Minor != tt.price || res.Gross.Minor != tt.price {
				t.Errorf("inclusive total %d, gross %d, want the price %d", res.Total.Minor, res.Gross.Minor, tt.price)
			}
		})
	}
}

func TestGSTInclusiveReconcilesAcrossLines(t *testing.T) {
	// Two lines at 99.00 inclusive. Each alone is 9429 taxable and 236 CGST.
	// Together the taxable base is 18858, and 18858 x 2.5% = 471.45, so the
	// order's CGST is 471, not 472. The first line gives back a paisa of
	// CGST and of SGST, and its taxable value takes both: 9430.
	res, err := GST{}.Compute([]Line{restaurant(1, 9900), restaurant(2, 9900)}, bengaluru(Inclusive))
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{
		{9430, []int64{235, 235}, 9900},
		{9428, []int64{236, 236}, 9900},
	})
	checkTotals(t, res, 18858, 942, 19800)
}

func TestGSTUnionTerritoryChargesUTGST(t *testing.T) {
	ctx := VenueContext{Currency: "INR", Pricing: Exclusive, GSTIN: "04ABCDE1234F1Z5", StateCode: "04"} // Chandigarh
	res, err := GST{}.Compute([]Line{restaurant(1, 10000)}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c := res.Lines[0].Components; len(c) != 2 || c[0].Name != "CGST" || c[1].Name != "UTGST" || c[1].Amount != inr(250) {
		t.Errorf("components = %+v, want CGST and UTGST of 250", c)
	}

	delhi := VenueContext{Currency: "INR", Pricing: Exclusive, GSTIN: "07ABCDE1234F1Z5", StateCode: "07"}
	res, err = GST{}.Compute([]Line{restaurant(1, 10000)}, delhi)
	if err != nil {
		t.Fatal(err)
	}
	if res.Lines[0].Components[1].Name != "SGST" {
		t.Errorf("Delhi has a legislature and charges SGST, got %q", res.Lines[0].Components[1].Name)
	}
}

func TestGSTNilRatedAndExemptLines(t *testing.T) {
	lines := []Line{
		restaurant(1, 10000),
		{ID: 2, HSN: "2201", Amount: inr(2000), Rate: 0},                  // nil-rated
		{ID: 3, HSN: "0401", Amount: inr(3000), Rate: 5000, Exempt: true}, // exempt whatever the rate
	}
	res, err := GST{}.Compute(lines, bengaluru(Exclusive))
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{
		{10000, []int64{250, 250}, 10500},
		{2000, []int64{}, 2000},
		{3000, []int64{}, 3000},
	})
	checkTotals(t, res, 15000, 500, 15500)
	if res.Components[0].Taxable != inr(10000) {
		t.Errorf("CGST taxable %+v counts lines CGST does not apply to", res.Components[0].Taxable)
	}
}

func TestGSTRefuses(t *testing.T) {
	good := bengaluru(Exclusive)
	with := func(f func(*VenueContext)) VenueContext { c := good; f(&c); return c }

	tests := []struct {
		name    string
		lines   []Line
		ctx     VenueContext
		wantErr error
	}{
		{"no HSN", []Line{{ID: 1, Amount: inr(100), Rate: 5000}}, good, ErrMissingHSN},
		{"a five-digit HSN", []Line{{ID: 1, HSN: "99633", Amount: inr(100), Rate: 5000}}, good, ErrMissingHSN},
		{"letters in the HSN", []Line{{ID: 1, HSN: "99633A", Amount: inr(100), Rate: 5000}}, good, ErrMissingHSN},
		{"a rate that cannot split evenly", []Line{{ID: 1, HSN: "996331", Amount: inr(100), Rate: 5001}}, good, ErrInvalidRate},
		{"a rate over 100%", []Line{{ID: 1, HSN: "996331", Amount: inr(100), Rate: 100002}}, good, ErrInvalidRate},
		{"a negative rate", []Line{{ID: 1, HSN: "996331", Amount: inr(100), Rate: -2}}, good, ErrInvalidRate},
		{"a negative amount", []Line{restaurant(1, -1)}, good, ErrInvalidLine},
		{"an amount in dollars", []Line{{ID: 1, HSN: "996331", Amount: money.Amount{Minor: 1, Currency: "USD"}, Rate: 5000}}, good, ErrInvalidLine},
		{"no GSTIN", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.GSTIN = "" }), ErrInvalidContext},
		{"a lowercase GSTIN", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.GSTIN = "29abcde1234f1z5" }), ErrInvalidContext},
		{"a GSTIN from another state", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.GSTIN = "27ABCDE1234F1Z5" }), ErrInvalidContext},
		{"no state code", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.StateCode = "" }), ErrInvalidContext},
		{"state 39", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.StateCode = "39" }), ErrInvalidContext},
		{"a bad place of supply", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.PlaceOfSupply = "KA" }), ErrInvalidContext},
		{"no currency", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.Currency = "" }), ErrInvalidContext},
		{"no pricing", []Line{restaurant(1, 100)}, with(func(c *VenueContext) { c.Pricing = "" }), ErrInvalidContext},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := (GST{}).Compute(tt.lines, tt.ctx); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestGSTOddRateIsFineInterState(t *testing.T) {
	ctx := bengaluru(Exclusive)
	ctx.PlaceOfSupply = "97"
	if _, err := (GST{}).Compute([]Line{{ID: 1, HSN: "996331", Amount: inr(100), Rate: 5001}}, ctx); err != nil {
		t.Errorf("IGST does not split, so any rate works: %v", err)
	}
}

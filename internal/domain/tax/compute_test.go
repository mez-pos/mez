package tax

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// TestEngineInvariants is the engine's contract as a property, over random
// GST bills in both pricing modes, intra- and inter-state. GST exercises the
// most of the engine every profile shares: several rates, split components,
// exempt and nil-rated lines. For every bill:
//
//  1. every line: taxable + tax = total; inclusive: total = the price, and
//     exclusive: taxable = the price;
//  2. the rate-wise summary is the sum of the lines;
//  3. exclusive: each component's total is exactly its taxable × rate,
//     rounded once, half to even;
//  4. each line's component is within one minor unit of that line's own
//     rounding (reconciliation never moves a line further);
//  5. CGST equals SGST on every line;
//  6. the order totals are the sums of the lines;
//  7. the same bill always gives the same result.
func TestEngineInvariants(t *testing.T) {
	gstRates := []Rate{0, 250, 3000, 5000, 12000, 18000, 28000, 40000}
	for seed := range uint64(500) {
		rng := rand.New(rand.NewPCG(seed, 7))
		pricing := []Pricing{Inclusive, Exclusive}[rng.IntN(2)]
		ctx := bengaluru(pricing)
		if rng.IntN(3) == 0 {
			ctx.PlaceOfSupply = "27"
		}
		lines := make([]Line, 1+rng.IntN(12))
		for i := range lines {
			lines[i] = Line{
				ID:     int64(i + 1),
				HSN:    "996331",
				Amount: inr(rng.Int64N(500000)),
				Rate:   gstRates[rng.IntN(len(gstRates))],
				Exempt: rng.IntN(10) == 0,
			}
		}
		res, err := GST{}.Compute(lines, ctx)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		checkInvariants(t, seed, lines, ctx, res)

		again, err := GST{}.Compute(lines, ctx)
		if err != nil || !reflect.DeepEqual(res, again) {
			t.Fatalf("seed %d: the same bill gave a different result", seed)
		}
	}
}

func checkInvariants(t *testing.T, seed uint64, lines []Line, ctx VenueContext, res TaxResult) {
	t.Helper()
	sums := map[Component]int64{}
	var gross, taxable, tax, total int64
	for i, lt := range res.Lines {
		price := lines[i].Amount.Minor
		if lt.Taxable.Minor+lt.Tax.Minor != lt.Total.Minor {
			t.Fatalf("seed %d line %d: %d + %d != %d", seed, lt.LineID, lt.Taxable.Minor, lt.Tax.Minor, lt.Total.Minor)
		}
		if ctx.Pricing == Inclusive && lt.Total.Minor != price {
			t.Fatalf("seed %d line %d: inclusive total %d, price %d", seed, lt.LineID, lt.Total.Minor, price)
		}
		if ctx.Pricing == Exclusive && lt.Taxable.Minor != price {
			t.Fatalf("seed %d line %d: exclusive taxable %d, price %d", seed, lt.LineID, lt.Taxable.Minor, price)
		}
		base := lt.Taxable
		if ctx.Pricing == Inclusive && lt.Rate > 0 {
			base, _ = lines[i].Amount.Scale(RateScale, RateScale+int64(lt.Rate))
		}
		var lineTax int64
		for j, c := range lt.Components {
			own, _ := base.Scale(int64(c.Rate), RateScale)
			if d := c.Amount.Minor - own.Minor; d < -1 || d > 1 {
				t.Fatalf("seed %d line %d: %s moved %d from its own rounding", seed, lt.LineID, c.Name, d)
			}
			if c.Name == "SGST" && c.Amount != lt.Components[j-1].Amount {
				t.Fatalf("seed %d line %d: CGST %d, SGST %d", seed, lt.LineID, lt.Components[j-1].Amount.Minor, c.Amount.Minor)
			}
			sums[Component{Name: c.Name, Rate: c.Rate}] += c.Amount.Minor
			lineTax += c.Amount.Minor
		}
		if lineTax != lt.Tax.Minor {
			t.Fatalf("seed %d line %d: components %d, tax %d", seed, lt.LineID, lineTax, lt.Tax.Minor)
		}
		gross += price
		taxable += lt.Taxable.Minor
		tax += lt.Tax.Minor
		total += lt.Total.Minor
	}
	for _, ct := range res.Components {
		if sums[Component{Name: ct.Name, Rate: ct.Rate}] != ct.Amount.Minor {
			t.Fatalf("seed %d: %s %s summary %d, lines %d", seed, ct.Name, ct.Rate, ct.Amount.Minor, sums[Component{Name: ct.Name, Rate: ct.Rate}])
		}
		if ctx.Pricing == Exclusive {
			exact, _ := ct.Taxable.Scale(int64(ct.Rate), RateScale)
			if exact != ct.Amount {
				t.Fatalf("seed %d: %s %s is %d, taxable %d x rate is %d", seed, ct.Name, ct.Rate, ct.Amount.Minor, ct.Taxable.Minor, exact.Minor)
			}
		}
	}
	if res.Gross.Minor != gross || res.Taxable.Minor != taxable || res.Tax.Minor != tax || res.Total.Minor != total {
		t.Fatalf("seed %d: order totals %+v do not sum the lines", seed, res)
	}
}

func TestTheSnapshotRoundTrips(t *testing.T) {
	res, err := GST{}.Compute([]Line{restaurant(1, 36000), restaurant(2, 9900)}, bengaluru(Inclusive))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var back TaxResult
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res, back) {
		t.Errorf("snapshot changed in a round trip:\n%s", b)
	}
}

func TestEmptyBill(t *testing.T) {
	res, err := VAT{}.Compute(nil, VenueContext{Currency: "GBP", Pricing: Inclusive, VATNumber: "GB1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != gbp(0) || res.Tax != gbp(0) || len(res.Lines) != 0 || len(res.Components) != 0 {
		t.Errorf("an empty bill = %+v", res)
	}
}

func TestEngineRefusesOverflow(t *testing.T) {
	huge := money.Amount{Minor: 1 << 62, Currency: "INR"}
	for _, p := range []Pricing{Exclusive, Inclusive} {
		_, err := GST{}.Compute([]Line{{ID: 1, HSN: "996331", Amount: huge, Rate: 5000}}, bengaluru(p))
		if !errors.Is(err, money.ErrOverflow) {
			t.Errorf("%s: err = %v, want money.ErrOverflow", p, err)
		}
	}
}

func TestAZeroRatedJurisdictionPartChargesNothing(t *testing.T) {
	// Some US counties levy no local rate; the part is still listed so the
	// return shows it.
	ctx := VenueContext{Currency: "USD", Pricing: Exclusive, Jurisdiction: []Component{
		{Name: "OR state", Rate: 0},
		{Name: "Portland arts levy", Rate: 0},
	}}
	res, err := USSalesTax{}.Compute([]Line{{ID: 1, Amount: usd(1000)}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	checkLines(t, res, []lineWant{{1000, []int64{0, 0}, 1000}})
}

func TestRates(t *testing.T) {
	tests := []struct {
		r    Rate
		want string
	}{
		{0, "0%"}, {5000, "5%"}, {2500, "2.5%"}, {8875, "8.875%"}, {250, "0.25%"},
		{375, "0.375%"}, {RateFromBP(1800), "18%"}, {RateScale, "100%"},
	}
	for _, tt := range tests {
		if got := tt.r.String(); got != tt.want {
			t.Errorf("Rate(%d).String() = %q, want %q", tt.r, got, tt.want)
		}
		if err := tt.r.Validate(); err != nil {
			t.Errorf("Rate(%d) is valid: %v", tt.r, err)
		}
	}
	for _, bad := range []Rate{-1, RateScale + 1} {
		if err := bad.Validate(); !errors.Is(err, ErrInvalidRate) {
			t.Errorf("Rate(%d): err = %v", bad, err)
		}
	}
}

// TestProfilesDescribeTheirPaperwork checks every profile implements the
// interface and declares what its invoices and registers need, with no key
// used twice.
func TestProfilesDescribeTheirPaperwork(t *testing.T) {
	profiles := []TaxProfile{GST{}, VAT{}, USSalesTax{}}
	names := map[string]bool{}
	for _, p := range profiles {
		if p.Name() == "" || names[p.Name()] {
			t.Errorf("profile name %q is empty or repeated", p.Name())
		}
		names[p.Name()] = true

		fields, columns := p.InvoiceFields(), p.RegisterColumns()
		if len(fields) == 0 || len(columns) == 0 {
			t.Errorf("%s declares %d invoice fields and %d register columns", p.Name(), len(fields), len(columns))
		}
		seen := map[string]bool{}
		for _, f := range fields {
			if f.Key == "" || f.Label == "" || seen["f"+f.Key] || (f.Scope != ScopeInvoice && f.Scope != ScopeLine) {
				t.Errorf("%s: bad or repeated invoice field %+v", p.Name(), f)
			}
			seen["f"+f.Key] = true
		}
		for _, c := range columns {
			if c.Key == "" || c.Label == "" || seen["c"+c.Key] {
				t.Errorf("%s: bad or repeated register column %+v", p.Name(), c)
			}
			seen["c"+c.Key] = true
		}
	}
	if !contains(GST{}.InvoiceFields(), "gstin") || !contains(GST{}.InvoiceFields(), "hsn") || !contains(VAT{}.InvoiceFields(), "vat_number") {
		t.Error("GST invoices must print GSTIN and HSN; VAT receipts the VAT number")
	}
}

func contains(fields []Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

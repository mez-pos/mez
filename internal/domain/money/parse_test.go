package money

import (
	"errors"
	"math"
	"testing"
)

func TestString(t *testing.T) {
	tests := []struct {
		name string
		in   Amount
		want string
	}{
		{name: "rupees", in: Amount{Minor: 123400, Currency: "INR"}, want: "1234.00 INR"},
		{name: "zero", in: Amount{Minor: 0, Currency: "INR"}, want: "0.00 INR"},
		{name: "under a rupee", in: Amount{Minor: 5, Currency: "INR"}, want: "0.05 INR"},
		{name: "negative under a rupee", in: Amount{Minor: -5, Currency: "INR"}, want: "-0.05 INR"},
		{name: "negative", in: Amount{Minor: -123400, Currency: "INR"}, want: "-1234.00 INR"},
		{name: "exactly one rupee", in: Amount{Minor: 100, Currency: "INR"}, want: "1.00 INR"},

		// Not every currency has two decimal places, and a yen printed as
		// 12.34 instead of 1234 is a hundredfold error on a real bill.
		{name: "yen has no minor unit", in: Amount{Minor: 1234, Currency: "JPY"}, want: "1234 JPY"},
		{name: "dinar has three", in: Amount{Minor: 1234, Currency: "KWD"}, want: "1.234 KWD"},
		{name: "an unknown currency is assumed to have two", in: Amount{Minor: 1234, Currency: "ZZZ"}, want: "12.34 ZZZ"},

		{name: "the most negative value still prints", in: Amount{Minor: math.MinInt64, Currency: "INR"}, want: "-92233720368547758.08 INR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantMinor    int64
		wantCurrency string
		wantErr      error
	}{
		{name: "canonical", in: "1234.00 INR", wantMinor: 123400, wantCurrency: "INR"},
		{name: "currency first", in: "INR 1234.00", wantMinor: 123400, wantCurrency: "INR"},
		{name: "surrounding space", in: "  1234.00   INR  ", wantMinor: 123400, wantCurrency: "INR"},
		{name: "no decimal point", in: "1234 INR", wantMinor: 123400, wantCurrency: "INR"},
		{name: "one decimal place", in: "12.3 INR", wantMinor: 1230, wantCurrency: "INR"},
		{name: "zero", in: "0.00 INR", wantMinor: 0, wantCurrency: "INR"},
		{name: "negative", in: "-12.34 USD", wantMinor: -1234, wantCurrency: "USD"},
		{name: "negative under one", in: "-0.05 INR", wantMinor: -5, wantCurrency: "INR"},
		{name: "explicit plus", in: "+12.34 INR", wantMinor: 1234, wantCurrency: "INR"},
		{name: "grouped digits", in: "1,234.56 INR", wantMinor: 123456, wantCurrency: "INR"},
		{name: "indian grouping", in: "1,00,000.00 INR", wantMinor: 10000000, wantCurrency: "INR"},
		{name: "leading dot", in: ".05 INR", wantMinor: 5, wantCurrency: "INR"},

		{name: "yen has no minor unit", in: "1234 JPY", wantMinor: 1234, wantCurrency: "JPY"},
		{name: "dinar has three", in: "1.234 KWD", wantMinor: 1234, wantCurrency: "KWD"},

		{name: "more precision than the currency has", in: "1.234 INR", wantErr: ErrPrecision},
		{name: "a fraction of a yen", in: "12.34 JPY", wantErr: ErrPrecision},

		{name: "float notation is not money", in: "1e3 INR", wantErr: ErrSyntax},
		{name: "empty", in: "", wantErr: ErrSyntax},
		{name: "no currency", in: "1234.00", wantErr: ErrSyntax},
		{name: "no amount", in: "INR", wantErr: ErrSyntax},
		{name: "not a number", in: "abc INR", wantErr: ErrSyntax},
		{name: "two decimal points", in: "12.34.56 INR", wantErr: ErrSyntax},
		{name: "a symbol instead of a code", in: "₹1234.00 INR", wantErr: ErrSyntax},
		{name: "three parts", in: "1234.00 INR INR", wantErr: ErrSyntax},
		{name: "sign with no digits", in: "- INR", wantErr: ErrSyntax},

		{name: "lowercase currency", in: "1234.00 inr", wantErr: ErrInvalidCurrency},
		{name: "a currency that is not three letters", in: "1234.00 RUPEE", wantErr: ErrInvalidCurrency},

		{name: "too large for int64", in: "99999999999999999999.00 INR", wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse(%q) error = %v, want %v", tt.in, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Minor != tt.wantMinor {
				t.Errorf("Parse(%q).Minor = %d, want %d", tt.in, got.Minor, tt.wantMinor)
			}
			if got.Currency != tt.wantCurrency {
				t.Errorf("Parse(%q).Currency = %q, want %q", tt.in, got.Currency, tt.wantCurrency)
			}
		})
	}
}

// TestParseRoundTripsString pins the two halves together: anything String
// writes, Parse reads back to the same amount. Without this they drift apart
// the first time a currency with a different exponent shows up.
func TestParseRoundTripsString(t *testing.T) {
	amounts := []Amount{
		{Minor: 0, Currency: "INR"},
		{Minor: 1, Currency: "INR"},
		{Minor: -1, Currency: "INR"},
		{Minor: 99, Currency: "INR"},
		{Minor: 100, Currency: "INR"},
		{Minor: 123456789, Currency: "INR"},
		{Minor: -123456789, Currency: "USD"},
		{Minor: 1234, Currency: "JPY"},
		{Minor: -1234, Currency: "JPY"},
		{Minor: 1234, Currency: "KWD"},
		{Minor: -1, Currency: "KWD"},
		{Minor: math.MaxInt64, Currency: "INR"},
		{Minor: math.MinInt64 + 1, Currency: "INR"},
	}

	for _, want := range amounts {
		s := want.String()
		got, err := Parse(s)
		if err != nil {
			t.Errorf("Parse(%q) from %+v: %v", s, want, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", s, got, want)
		}
	}
}

func TestExponent(t *testing.T) {
	tests := []struct {
		currency string
		want     int
	}{
		{currency: "INR", want: 2},
		{currency: "USD", want: 2},
		{currency: "GBP", want: 2},
		{currency: "JPY", want: 0},
		{currency: "KRW", want: 0},
		{currency: "VND", want: 0},
		{currency: "KWD", want: 3},
		{currency: "BHD", want: 3},
		{currency: "OMR", want: 3},
		{currency: "ZZZ", want: 2},
		{currency: "", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			if got := Exponent(tt.currency); got != tt.want {
				t.Errorf("Exponent(%q) = %d, want %d", tt.currency, got, tt.want)
			}
		})
	}
}

func TestExponentsAgreesWithExponent(t *testing.T) {
	got := Exponents()
	if len(got) == 0 {
		t.Fatal("Exponents is empty")
	}
	for code, e := range got {
		if e == DefaultExponent {
			t.Errorf("%s is listed with the default exponent %d; the list is exceptions only", code, e)
		}
		if want := Exponent(code); e != want {
			t.Errorf("Exponents()[%q] = %d, Exponent says %d", code, e, want)
		}
	}
	if Exponent("INR") != DefaultExponent {
		t.Errorf("INR should take the default exponent")
	}
}

func TestExponentsIsACopy(t *testing.T) {
	Exponents()["JPY"] = 2
	delete(Exponents(), "KWD")
	if Exponent("JPY") != 0 || Exponent("KWD") != 3 {
		t.Fatal("changing the map Exponents returned changed how amounts are read")
	}
}

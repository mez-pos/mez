package money

import (
	"errors"
	"math"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		minor    int64
		currency string
		want     Amount
		wantErr  error
	}{
		{name: "rupees", minor: 123400, currency: "INR", want: Amount{Minor: 123400, Currency: "INR"}},
		{name: "zero", minor: 0, currency: "INR", want: Amount{Minor: 0, Currency: "INR"}},
		{name: "negative", minor: -50, currency: "USD", want: Amount{Minor: -50, Currency: "USD"}},
		{name: "lowercase is rejected loudly", currency: "inr", wantErr: ErrInvalidCurrency},
		{name: "empty currency", currency: "", wantErr: ErrInvalidCurrency},
		{name: "two letters", currency: "IN", wantErr: ErrInvalidCurrency},
		{name: "four letters", currency: "INRR", wantErr: ErrInvalidCurrency},
		{name: "digits", currency: "IN1", wantErr: ErrInvalidCurrency},
		{name: "most negative minor is not representable", minor: math.MinInt64, currency: "INR", wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.minor, tt.currency)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("New(%d, %q) error = %v, want %v", tt.minor, tt.currency, err, tt.wantErr)
			}
			if tt.wantErr == nil && got != tt.want {
				t.Errorf("New(%d, %q) = %+v, want %+v", tt.minor, tt.currency, got, tt.want)
			}
		})
	}
}

func TestAddAndSub(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Amount
		wantAdd int64
		wantSub int64
		wantErr error
	}{
		{
			name:    "ordinary",
			a:       Amount{Minor: 123400, Currency: "INR"},
			b:       Amount{Minor: 600, Currency: "INR"},
			wantAdd: 124000,
			wantSub: 122800,
		},
		{
			name:    "zero is the identity",
			a:       Amount{Minor: 0, Currency: "INR"},
			b:       Amount{Minor: 0, Currency: "INR"},
			wantAdd: 0,
			wantSub: 0,
		},
		{
			name:    "subtraction crosses zero",
			a:       Amount{Minor: 100, Currency: "INR"},
			b:       Amount{Minor: 350, Currency: "INR"},
			wantAdd: 450,
			wantSub: -250,
		},
		{
			name:    "both negative",
			a:       Amount{Minor: -100, Currency: "INR"},
			b:       Amount{Minor: -250, Currency: "INR"},
			wantAdd: -350,
			wantSub: 150,
		},
		{
			name:    "currency mismatch",
			a:       Amount{Minor: 100, Currency: "INR"},
			b:       Amount{Minor: 100, Currency: "USD"},
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:    "zero does not excuse a currency mismatch",
			a:       Amount{Minor: 0, Currency: "INR"},
			b:       Amount{Minor: 0, Currency: "USD"},
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:    "the zero value is not a universal identity",
			a:       Amount{Minor: 100, Currency: "INR"},
			b:       Amount{},
			wantErr: ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sum, err := tt.a.Add(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Add error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && sum.Minor != tt.wantAdd {
				t.Errorf("Add = %d, want %d", sum.Minor, tt.wantAdd)
			}

			diff, err := tt.a.Sub(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sub error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && diff.Minor != tt.wantSub {
				t.Errorf("Sub = %d, want %d", diff.Minor, tt.wantSub)
			}
		})
	}
}

func TestAddAndSubOverflow(t *testing.T) {
	big := Amount{Minor: math.MaxInt64, Currency: "INR"}
	one := Amount{Minor: 1, Currency: "INR"}

	if _, err := big.Add(one); !errors.Is(err, ErrOverflow) {
		t.Errorf("Add overflow error = %v, want %v", err, ErrOverflow)
	}
	small := Amount{Minor: math.MinInt64 + 2, Currency: "INR"}
	if _, err := small.Sub(one); err != nil {
		t.Errorf("Sub that stays in range: error = %v, want nil", err)
	}
}

// TestNoOperationProducesAnUnnegatableAmount pins the guarantee Neg documents:
// an amount this package hands back can always be negated for a refund.
//
// math.MinInt64 fits in an int64 but has no positive counterpart, so it is the
// one value an Amount may not hold. New refuses it, and so must any arithmetic
// that would land on it — including subtractions that do not wrap at all and
// would otherwise look perfectly fine.
func TestNoOperationProducesAnUnnegatableAmount(t *testing.T) {
	edge := Amount{Minor: math.MinInt64 + 1, Currency: "INR"}
	one := Amount{Minor: 1, Currency: "INR"}
	minusOne := Amount{Minor: -1, Currency: "INR"}

	if _, err := edge.Sub(one); !errors.Is(err, ErrOverflow) {
		t.Errorf("Sub landing on the most negative value: error = %v, want %v", err, ErrOverflow)
	}
	if _, err := edge.Add(minusOne); !errors.Is(err, ErrOverflow) {
		t.Errorf("Add landing on the most negative value: error = %v, want %v", err, ErrOverflow)
	}
	if _, err := New(math.MinInt64, "INR"); !errors.Is(err, ErrOverflow) {
		t.Errorf("New with the most negative value: error = %v, want %v", err, ErrOverflow)
	}
}

func TestCurrencyMismatchErrorCarriesBothSides(t *testing.T) {
	a := Amount{Minor: 1, Currency: "INR"}
	b := Amount{Minor: 1, Currency: "USD"}

	_, err := a.Add(b)

	var mismatch *CurrencyMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Add error = %v, want a *CurrencyMismatchError", err)
	}
	if mismatch.Want != "INR" || mismatch.Got != "USD" {
		t.Errorf("mismatch = %+v, want {Want:INR Got:USD}", mismatch)
	}
	if !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("errors.Is(err, ErrCurrencyMismatch) = false, want true")
	}
}

func TestNeg(t *testing.T) {
	tests := []struct {
		name    string
		in      Amount
		want    int64
		wantErr error
	}{
		{name: "positive", in: Amount{Minor: 250, Currency: "INR"}, want: -250},
		{name: "negative", in: Amount{Minor: -250, Currency: "INR"}, want: 250},
		{name: "zero stays zero", in: Amount{Minor: 0, Currency: "INR"}, want: 0},
		{name: "most negative minor overflows", in: Amount{Minor: math.MinInt64, Currency: "INR"}, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Neg()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Neg error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Minor != tt.want {
				t.Errorf("Neg = %d, want %d", got.Minor, tt.want)
			}
			if got.Currency != tt.in.Currency {
				t.Errorf("Neg currency = %q, want %q", got.Currency, tt.in.Currency)
			}
		})
	}
}

func TestIsZero(t *testing.T) {
	tests := []struct {
		name string
		in   Amount
		want bool
	}{
		{name: "zero with a currency", in: Amount{Minor: 0, Currency: "INR"}, want: true},
		{name: "the zero value", in: Amount{}, want: true},
		{name: "one paisa", in: Amount{Minor: 1, Currency: "INR"}, want: false},
		{name: "minus one paisa", in: Amount{Minor: -1, Currency: "INR"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.IsZero(); got != tt.want {
				t.Errorf("IsZero = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		a, b    Amount
		want    int
		wantErr error
	}{
		{name: "less", a: Amount{Minor: 1, Currency: "INR"}, b: Amount{Minor: 2, Currency: "INR"}, want: -1},
		{name: "equal", a: Amount{Minor: 2, Currency: "INR"}, b: Amount{Minor: 2, Currency: "INR"}, want: 0},
		{name: "greater", a: Amount{Minor: 3, Currency: "INR"}, b: Amount{Minor: 2, Currency: "INR"}, want: 1},
		{name: "negative is less than zero", a: Amount{Minor: -1, Currency: "INR"}, b: Amount{Minor: 0, Currency: "INR"}, want: -1},
		{
			name: "far apart without overflowing",
			a:    Amount{Minor: math.MinInt64 + 1, Currency: "INR"},
			b:    Amount{Minor: math.MaxInt64, Currency: "INR"},
			want: -1,
		},
		{name: "mismatch", a: Amount{Minor: 1, Currency: "INR"}, b: Amount{Minor: 1, Currency: "USD"}, wantErr: ErrCurrencyMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.a.Compare(tt.b)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Compare error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && got != tt.want {
				t.Errorf("Compare = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSum(t *testing.T) {
	tests := []struct {
		name     string
		currency string
		amounts  []Amount
		want     int64
		wantErr  error
	}{
		{
			name:     "empty sums to zero in the named currency",
			currency: "INR",
			want:     0,
		},
		{
			name:     "three tenders",
			currency: "INR",
			amounts: []Amount{
				{Minor: 33334, Currency: "INR"},
				{Minor: 33333, Currency: "INR"},
				{Minor: 33333, Currency: "INR"},
			},
			want: 100000,
		},
		{
			name:     "a refund nets out",
			currency: "INR",
			amounts: []Amount{
				{Minor: 500, Currency: "INR"},
				{Minor: -500, Currency: "INR"},
			},
			want: 0,
		},
		{
			name:     "one stray currency fails the whole sum",
			currency: "INR",
			amounts: []Amount{
				{Minor: 100, Currency: "INR"},
				{Minor: 100, Currency: "USD"},
			},
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:     "invalid target currency",
			currency: "inr",
			wantErr:  ErrInvalidCurrency,
		},
		{
			name:     "overflow",
			currency: "INR",
			amounts: []Amount{
				{Minor: math.MaxInt64, Currency: "INR"},
				{Minor: 1, Currency: "INR"},
			},
			wantErr: ErrOverflow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sum(tt.currency, tt.amounts...)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Sum error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Minor != tt.want {
				t.Errorf("Sum = %d, want %d", got.Minor, tt.want)
			}
			if got.Currency != tt.currency {
				t.Errorf("Sum currency = %q, want %q", got.Currency, tt.currency)
			}
		})
	}
}

func TestMul(t *testing.T) {
	tests := []struct {
		name    string
		in      Amount
		bp      int32
		want    int64
		wantErr error
	}{
		{name: "18 percent GST on 100 rupees", in: Amount{Minor: 10000, Currency: "INR"}, bp: 1800, want: 1800},
		{name: "whole", in: Amount{Minor: 12345, Currency: "INR"}, bp: 10000, want: 12345},
		{name: "nothing", in: Amount{Minor: 12345, Currency: "INR"}, bp: 0, want: 0},
		{name: "zero amount", in: Amount{Minor: 0, Currency: "INR"}, bp: 1800, want: 0},
		{name: "a negative rate is a deduction", in: Amount{Minor: 10000, Currency: "INR"}, bp: -1000, want: -1000},
		{name: "negative amount", in: Amount{Minor: -10000, Currency: "INR"}, bp: 1800, want: -1800},

		// Half-to-even boundaries. At 5000bp the product is exactly half a
		// minor unit whenever the amount is odd, which is the case that
		// separates banker's rounding from every other rule.
		{name: "half rounds down to even", in: Amount{Minor: 5, Currency: "INR"}, bp: 5000, want: 2},
		{name: "half rounds up to even", in: Amount{Minor: 7, Currency: "INR"}, bp: 5000, want: 4},
		{name: "half at one rounds to zero", in: Amount{Minor: 1, Currency: "INR"}, bp: 5000, want: 0},
		{name: "half at three rounds to two", in: Amount{Minor: 3, Currency: "INR"}, bp: 5000, want: 2},
		{name: "negative half rounds to even", in: Amount{Minor: -5, Currency: "INR"}, bp: 5000, want: -2},
		{name: "negative half rounds away to even", in: Amount{Minor: -7, Currency: "INR"}, bp: 5000, want: -4},
		{name: "above half rounds away from zero", in: Amount{Minor: 3, Currency: "INR"}, bp: 3334, want: 1},
		{name: "below half rounds toward zero", in: Amount{Minor: 3, Currency: "INR"}, bp: 1666, want: 0},

		{name: "overflow", in: Amount{Minor: math.MaxInt64 / 2, Currency: "INR"}, bp: 30000, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Mul(tt.bp)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Mul error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Minor != tt.want {
				t.Errorf("Mul(%d bp) = %d, want %d", tt.bp, got.Minor, tt.want)
			}
			if got.Currency != tt.in.Currency {
				t.Errorf("Mul currency = %q, want %q", got.Currency, tt.in.Currency)
			}
		})
	}
}

func TestTimes(t *testing.T) {
	tests := []struct {
		name    string
		in      Amount
		n       int64
		want    Amount
		wantErr error
	}{
		{name: "a line of three", in: Amount{Minor: 22000, Currency: "INR"}, n: 3, want: Amount{Minor: 66000, Currency: "INR"}},
		{name: "times one", in: Amount{Minor: 22000, Currency: "INR"}, n: 1, want: Amount{Minor: 22000, Currency: "INR"}},
		{name: "times zero", in: Amount{Minor: 22000, Currency: "INR"}, n: 0, want: Amount{Minor: 0, Currency: "INR"}},
		{name: "negative", in: Amount{Minor: -5, Currency: "INR"}, n: 4, want: Amount{Minor: -20, Currency: "INR"}},
		{name: "overflow", in: Amount{Minor: math.MaxInt64 / 2, Currency: "INR"}, n: 3, wantErr: ErrOverflow},
		{name: "min int is refused", in: Amount{Minor: math.MinInt64 / 2, Currency: "INR"}, n: 2, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.Times(tt.n)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Times(%d) err = %v, want %v", tt.n, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Times(%d) = %+v, want %+v", tt.n, got, tt.want)
			}
		})
	}
}

func TestScale(t *testing.T) {
	tests := []struct {
		name     string
		in       int64
		num, den int64
		want     int64
		wantErr  error
	}{
		{name: "back out 5% from 100.00", in: 10000, num: 100000, den: 105000, want: 9524}, // 9523.81
		{name: "back out 5% from 99.00", in: 9900, num: 100000, den: 105000, want: 9429},   // 9428.57
		{name: "back out 20% from 3.40", in: 340, num: 100000, den: 120000, want: 283},     // 283.33
		{name: "an odd divisor", in: 10000, num: 100000, den: 108875, want: 9185},          // 9184.845
		{name: "exactly half rounds to even, down", in: 25, num: 1, den: 10, want: 2},      // 2.5
		{name: "exactly half rounds to even, up", in: 35, num: 1, den: 10, want: 4},        // 3.5
		{name: "0.375% of 10.75", in: 1075, num: 375, den: 100000, want: 4},                // 4.03125
		{name: "negative is symmetric", in: -25, num: 1, den: 10, want: -2},
		{name: "overflow", in: math.MaxInt64 / 2, num: 3, den: 1, wantErr: ErrOverflow},
		{name: "zero denominator", in: 100, num: 1, den: 0, wantErr: ErrInvalidStep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Amount{Minor: tt.in, Currency: "INR"}.Scale(tt.num, tt.den)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && (got.Minor != tt.want || got.Currency != "INR") {
				t.Errorf("Scale = %+v, want %d INR", got, tt.want)
			}
		})
	}
}

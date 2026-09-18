package money

import (
	"errors"
	"math"
	"testing"
)

func TestRoundToNearest(t *testing.T) {
	tests := []struct {
		name    string
		in      Amount
		step    int64
		want    int64
		wantErr error
	}{
		{name: "a step of one changes nothing", in: Amount{Minor: 12345, Currency: "INR"}, step: 1, want: 12345},
		{name: "already on the step", in: Amount{Minor: 100, Currency: "INR"}, step: 50, want: 100},
		{name: "below half rounds down", in: Amount{Minor: 120, Currency: "INR"}, step: 50, want: 100},
		{name: "above half rounds up", in: Amount{Minor: 130, Currency: "INR"}, step: 50, want: 150},

		// Exactly half a step: the multiple of the step is what must come out
		// even, not the amount. 25 sits between 0 and 50, so it goes to 0;
		// 75 sits between 50 and 100, so it goes to 100.
		{name: "half goes to the even multiple down", in: Amount{Minor: 25, Currency: "INR"}, step: 50, want: 0},
		{name: "half goes to the even multiple up", in: Amount{Minor: 75, Currency: "INR"}, step: 50, want: 100},
		{name: "half at one and a quarter", in: Amount{Minor: 125, Currency: "INR"}, step: 50, want: 100},
		{name: "half at one and three quarters", in: Amount{Minor: 175, Currency: "INR"}, step: 50, want: 200},

		{name: "negative half", in: Amount{Minor: -25, Currency: "INR"}, step: 50, want: 0},
		{name: "negative half the other way", in: Amount{Minor: -75, Currency: "INR"}, step: 50, want: -100},
		{name: "negative below half", in: Amount{Minor: -120, Currency: "INR"}, step: 50, want: -100},

		{name: "zero", in: Amount{Minor: 0, Currency: "INR"}, step: 50, want: 0},
		{name: "rounding a small amount away entirely", in: Amount{Minor: 4, Currency: "INR"}, step: 100, want: 0},

		// The classic cash-rounding case: no one-paisa coins, so settle to
		// the nearest 50 paise.
		{name: "cash rounding on a real total", in: Amount{Minor: 24799, Currency: "INR"}, step: 50, want: 24800},

		{name: "step of zero", in: Amount{Minor: 100, Currency: "INR"}, step: 0, wantErr: ErrInvalidStep},
		{name: "negative step", in: Amount{Minor: 100, Currency: "INR"}, step: -50, wantErr: ErrInvalidStep},
		{name: "overflow", in: Amount{Minor: math.MaxInt64, Currency: "INR"}, step: 1000, wantErr: ErrOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.RoundToNearest(tt.step)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("RoundToNearest(%d) error = %v, want %v", tt.step, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Minor != tt.want {
				t.Errorf("RoundToNearest(%d) on %d = %d, want %d", tt.step, tt.in.Minor, got.Minor, tt.want)
			}
			if got.Currency != tt.in.Currency {
				t.Errorf("RoundToNearest currency = %q, want %q", got.Currency, tt.in.Currency)
			}
		})
	}
}

// TestRoundToNearestIsSymmetric checks the property that makes half-to-even
// worth having: rounding a value and rounding its negation give answers that
// are themselves negations. A rule biased in one direction would drift over a
// day of refunds.
func TestRoundToNearestIsSymmetric(t *testing.T) {
	const step = 50

	for minor := int64(-500); minor <= 500; minor++ {
		up, err := Amount{Minor: minor, Currency: "INR"}.RoundToNearest(step)
		if err != nil {
			t.Fatalf("rounding %d: %v", minor, err)
		}
		down, err := Amount{Minor: -minor, Currency: "INR"}.RoundToNearest(step)
		if err != nil {
			t.Fatalf("rounding %d: %v", -minor, err)
		}
		if up.Minor != -down.Minor {
			t.Fatalf("round(%d) = %d but round(%d) = %d: not symmetric", minor, up.Minor, -minor, down.Minor)
		}
	}
}

// TestRoundToNearestNeverMovesMoreThanHalfAStep is the safety property: the
// rounded figure is always the nearest multiple, so a cash-rounding rule can
// never shift a total by more than half the smallest coin.
func TestRoundToNearestNeverMovesMoreThanHalfAStep(t *testing.T) {
	steps := []int64{1, 5, 25, 50, 100}

	for _, step := range steps {
		for minor := int64(-1000); minor <= 1000; minor++ {
			got, err := Amount{Minor: minor, Currency: "INR"}.RoundToNearest(step)
			if err != nil {
				t.Fatalf("rounding %d to %d: %v", minor, step, err)
			}
			if got.Minor%step != 0 {
				t.Fatalf("round(%d, step %d) = %d, which is not a multiple of the step", minor, step, got.Minor)
			}
			drift := got.Minor - minor
			if drift < 0 {
				drift = -drift
			}
			if 2*drift > step {
				t.Fatalf("round(%d, step %d) = %d, moved %d which is more than half a step", minor, step, got.Minor, drift)
			}
		}
	}
}

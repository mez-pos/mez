package money

import (
	"errors"
	"testing"
)

func TestSplitEvenly(t *testing.T) {
	tests := []struct {
		name    string
		total   Amount
		n       int
		want    []int64
		wantErr error
	}{
		{
			name:  "divides exactly",
			total: Amount{Minor: 100000, Currency: "INR"},
			n:     4,
			want:  []int64{25000, 25000, 25000, 25000},
		},
		{
			// The case named in docs/02-domain.md: a thousand rupees three
			// ways is 333.34 / 333.33 / 333.33, and the first payer carries
			// the extra paisa every time.
			name:  "a thousand rupees three ways",
			total: Amount{Minor: 100000, Currency: "INR"},
			n:     3,
			want:  []int64{33334, 33333, 33333},
		},
		{
			// A prime number of paise cannot divide evenly by anything, which
			// is exactly where a naive split loses a unit.
			name:  "a prime number of paise three ways",
			total: Amount{Minor: 100003, Currency: "INR"},
			n:     3,
			want:  []int64{33335, 33334, 33334},
		},
		{
			name:  "a prime number of paise seven ways",
			total: Amount{Minor: 104729, Currency: "INR"},
			n:     7,
			want:  []int64{14962, 14962, 14961, 14961, 14961, 14961, 14961},
		},
		{
			name:  "one way is the whole amount",
			total: Amount{Minor: 12345, Currency: "INR"},
			n:     1,
			want:  []int64{12345},
		},
		{
			name:  "zero splits into zeroes",
			total: Amount{Minor: 0, Currency: "INR"},
			n:     3,
			want:  []int64{0, 0, 0},
		},
		{
			name:  "more payers than paise",
			total: Amount{Minor: 2, Currency: "INR"},
			n:     5,
			want:  []int64{1, 1, 0, 0, 0},
		},
		{
			// A refund splits the same way, mirrored, so that reversing a
			// split payment returns each payer what they put in.
			name:  "a negative total splits symmetrically",
			total: Amount{Minor: -100000, Currency: "INR"},
			n:     3,
			want:  []int64{-33334, -33333, -33333},
		},
		{
			name:    "zero payers",
			total:   Amount{Minor: 100, Currency: "INR"},
			n:       0,
			wantErr: ErrInvalidSplit,
		},
		{
			name:    "negative payers",
			total:   Amount{Minor: 100, Currency: "INR"},
			n:       -2,
			wantErr: ErrInvalidSplit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SplitEvenly(tt.total, tt.n)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("SplitEvenly(%d, %d) error = %v, want %v", tt.total.Minor, tt.n, err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("SplitEvenly returned %d parts, want %d", len(got), len(tt.want))
			}
			for i, part := range got {
				if part.Minor != tt.want[i] {
					t.Errorf("part %d = %d, want %d (whole split: %v)", i, part.Minor, tt.want[i], minors(got))
				}
				if part.Currency != tt.total.Currency {
					t.Errorf("part %d currency = %q, want %q", i, part.Currency, tt.total.Currency)
				}
			}

			sum, err := Sum(tt.total.Currency, got...)
			if err != nil {
				t.Fatalf("summing the parts: %v", err)
			}
			if sum.Minor != tt.total.Minor {
				t.Errorf("parts sum to %d, want %d: the split lost money", sum.Minor, tt.total.Minor)
			}
		})
	}
}

// TestSplitEvenlyAlwaysSumsToTheTotal is the split-payment invariant from
// docs/02-domain.md stated as a property: across a wide sweep of totals and
// payer counts, the parts always add back up to exactly what was owed. This is
// the one thing about this function that must never be false.
func TestSplitEvenlyAlwaysSumsToTheTotal(t *testing.T) {
	for _, total := range []int64{0, 1, 2, 7, 99, 100, 101, 100000, 100003, 104729, -1, -7, -100003} {
		for n := 1; n <= 17; n++ {
			parts, err := SplitEvenly(Amount{Minor: total, Currency: "INR"}, n)
			if err != nil {
				t.Fatalf("SplitEvenly(%d, %d): %v", total, n, err)
			}

			sum, err := Sum("INR", parts...)
			if err != nil {
				t.Fatalf("summing SplitEvenly(%d, %d): %v", total, n, err)
			}
			if sum.Minor != total {
				t.Fatalf("SplitEvenly(%d, %d) sums to %d: %v", total, n, sum.Minor, minors(parts))
			}

			// No part may be more than one minor unit away from any other, or
			// the split is not even in any sense a customer would accept.
			lo, hi := parts[0].Minor, parts[0].Minor
			for _, p := range parts {
				if p.Minor < lo {
					lo = p.Minor
				}
				if p.Minor > hi {
					hi = p.Minor
				}
			}
			if hi-lo > 1 {
				t.Fatalf("SplitEvenly(%d, %d) spread from %d to %d: %v", total, n, lo, hi, minors(parts))
			}
		}
	}
}

func minors(amounts []Amount) []int64 {
	out := make([]int64, len(amounts))
	for i, a := range amounts {
		out[i] = a.Minor
	}
	return out
}

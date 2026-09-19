package money

import (
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

func TestAllocate(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		weights []int64
		want    []int64
	}{
		{"in proportion", 5000, []int64{30000, 10000}, []int64{3750, 1250}},
		{"a paisa left over goes to the largest remainder", 100, []int64{1, 1, 1}, []int64{34, 33, 33}},
		{"remainders decide, not position", 10, []int64{1, 2}, []int64{3, 7}}, // 3.33, 6.67
		{"a zero weight gets nothing", 1000, []int64{0, 5, 5}, []int64{0, 500, 500}},
		{"nothing to allocate", 0, []int64{7, 3}, []int64{0, 0}},
		{"zero over zero weight", 0, []int64{0, 0}, []int64{0, 0}},
		{"no parts", 0, nil, []int64{}},
		{"huge values do not overflow", math.MaxInt64 - 1, []int64{math.MaxInt64 - 1, math.MaxInt64 - 1},
			[]int64{(math.MaxInt64 - 1) / 2, (math.MaxInt64 - 1) / 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Allocate(Amount{Minor: tt.total, Currency: "INR"}, tt.weights)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("%d parts, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Minor != tt.want[i] || got[i].Currency != "INR" {
					t.Errorf("parts = %+v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}

func TestAllocateRefuses(t *testing.T) {
	tests := []struct {
		name    string
		total   int64
		weights []int64
		wantErr error
	}{
		{"a negative total", -1, []int64{1}, ErrInvalidSplit},
		{"a negative weight", 10, []int64{5, -1}, ErrInvalidSplit},
		{"money over zero weight", 10, []int64{0, 0}, ErrInvalidSplit},
		{"weights past 2^64", 10, []int64{math.MaxInt64, math.MaxInt64, math.MaxInt64}, ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Allocate(Amount{Minor: tt.total, Currency: "INR"}, tt.weights); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestAllocateSumsExactly is the property: for random totals and weights,
// the parts sum to the total and no part is more than one minor unit from
// the floor of its exact share. No floats: integers are the oracle.
func TestAllocateSumsExactly(t *testing.T) {
	for seed := range uint64(1000) {
		rng := rand.New(rand.NewPCG(seed, 3))
		total := rng.Int64N(10_000_000)
		weights := make([]int64, 1+rng.IntN(10))
		var sum int64
		for i := range weights {
			weights[i] = rng.Int64N(1_000_000)
			sum += weights[i]
		}
		if sum == 0 {
			continue
		}
		parts, err := Allocate(Amount{Minor: total, Currency: "INR"}, weights)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		var got int64
		for i, p := range parts {
			got += p.Minor
			// The exact share's floor, in integers: total × w / sum fits in
			// an int64 at these sizes. A part is the floor or one more.
			floor := total * weights[i] / sum
			if p.Minor != floor && p.Minor != floor+1 {
				t.Fatalf("seed %d: part %d is %d, exact share's floor %d", seed, i, p.Minor, floor)
			}
		}
		if got != total {
			t.Fatalf("seed %d: parts sum to %d, want %d", seed, got, total)
		}
	}
}

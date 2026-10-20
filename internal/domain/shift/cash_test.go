package shift

import (
	"errors"
	"strings"
	"testing"

	"github.com/mez-pos/mez/internal/domain/money"
)

func inr(minor int64) money.Amount { return money.Amount{Minor: minor, Currency: "INR"} }

func TestNewCountAddsUpTheDrawer(t *testing.T) {
	c, err := NewCount(CountOpening, "INR", []Denomination{
		{Face: 1000, Count: 7},  // ₹10 x 7
		{Face: 50000, Count: 2}, // ₹500 x 2
		{Face: 10000, Count: 0}, // ₹100 x 0
		{Face: 200, Count: 5},   // ₹2 x 5
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Total != inr(108000) {
		t.Errorf("total = %s, want 1080.00", c.Total)
	}
	faces := []int64{}
	for _, d := range c.Denominations {
		faces = append(faces, d.Face)
	}
	if want := []int64{50000, 10000, 1000, 200}; !equal(faces, want) {
		t.Errorf("faces = %v, want largest first %v", faces, want)
	}
}

func TestNewCountRefusals(t *testing.T) {
	tests := []struct {
		name  string
		kind  CountKind
		cur   string
		denom []Denomination
	}{
		{"a note of nothing", CountOpening, "INR", []Denomination{{Face: 0, Count: 1}}},
		{"a negative note", CountOpening, "INR", []Denomination{{Face: -500, Count: 1}}},
		{"negative pieces", CountOpening, "INR", []Denomination{{Face: 500, Count: -1}}},
		{"a warehouse of coins", CountOpening, "INR", []Denomination{{Face: 100, Count: maxPieces + 1}}},
		{"a face listed twice", CountOpening, "INR", []Denomination{{Face: 500, Count: 1}, {Face: 500, Count: 2}}},
		{"an unknown kind", "midway", "INR", nil},
		{"a malformed currency", CountOpening, "inr", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewCount(tt.kind, tt.cur, tt.denom); !errors.Is(err, ErrInvalidCount) {
				t.Errorf("err = %v, want ErrInvalidCount", err)
			}
		})
	}
}

func TestCountValidate(t *testing.T) {
	good := Count{Kind: CountClosing, Denominations: []Denomination{{Face: 50000, Count: 1}}, Total: inr(50000)}
	tests := []struct {
		name  string
		count Count
		ok    bool
	}{
		{"denominations that add up", good, true},
		{"a lump sum", Count{Kind: CountOpening, Total: inr(200000)}, true},
		{"an empty drawer", Count{Kind: CountClosing, Total: inr(0)}, true},
		{"denominations that do not add up", Count{Kind: CountClosing, Denominations: good.Denominations, Total: inr(50100)}, false},
		{"a negative drawer", Count{Kind: CountOpening, Total: inr(-1)}, false},
		{"another currency", Count{Kind: CountOpening, Total: money.Amount{Minor: 100, Currency: "USD"}}, false},
		{"no kind", Count{Total: inr(100)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.count.Validate("INR")
			if tt.ok != (err == nil) {
				t.Errorf("Validate = %v, want ok %v", err, tt.ok)
			}
			if err != nil && !errors.Is(err, ErrInvalidCount) {
				t.Errorf("err = %v, want ErrInvalidCount", err)
			}
		})
	}
}

func TestLumpSum(t *testing.T) {
	c, err := LumpSum(CountOpening, inr(250000))
	if err != nil {
		t.Fatal(err)
	}
	if c.Total != inr(250000) || c.Denominations == nil || len(c.Denominations) != 0 {
		t.Errorf("LumpSum = %+v; want 2500.00 and no denominations, not nil", c)
	}
	if _, err := LumpSum(CountOpening, inr(-5)); !errors.Is(err, ErrInvalidCount) {
		t.Errorf("a negative float: err = %v, want ErrInvalidCount", err)
	}
}

func TestNewMovement(t *testing.T) {
	tests := []struct {
		name   string
		kind   MovementKind
		amount money.Amount
		reason string
		by     int64
		ok     bool
	}{
		{"a payout", CashOut, inr(12000), "  milk from the shop ", 2, true},
		{"change brought in", CashIn, inr(100000), "coins from the bank", 2, true},
		{"a reason at the limit", CashOut, inr(1), strings.Repeat("é", MaxReasonLength), 2, true},
		{"no reason", CashOut, inr(12000), "   ", 2, false},
		{"a reason over the limit", CashOut, inr(1), strings.Repeat("a", MaxReasonLength+1), 2, false},
		{"nothing moved", CashIn, inr(0), "float", 2, false},
		{"a negative amount", CashOut, inr(-100), "float", 2, false},
		{"another currency", CashIn, money.Amount{Minor: 100, Currency: "USD"}, "float", 2, false},
		{"nobody authorised it", CashOut, inr(100), "milk", 0, false},
		{"an unknown kind", "cash_sideways", inr(100), "milk", 2, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMovement(tt.kind, tt.amount, tt.reason, tt.by, "INR")
			if tt.ok != (err == nil) {
				t.Fatalf("NewMovement = %v, want ok %v", err, tt.ok)
			}
			if err != nil {
				if !errors.Is(err, ErrInvalidMovement) {
					t.Errorf("err = %v, want ErrInvalidMovement", err)
				}
				return
			}
			if m.Reason != strings.TrimSpace(tt.reason) {
				t.Errorf("reason = %q, want it trimmed", m.Reason)
			}
		})
	}
}

func TestMovementValidateRefusesAnUntrimmedReason(t *testing.T) {
	m := Movement{Kind: CashIn, Amount: inr(100), Reason: " float", AuthorisedBy: 1}
	if err := m.Validate("INR"); !errors.Is(err, ErrInvalidMovement) {
		t.Errorf("err = %v, want ErrInvalidMovement", err)
	}
}

func TestDrawer(t *testing.T) {
	float := Count{Kind: CountOpening, Total: inr(200000)}
	tests := []struct {
		name  string
		moves []Movement
		want  money.Amount
	}{
		{"the float alone", nil, inr(200000)},
		{"in and out", []Movement{
			{Kind: CashIn, Amount: inr(50000)},
			{Kind: CashOut, Amount: inr(12050)},
			{Kind: CashOut, Amount: inr(100000)},
		}, inr(137950)},
		{"more out than there was", []Movement{{Kind: CashOut, Amount: inr(250000)}}, inr(-50000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Drawer(float, tt.moves)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Drawer = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDrawerRefusals(t *testing.T) {
	if _, err := Drawer(Count{Kind: CountClosing, Total: inr(0)}, nil); err == nil {
		t.Error("a closing count as the float: want an error")
	}
	float := Count{Kind: CountOpening, Total: inr(0)}
	if _, err := Drawer(float, []Movement{{Kind: CashIn, Amount: money.Amount{Minor: 1, Currency: "USD"}}}); !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Errorf("a dollar in a rupee drawer: err = %v, want ErrCurrencyMismatch", err)
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

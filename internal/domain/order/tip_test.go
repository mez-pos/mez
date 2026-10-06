package order

import (
	"errors"
	"math"
	"testing"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// The ceiling is the payment's share at the venue's percent, rounded up to
// the next minor unit: a tip at the ceiling never needs a manager because the
// division lost a paisa (ADR-261).
func TestTipCeiling(t *testing.T) {
	tests := []struct {
		name    string
		amount  money.Amount
		percent int64
		want    int64
		err     error
	}{
		{"half of a round bill", inr(20000), 50, 10000, nil},
		{"rounds up, never down", inr(333), 50, 167, nil},
		{"a third of a paisa is a paisa", inr(1), 1, 1, nil},
		{"exact needs no rounding", inr(1000), 10, 100, nil},
		{"zero percent is no tip at all", inr(20000), 0, 0, nil},
		{"the most a venue may allow", inr(20000), 1000, 200000, nil},
		{"nothing paid has no room for a tip", inr(0), 50, 0, nil},
		{"a negative percent is refused", inr(20000), -1, 0, ErrInvalidPayload},
		{"a negative amount is refused", inr(-1), 50, 0, ErrInvalidPayload},
		{"an amount too large to scale overflows", inr(math.MaxInt64 / 10), 1000, 0, money.ErrOverflow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TipCeiling(tt.amount, tt.percent)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if err != nil {
				return
			}
			if got != inr(tt.want) {
				t.Errorf("ceiling = %+v, want %d INR", got, tt.want)
			}
		})
	}
}

func TestTipNeedsApproval(t *testing.T) {
	tests := []struct {
		name    string
		tip     money.Amount
		amount  money.Amount
		percent int64
		want    bool
		err     error
	}{
		{"no tip", inr(0), inr(20000), 0, false, nil},
		{"a tip with no currency is no tip", money.Amount{}, inr(20000), 0, false, nil},
		{"under the ceiling", inr(9999), inr(20000), 50, false, nil},
		{"at the ceiling", inr(10000), inr(20000), 50, false, nil},
		{"one paisa over", inr(10001), inr(20000), 50, true, nil},
		{"at a rounded ceiling", inr(167), inr(333), 50, false, nil},
		{"over a rounded ceiling", inr(168), inr(333), 50, true, nil},
		{"any tip at zero percent", inr(1), inr(20000), 0, true, nil},
		{"5000 rupees on 200", inr(500000), inr(20000), 50, true, nil},
		{"another currency is refused", money.Amount{Minor: 100, Currency: "USD"}, inr(20000), 50, false, money.ErrCurrencyMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TipNeedsApproval(tt.tip, tt.amount, tt.percent)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if got != tt.want {
				t.Errorf("needs approval = %v, want %v", got, tt.want)
			}
		})
	}
}

// Who approved a tip over the ceiling is on the tender for as long as the
// order lasts, and a tender that needed nobody says nobody.
func TestATenderKeepsWhoApprovedItsTip(t *testing.T) {
	approved := pay(PaymentCard, 10000)
	approved.Tip, approved.AuthorisedBy = inr(9000), 7
	s := mustProject(t, opened(), dish("Masala dosa", 1, 12000), approved, pay(PaymentCash, 2000))
	if len(s.Tenders) != 2 || s.Tenders[0].AuthorisedBy != 7 || s.Tenders[1].AuthorisedBy != 0 {
		t.Errorf("tenders = %+v", s.Tenders)
	}
}

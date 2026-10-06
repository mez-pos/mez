package order

import (
	"fmt"
	"math"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// The tip ceiling (ADR-261). A tip is kept apart from the bill and paid out
// to staff through the pool, and a card or UPI tip is never counted against
// the drawer, so a tip far out of proportion to the payment is a manager's to
// let through. The venue sets the ceiling as a whole percent of the payment;
// the service enforces it, since the projection does not know the venue's
// rules.

// TipCeiling is the largest tip a payment of amount may carry without a
// manager at percent: amount × percent / 100, rounded up to the next minor
// unit, so a tip at the ceiling never needs approval because a division lost
// a paisa. Integer arithmetic only.
func TipCeiling(amount money.Amount, percent int64) (money.Amount, error) {
	if percent < 0 {
		return money.Amount{}, invalid("tip ceiling of %d%%", percent)
	}
	if amount.Minor < 0 {
		return money.Amount{}, invalid("tip ceiling on a payment of %d", amount.Minor)
	}
	if percent > 0 && amount.Minor > (math.MaxInt64-99)/percent {
		return money.Amount{}, fmt.Errorf("%w: %d at %d%%", money.ErrOverflow, amount.Minor, percent)
	}
	return money.Amount{Minor: (amount.Minor*percent + 99) / 100, Currency: amount.Currency}, nil
}

// TipNeedsApproval reports whether a tip on a payment of amount is over the
// ceiling at percent. No tip never needs one; at zero percent any tip does.
func TipNeedsApproval(tip, amount money.Amount, percent int64) (bool, error) {
	if tip.IsZero() {
		return false, nil
	}
	ceiling, err := TipCeiling(amount, percent)
	if err != nil {
		return false, err
	}
	over, err := tip.Compare(ceiling)
	if err != nil {
		return false, fmt.Errorf("tip against its payment: %w", err)
	}
	return over > 0, nil
}

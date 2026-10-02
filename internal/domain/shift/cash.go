package shift

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// MovementKind is which way cash went, outside a sale.
type MovementKind string

// The ways cash moves outside a sale.
const (
	CashIn  MovementKind = "cash_in"  // change brought in, a float top-up
	CashOut MovementKind = "cash_out" // a payout: milk from the shop, a drop to the safe
)

// MovementKinds is every kind, for the generated vocabulary.
func MovementKinds() []MovementKind { return []MovementKind{CashIn, CashOut} }

// MaxReasonLength bounds a movement's reason: a line on a Z-report, not a
// letter.
const MaxReasonLength = 120

// ErrInvalidMovement refuses a cash movement missing what makes it
// accountable: an amount, a reason, and who authorised it.
var ErrInvalidMovement = errors.New("invalid cash movement")

// Movement is cash put into or taken out of the drawer other than by a
// sale. Every one names why and the manager who allowed it, so the count
// at close can be explained line by line.
type Movement struct {
	Kind         MovementKind `json:"kind"`
	Amount       money.Amount `json:"amount"`
	Reason       string       `json:"reason"`
	AuthorisedBy int64        `json:"authorised_by"`
}

// NewMovement checks and tidies a movement: the reason trimmed, the amount
// in the venue's currency and more than nothing, an authorising staff
// member named.
func NewMovement(kind MovementKind, amount money.Amount, reason string, authorisedBy int64, currency string) (Movement, error) {
	m := Movement{Kind: kind, Amount: amount, Reason: strings.TrimSpace(reason), AuthorisedBy: authorisedBy}
	return m, m.Validate(currency)
}

// Validate checks a movement as stored or received.
func (m Movement) Validate(currency string) error {
	switch {
	case !slices.Contains(MovementKinds(), m.Kind):
		return fmt.Errorf("%w: kind %q", ErrInvalidMovement, m.Kind)
	case m.Amount.Currency != currency:
		return fmt.Errorf("%w: moved in %q, the venue sells in %q", ErrInvalidMovement, m.Amount.Currency, currency)
	case m.Amount.Minor <= 0:
		return fmt.Errorf("%w: the amount must be more than nothing", ErrInvalidMovement)
	case m.Reason == "" || m.Reason != strings.TrimSpace(m.Reason):
		return fmt.Errorf("%w: a reason is required", ErrInvalidMovement)
	case utf8.RuneCountInString(m.Reason) > MaxReasonLength:
		return fmt.Errorf("%w: the reason is over %d characters", ErrInvalidMovement, MaxReasonLength)
	case m.AuthorisedBy < 1:
		return fmt.Errorf("%w: a manager must authorise it", ErrInvalidMovement)
	}
	return nil
}

// Signed is the movement's effect on the drawer: positive in, negative out.
func (m Movement) Signed() (money.Amount, error) {
	if m.Kind == CashOut {
		return m.Amount.Neg()
	}
	return m.Amount, nil
}

// Drawer is what should be in the drawer from the float and the movements
// alone: the float, plus cash in, less cash out. Cash taken and refunded at
// the till is added to it by the close (Task 2); it is not known here.
func Drawer(float Count, movements []Movement) (money.Amount, error) {
	if float.Kind != CountOpening {
		return money.Amount{}, fmt.Errorf("shift: the drawer starts from the opening count, not a %s count", float.Kind)
	}
	balance := float.Total
	for _, m := range movements {
		signed, err := m.Signed()
		if err != nil {
			return money.Amount{}, fmt.Errorf("shift: %s of %s: %w", m.Kind, m.Amount, err)
		}
		if balance, err = balance.Add(signed); err != nil {
			return money.Amount{}, fmt.Errorf("shift: adding %s of %s: %w", m.Kind, m.Amount, err)
		}
	}
	return balance, nil
}

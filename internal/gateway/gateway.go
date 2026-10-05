// Package gateway is the one shape every payment gateway is used through
// (phase 11, Task 4f, ADR-220): Razorpay, Cashfree, Stripe or any other.
//
// A gateway takes a guest's money on its own page and can be asked whether
// it did. That is all mez needs of one, and all this interface has: start a
// charge and get somewhere to send the guest, and ask how a charge stands.
// Everything about a provider (its API, its keys, how it names things) is
// inside its adapter. Nothing outside an adapter names a provider.
//
// A gateway is the one part of mez that needs the internet, and it is only
// ever for a guest paying from a phone. The till never touches it.
package gateway

import (
	"context"
	"errors"

	"github.com/byjoellobo/mez/internal/domain/money"
)

// ErrUnavailable means the gateway could not be asked: no network, a
// timeout, or the provider answering with a failure of its own. Nothing is
// known about any charge from it, and asking again later is right.
var ErrUnavailable = errors.New("gateway: could not reach the payment provider")

// Gateway is a payment provider.
type Gateway interface {
	// Name is the provider's name as it is stored: "razorpay", "test".
	Name() string

	// Start opens a charge at the provider and answers where to send the
	// guest to pay it. It takes no money.
	Start(ctx context.Context, c Charge) (Checkout, error)

	// Status asks the provider how a charge it started stands. It is the
	// only thing that says a charge was paid: nothing a guest's browser
	// carries back is believed.
	Status(ctx context.Context, id string) (Status, error)
}

// Charge is money to be asked for.
type Charge struct {
	// Reference is mez's own id for the charge, "chg_7K3M9QX2": what the
	// provider is told to call it, so its dashboard and ours can be matched.
	Reference string
	Amount    money.Amount
	// Description is what the guest sees they are paying for: "Table T5".
	Description string
	// ReturnURL is where the provider sends the guest's browser afterwards.
	// It says nothing of whether they paid.
	ReturnURL string
}

// Checkout is a charge opened at the provider.
type Checkout struct {
	// ID is the provider's id for it, which Status takes.
	ID string
	// URL is the provider's page where the guest pays.
	URL string
}

// State is how a charge stands at the provider.
type State string

// The states.
const (
	// Pending: opened, and not yet paid or given up on.
	Pending State = "pending"
	// Paid: the provider has the money.
	Paid State = "paid"
	// Failed: it will not be paid: declined, cancelled or expired.
	Failed State = "failed"
)

// Method is how a guest paid, as far as the bill needs to know.
type Method string

// The methods. Anything a provider offers that is neither is Other.
const (
	UPI   Method = "upi"
	Card  Method = "card"
	Other Method = "other"
)

// Status is what the provider says of a charge.
type Status struct {
	State State
	// What follows is set when State is Paid.

	// Amount is what the provider took, which must be what was asked.
	Amount money.Amount
	Method Method
	// Reference is the provider's id for the payment itself, the one a
	// refund or a dispute names.
	Reference string
}

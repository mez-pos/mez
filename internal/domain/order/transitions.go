package order

import "github.com/byjoellobo/nofiltr-pos/internal/domain/fsm"

// The order and payment lifecycles from docs/02-domain.md, as transition
// tables. Project checks every status change it derives against them, so the
// projection and the tables cannot drift apart. ADR-047.

// orderMachine is
//
//	open -> (held) -> open -> settling -> closed -> (reopened) -> closed
//
// plus two moves the projection produces from real events: settling -> open
// when the only live tender is voided or refunded, and open -> closed for an
// order that settles at zero with no tender (comped in full, or abandoned
// before anything was added).
var orderMachine = fsm.Must("order",
	[]Status{StatusOpen, StatusHeld, StatusSettling, StatusClosed, StatusReopened},
	map[Status][]Status{
		StatusOpen:     {StatusHeld, StatusSettling, StatusClosed},
		StatusHeld:     {StatusOpen},
		StatusSettling: {StatusOpen, StatusClosed},
		StatusClosed:   {StatusReopened},
		StatusReopened: {StatusClosed},
	})

// paymentMachine is
//
//	pending -> captured -> (voided | refunded)
//
// A partial refund leaves a payment captured; only a refund of the whole
// amount moves it to refunded.
var paymentMachine = fsm.Must("payment",
	[]PaymentStatus{PaymentStatusPending, PaymentStatusCaptured, PaymentStatusVoided, PaymentStatusRefunded},
	map[PaymentStatus][]PaymentStatus{
		PaymentStatusPending:  {PaymentStatusCaptured},
		PaymentStatusCaptured: {PaymentStatusVoided, PaymentStatusRefunded},
	})

// TransitionOrder checks an order status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func TransitionOrder(from, to Status) (Status, error) {
	return orderMachine.Transition(from, to)
}

// TransitionPayment checks a payment status change. A refusal unwraps to
// fsm.ErrInvalidTransition.
func TransitionPayment(from, to PaymentStatus) (PaymentStatus, error) {
	return paymentMachine.Transition(from, to)
}

// OrderMachine is the order lifecycle, for callers that need to list the
// moves open to an order, such as a terminal deciding which buttons to show.
func OrderMachine() fsm.Machine[Status] { return orderMachine }

// PaymentMachine is the payment lifecycle.
func PaymentMachine() fsm.Machine[PaymentStatus] { return paymentMachine }

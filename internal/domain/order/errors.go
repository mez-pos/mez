package order

import (
	"errors"
	"fmt"
)

// The sentinel errors of this package. Callers match with errors.Is and map
// them to API error codes; they never match on message text.
var (
	// ErrUnknownEventType means a stored event names a type this build does
	// not know. Every such error also carries the type and seq as an
	// *UnknownEventTypeError.
	//
	// It is an error and never a skip. An order whose events were written by
	// a newer build cannot be projected correctly by an older one, and a
	// projection that quietly drops a payment or a void produces a total that
	// looks right and is not.
	ErrUnknownEventType = errors.New("unknown event type")

	// ErrMalformedEvent means an event could not be encoded or decoded: the
	// envelope has no type or payload, the payload is not a JSON object, a
	// field holds the wrong kind of value, or the envelope's type disagrees
	// with its payload.
	ErrMalformedEvent = errors.New("malformed event")
)

// UnknownEventTypeError reports which type turned up, and where. The seq is
// what an operator needs to find the row in order_events.
type UnknownEventTypeError struct {
	Type Type  // the type string as stored
	Seq  int64 // the event's position in its order
}

func (e *UnknownEventTypeError) Error() string {
	return fmt.Sprintf("unknown event type %q at seq %d", e.Type, e.Seq)
}

// Unwrap lets errors.Is(err, ErrUnknownEventType) succeed while errors.As
// still recovers the type and seq.
func (e *UnknownEventTypeError) Unwrap() error { return ErrUnknownEventType }

// malformed wraps a cause as ErrMalformedEvent, keeping the cause readable.
func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformedEvent, fmt.Sprintf(format, args...))
}

// The errors Project returns for a stream that cannot have happened. Every
// one reaches the caller inside a *ProjectionError naming the event at fault,
// and still matches with errors.Is.
var (
	// ErrSeqGap means the events are not numbered 1, 2, 3 and so on with no
	// gap or repeat. A gap means an event is missing, and a projection of an
	// incomplete stream is a wrong total.
	ErrSeqGap = errors.New("event seq is not dense from 1")

	// ErrNotOpened means the stream is empty or does not start with
	// order.opened.
	ErrNotOpened = errors.New("order was never opened")

	// ErrAlreadyOpened means a second order.opened turned up.
	ErrAlreadyOpened = errors.New("order is already open")

	// ErrOrderClosed means an event that changes the bill arrived while the
	// order was closed. Reopen it first; that is what leaves a trail.
	ErrOrderClosed = errors.New("order is closed")

	// ErrOrderNotClosed means order.reopened arrived for an order that was
	// not closed.
	ErrOrderNotClosed = errors.New("order is not closed")

	// ErrLineNotFound means an event named a line ID that no line.added
	// created.
	ErrLineNotFound = errors.New("no such line")

	// ErrLineNotActive means an event tried to change a line that is already
	// voided or comped.
	ErrLineNotActive = errors.New("line is voided or comped")

	// ErrQtyMismatch means a line.qty_changed claimed a From quantity the
	// line does not have: the client was looking at a stale ticket.
	ErrQtyMismatch = errors.New("quantity changed from a value the line does not have")

	// ErrPaymentNotFound means an event named a payment ID that no
	// payment.tendered created.
	ErrPaymentNotFound = errors.New("no such payment")

	// ErrPaymentNotCaptured means a void or refund named a payment that is
	// already voided, fully refunded, or (for a void) partly refunded.
	ErrPaymentNotCaptured = errors.New("payment is not in a state that allows this")

	// ErrInvalidPayload means a payload is well formed JSON but not a
	// possible event: a zero quantity, a negative price, a void with no
	// authoriser, an unknown channel or payment method, and the like.
	ErrInvalidPayload = errors.New("invalid event payload")

	// ErrDiscountExceedsValue means the discounts on a line, or on the
	// order, add up to more than what they are discounting.
	ErrDiscountExceedsValue = errors.New("discount exceeds the value it applies to")

	// ErrOverTender means a tender is larger than the balance due. Change
	// given for cash is not part of the order, so a tender records only what
	// settles the bill.
	ErrOverTender = errors.New("tender exceeds the balance due")

	// ErrOverRefund means a refund is larger than what the payment has left
	// to refund, or larger than the credit the order owes the guest.
	ErrOverRefund = errors.New("refund exceeds what is owed back")

	// ErrBalanceDue means an order was closed or invoiced with a balance
	// that is not exactly zero.
	ErrBalanceDue = errors.New("balance due is not zero")

	// ErrAlreadyInvoiced means a second invoice was issued without a reopen
	// in between.
	ErrAlreadyInvoiced = errors.New("order is already invoiced")

	// ErrCourseAlreadyFired means a course was fired twice.
	ErrCourseAlreadyFired = errors.New("course is already fired")

	// ErrTaxConfig means the venue's tax configuration cannot apply to the
	// order, such as a venue taxed in rupees and an order in dollars.
	ErrTaxConfig = errors.New("tax configuration does not fit the order")
)

// ProjectionError says which event made a stream impossible. The seq and
// type are what an operator needs to find it in order_events, and what a
// handler needs to reject the one event a client just tried to append.
type ProjectionError struct {
	Seq  int64 // the event at fault
	Type Type  // its type
	Err  error // the reason, one of the sentinels above or a money error
}

func (e *ProjectionError) Error() string {
	return fmt.Sprintf("projecting %s at seq %d: %v", e.Type, e.Seq, e.Err)
}

// Unwrap lets errors.Is match the sentinel inside.
func (e *ProjectionError) Unwrap() error { return e.Err }

// invalid wraps a reason as ErrInvalidPayload.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPayload, fmt.Sprintf(format, args...))
}

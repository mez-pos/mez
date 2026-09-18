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

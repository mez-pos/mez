package order

import (
	"bytes"
	"encoding/json"
	"slices"
	"time"
)

// Type names an event. The string is what is stored in order_events.type, so
// a constant here is never renamed once it has shipped.
type Type string

// The event types of docs/02-domain.md, one per row of its table.
const (
	TypeOrderOpened      Type = "order.opened"
	TypeLineAdded        Type = "line.added"
	TypeLineQtyChanged   Type = "line.qty_changed"
	TypeLineModified     Type = "line.modified"
	TypeLineVoided       Type = "line.voided"
	TypeLineComped       Type = "line.comped"
	TypeDiscountApplied  Type = "discount.applied"
	TypeNoteAdded        Type = "note.added"
	TypeCourseFired      Type = "course.fired"
	TypeTicketSent       Type = "ticket.sent"
	TypePaymentTendered  Type = "payment.tendered"
	TypePaymentVoided    Type = "payment.voided"
	TypeInvoiceIssued    Type = "invoice.issued"
	TypeRefundIssued     Type = "refund.issued"
	TypeOrderClosed      Type = "order.closed"
	TypeOrderReopened    Type = "order.reopened"
	TypeOrderHeld        Type = "order.held"
	TypeOrderResumed     Type = "order.resumed"
	TypeInvoiceReprinted Type = "invoice.reprinted"
)

// Payload is the type-specific body of an event. Each payload type knows its
// own Type, so an Event cannot be built with a body that disagrees with it.
type Payload interface {
	EventType() Type
}

// decoders maps every known type to the function that decodes its payload.
// It is the single list of what this build understands: a type missing here
// is an unknown type on read, by construction.
var decoders = map[Type]func(json.RawMessage) (Payload, error){
	TypeOrderOpened:      decodeAs[OrderOpened],
	TypeLineAdded:        decodeAs[LineAdded],
	TypeLineQtyChanged:   decodeAs[LineQtyChanged],
	TypeLineModified:     decodeAs[LineModified],
	TypeLineVoided:       decodeAs[LineVoided],
	TypeLineComped:       decodeAs[LineComped],
	TypeDiscountApplied:  decodeAs[DiscountApplied],
	TypeNoteAdded:        decodeAs[NoteAdded],
	TypeCourseFired:      decodeAs[CourseFired],
	TypeTicketSent:       decodeAs[TicketSent],
	TypePaymentTendered:  decodeAs[PaymentTendered],
	TypePaymentVoided:    decodeAs[PaymentVoided],
	TypeInvoiceIssued:    decodeAs[InvoiceIssued],
	TypeRefundIssued:     decodeAs[RefundIssued],
	TypeOrderClosed:      decodeAs[OrderClosed],
	TypeOrderReopened:    decodeAs[OrderReopened],
	TypeOrderHeld:        decodeAs[OrderHeld],
	TypeOrderResumed:     decodeAs[OrderResumed],
	TypeInvoiceReprinted: decodeAs[InvoiceReprinted],
}

// Types lists every event type this build knows, sorted, for tests and for
// anything that must enumerate them.
func Types() []Type {
	out := make([]Type, 0, len(decoders))
	for t := range decoders {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

// Event is one entry in an order's append-only stream: the envelope every
// event shares, and its payload. It mirrors a row of order_events.
//
// Events are values. Nothing in the system edits one after it is appended;
// a correction is a later event.
type Event struct {
	// Seq is the event's position in its order, dense and starting at 1.
	// It doubles as the ID of whatever the event creates: the line a
	// line.added adds is line Seq, the payment a payment.tendered records
	// is payment Seq. See ADR-045.
	Seq int64

	// Type names the payload. It always equals Payload.EventType().
	Type Type

	// At is when the event happened, in UTC. It is the only clock the
	// projection ever sees.
	At time.Time

	// ActorID is the staff member who caused the event, or 0 when no staff
	// member did, as for an order placed by a guest from a table QR.
	ActorID int64

	// DeviceID is the terminal, kitchen display or guest session that
	// recorded the event.
	DeviceID int64

	// Payload is the type-specific body.
	Payload Payload
}

// NewEvent builds an event whose Type is taken from its payload and whose
// time is normalised to UTC, so the two can never disagree.
func NewEvent(seq int64, at time.Time, actorID, deviceID int64, p Payload) Event {
	e := Event{Seq: seq, At: at.UTC(), ActorID: actorID, DeviceID: deviceID, Payload: p}
	if p != nil {
		e.Type = p.EventType()
	}
	return e
}

// wireEvent is the JSON shape of an Event. The payload stays raw until the
// type has been read, because the type decides what to decode it into.
type wireEvent struct {
	Seq      int64           `json:"seq"`
	Type     Type            `json:"type"`
	At       time.Time       `json:"at"`
	ActorID  int64           `json:"actor_id"`
	DeviceID int64           `json:"device_id"`
	Payload  json.RawMessage `json:"payload"`
}

// MarshalJSON writes the envelope with its payload nested under "payload".
// It refuses an event with no payload, or one whose Type disagrees with its
// payload, rather than storing a row that cannot be read back.
func (e Event) MarshalJSON() ([]byte, error) {
	if e.Payload == nil {
		return nil, malformed("event at seq %d has no payload", e.Seq)
	}
	if got := e.Payload.EventType(); e.Type != got {
		return nil, malformed("event at seq %d has type %q but a %q payload", e.Seq, e.Type, got)
	}
	body, err := json.Marshal(e.Payload)
	if err != nil {
		return nil, malformed("encoding %q payload at seq %d: %v", e.Type, e.Seq, err)
	}
	return json.Marshal(wireEvent{
		Seq:      e.Seq,
		Type:     e.Type,
		At:       e.At.UTC(),
		ActorID:  e.ActorID,
		DeviceID: e.DeviceID,
		Payload:  body,
	})
}

// UnmarshalJSON reads an envelope and decodes its payload into the concrete
// type its Type names.
//
// An unknown type is an *UnknownEventTypeError, never a skip. Unknown fields
// inside a known payload are ignored: a newer build may add an optional
// field, and an older reader must still be able to open the order. Anything
// that changes what an order is worth is a new event type, not a new field,
// so ignoring a field never changes a total. See ADR-045.
func (e *Event) UnmarshalJSON(data []byte) error {
	var w wireEvent
	if err := json.Unmarshal(data, &w); err != nil {
		return malformed("decoding envelope: %v", err)
	}
	if w.Type == "" {
		return malformed("event at seq %d has no type", w.Seq)
	}
	p, err := decodePayload(w.Type, w.Seq, w.Payload)
	if err != nil {
		return err
	}
	*e = Event{
		Seq:      w.Seq,
		Type:     w.Type,
		At:       w.At.UTC(),
		ActorID:  w.ActorID,
		DeviceID: w.DeviceID,
		Payload:  p,
	}
	return nil
}

// DecodePayload decodes a stored payload into the concrete type t names. It
// is how a database row, which keeps the envelope in columns and only the
// payload as JSON, becomes an Event. The rules are UnmarshalJSON's: an
// unknown type is an *UnknownEventTypeError and anything but a JSON object is
// ErrMalformedEvent.
func DecodePayload(t Type, raw json.RawMessage) (Payload, error) {
	return decodePayload(t, 0, raw)
}

func decodePayload(t Type, seq int64, raw json.RawMessage) (Payload, error) {
	decode, ok := decoders[t]
	if !ok {
		return nil, &UnknownEventTypeError{Type: t, Seq: seq}
	}
	if !isObject(raw) {
		return nil, malformed("%q event at seq %d has no payload object", t, seq)
	}
	p, err := decode(raw)
	if err != nil {
		return nil, malformed("decoding %q payload at seq %d: %v", t, seq, err)
	}
	return p, nil
}

// decodeAs decodes a raw payload into P. Payloads are held as values, not
// pointers, so a decoded event is as immutable as a built one.
func decodeAs[P Payload](raw json.RawMessage) (Payload, error) {
	var p P
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return p, nil
}

// isObject reports whether raw is a JSON object. A missing payload, null, or
// an array would otherwise decode into an empty struct without complaint.
func isObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

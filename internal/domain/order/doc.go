// Package order is the order as an append-only stream of events.
//
// An order is never a row that gets updated. It is a list of things that
// happened to it, in order: opened, a line added, a line voided, a payment
// taken. The current state is computed from that list. docs/02-domain.md is
// the rule; this package is the implementation of it.
//
// This file set defines the events themselves: the Event envelope every event
// shares, one payload type per row of the domain table, and their JSON form,
// which is what order_events.payload_json stores.
//
// # Reading is strict about types, lenient about fields
//
// An event whose type this build does not know fails to decode with an
// *UnknownEventTypeError. It is never skipped, because skipping a payment
// changes the balance. An unknown field inside a known payload is ignored, so
// that a newer build can add an optional field without making old orders
// unreadable to an older one. The price of that leniency is a rule: anything
// that changes what an order is worth must be a new event type. See ADR-045.
//
// The package is pure. It has no clock and no database: an event's time is
// a field, set by whoever records it.
package order

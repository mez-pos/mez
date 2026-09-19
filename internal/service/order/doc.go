// Package order is the order service: the one path from a command to events
// on disk.
//
// Every command runs the same way, inside one write transaction: load the
// order's stream, project it, decide the command's event, project again with
// it on the end (the domain's check), append, and commit. Only then is the
// change published to the hub. Handlers decode, call one method here, and
// encode; they hold no order logic.
//
// A command carrying an idempotency key that was already applied returns the
// earlier result and appends nothing.
//
// The package shares its name with internal/domain/order, whose types it
// speaks in. A caller importing both names one of them.
//
// Imports: internal/db and internal/domain. Never internal/api.
package order

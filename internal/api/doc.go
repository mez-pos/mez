// Package api holds the HTTP handlers and the request and response types.
//
// Handlers decode, validate, call into domain or a service, then encode. No
// business logic lives here, ever. Mutating endpoints honour Idempotency-Key.
//
// Imports: internal/domain, internal/db, internal/hub, internal/config.
// Nothing imports this package back; it is a top edge of the graph.
package api

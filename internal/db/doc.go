// Package db owns the SQLite connection, the goose migrations and the
// sqlc-generated query layer.
//
// # Two pools
//
// SQLite has one writer. Open returns a writer capped at a single connection
// and a separate read pool, so writes queue in Go, where the wait is orderly,
// rather than in SQLite, where it surfaces as a busy error in the middle of a
// sale. The read pool is opened read only at the driver, which turns "do not
// write through the reader" from a convention everybody has to remember into
// an error SQLite raises.
//
// # Settings
//
// WAL, foreign_keys, busy_timeout and synchronous=NORMAL are set in the DSN,
// not by a PRAGMA after opening. All but journal_mode are per connection, and
// database/sql opens connections whenever the pool decides to, so a pragma run
// once protects the first connection and leaves every later one unguarded.
//
// # Transactions
//
// Every write that matters goes through Tx, which commits on nil and rolls
// back on anything else, panics included. Invoice numbers, payments and shift
// closes especially: docs/02-domain.md explains what a half-committed one
// costs.
//
// # Health
//
// A goroutine owned by DB probes the database on a timer and caches the
// answer, which Health returns without doing any IO. The HTTP handler must be
// able to answer while the database is hung, because whatever is asking is
// usually asking because something already has.
//
// Imports: internal/domain, for the types it persists. Never internal/api,
// internal/hub or any transport package. Data flows up to them, not down to here.
package db

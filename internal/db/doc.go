// Package db owns the SQLite connection, the goose migrations and the
// sqlc-generated query layer.
//
// Every multi-statement write is an explicit transaction with a deferred
// rollback. Invoice numbers, payments and shift closes especially.
//
// Imports: internal/domain, for the types it persists. Never internal/api,
// internal/hub or any transport package. Data flows up to them, not down to here.
package db

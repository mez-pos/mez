// Package service is the home of the application services: one package per
// capability (order, menus, shift, stock, settings, ...), each sitting between
// internal/api and the domain and database.
//
// A service method is one use case. It takes ctx first, loads what it needs
// through internal/db, applies the rules in internal/domain, and writes in one
// transaction. Handlers decode, call one method and encode; they hold no
// business logic (docs/03-conventions.md).
//
// The Windows Service wrapper that phase 12 adds is not here: it lives in
// internal/winsvc (ADR-142).
package service

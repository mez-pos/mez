// Package logging builds the process logger: structured slog to stdout and to
// a rotating file under the data directory, at one level and in one format.
//
// It exists as a package rather than as part of cmd/nofiltr because the Windows
// Service wrapper in Phase 12 cannot import a main package and needs the same
// logger. See ADR-020.
//
// Imports: internal/config and the standard library. Nothing else, and nothing
// imports it back except the entrypoints.
package logging

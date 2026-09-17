// Package service wraps the server as a Windows Service so the venue PC runs it
// without anyone opening a terminal.
//
// Install, start, stop and restart on crash. The staff-facing equivalent of
// systemd, for a machine behind a counter.
//
// Imports: internal/config and the server type from internal/api.
package service

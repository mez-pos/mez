// Package winsvc runs the server as a Windows service: registered with the
// Service Control Manager to start with the computer, restarted if it dies,
// and stopped through the same drain a Ctrl-C gets.
//
// It wraps github.com/kardianos/service and is the only package that imports
// it (ADR-142, ADR-263). The server itself is handed in as a function, since
// a main package cannot be imported; cmd/mez owns the subcommands.
//
// Imports: the service library and the standard library. Nothing imports it
// except the entrypoint.
package winsvc

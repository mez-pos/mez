// Package domain holds the business rules: order events, projections, money,
// tax and the invoice series.
//
// Pure Go. It is the part of this system where a bug costs a real cafe real
// money, so it is kept trivially testable: no IO, no clock, no globals. Time
// arrives through a Clock interface, never time.Now.
//
// Imports: standard library only, and nothing that implies transport or
// storage. Never internal/db, internal/api, net/http or database/sql. This is
// enforced by TestDomainImportsStayPure in imports_test.go, not by good
// intentions.
package domain

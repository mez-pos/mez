# Phase 03 — Domain core

## Goal
The order event model, the projection, the invoice series and the tax profiles —
as pure Go, with no HTTP and no SQL. This is the highest-stakes phase in the
project.

## Depends on
Phase 02. **Read `docs/02-domain.md` in full before every task in this phase.**

## Tasks

- [ ] **Task 1 — Event types.**
  `internal/domain/order`. Every event from the table in `docs/02-domain.md` as
  a Go type, plus an `Event` envelope (`Seq`, `Type`, `At`, `ActorID`,
  `DeviceID`, payload). JSON marshal/unmarshal round-trip tests for every type.
  An unknown event type on read returns a typed error rather than silently
  dropping — a forward-compatibility guard.

- [ ] **Task 2 — The projection.**
  `Project(events []Event) (State, error)`. Pure: no clock, no DB. `State` holds
  lines with their price snapshots, modifiers, discounts, voids, course and
  station, subtotals, tax, tenders, balance due and order status. Illegal
  sequences return typed errors. Table-driven tests covering every event type
  and at least ten realistic full-order sequences including a void after
  payment, a comp, a reopen and a partial refund.

- [ ] **Task 3 — State machines.**
  Order, table, station-ticket and payment transitions from `docs/02-domain.md`,
  as explicit transition functions returning `ErrInvalidTransition`. Exhaustive
  tests: every legal transition passes, a sample of illegal ones fails.

- [ ] **Task 4 — Invoice series.**
  `internal/domain/invoice`. Series config (prefix, financial-year rule, format
  template). Allocation is an interface the DB layer implements; the domain
  defines the contract and the format. Financial-year boundary logic with the
  India 1-April case and a calendar-year case. Tests for year rollover,
  format templating, and the gapless guarantee expressed as a property.

- [ ] **Task 5 — Tax profiles.**
  `internal/domain/tax`. The `TaxProfile` interface from `docs/02-domain.md`.
  Implement **GST (India)**: CGST/SGST vs IGST, HSN per item, inclusive and
  exclusive pricing, line-level rounding then order-level reconciliation.
  Implement **VAT** and **US sales tax** as thinner profiles. Tests with real
  worked examples for each, including an inclusive-pricing GST case where the
  back-calculation must round to the paise.

- [ ] **Task 6 — Split payment.**
  `internal/domain/payment`. Split by amount, by item, by guest, and evenly.
  Every strategy asserts that tenders sum exactly to the total. Tips handled as
  a separate field, never folded into the tendered amount. Property test: for
  random totals and 2–8 ways, every strategy sums exactly.

## Done when
`go test ./internal/domain/...` passes with high coverage and no HTTP or SQL
imports anywhere under `internal/domain`. The imports test from Phase 00 still
passes.

## Notes
If the human is available, have them review the event list and the projection
before Task 3. A mistake here propagates through phases 06 to 10.

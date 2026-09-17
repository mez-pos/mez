# Phase 09 — Close of day

## Goal
The shift lifecycle: open, blind count, reconcile, Z-report, tips, export. This
is the phase that makes owners trust the software.

## Depends on
Phase 08.

## Tasks

- [ ] **Task 1 — Shift lifecycle.**
  Migration for `shifts`, `drawer_counts`, `shift_events`. Open a shift with a
  starting float. Every order and payment is attached to a shift. The trading
  day is the shift, not the calendar day — a bar closing at 2am belongs to the
  previous day. Cash movements in and out, with reason and authorisation.

- [ ] **Task 2 — Blind drawer count.**
  Staff enter counted denominations **before** the system reveals the expected
  figure. This is non-negotiable; showing expected first defeats the control.
  Variance computed after submission, over or short, with a mandatory reason
  above a configurable threshold. Recorded immutably.

- [ ] **Task 3 — Z-report.**
  Gross and net sales, by category, by item, by payment method, by channel, by
  staff. Discounts, comps, voids, refunds — each with counts and totals.
  Tax breakdown per rate. Cash reconciliation and variance. Immutable once
  generated, numbered, and a correction is a new document referencing the old.
  Print via the Phase 07 templates and view on screen.

- [ ] **Task 4 — Tips.**
  Per-payment tips aggregated to the shift. Pooling rules configurable: even
  split, by hours, by role weight. A tip report per staff member. Tips never
  touch the sales figures.

- [ ] **Task 5 — Exports.**
  CSV and Tally-compatible export of the tax register and the day's sales.
  Driven by `RegisterColumns()` from the tax profile, so a VAT venue exports
  VAT columns without a code change. Queued at close, re-downloadable from back
  office.

## Done when
A full trading day — open, sell across channels, discount, void, refund, close
with a blind count — produces a Z-report that reconciles to the paise, and an
export a bookkeeper could actually use.

## Notes
Have the human check the Z-report against a hand-computed day before moving on.
A Z-report that does not balance destroys trust permanently.

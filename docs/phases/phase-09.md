# Phase 09 — Close of day

## Goal
The shift lifecycle: open, blind count, reconcile, Z-report, tips, export. This
is the phase that makes owners trust the software.

## Depends on
Phase 08.

## Tasks

- [x] **Task 1 — Shift lifecycle.**
  Migration for `shifts`, `drawer_counts`, `shift_events`. Open a shift with a
  starting float. Every order and payment is attached to a shift. The trading
  day is the shift, not the calendar day — a bar closing at 2am belongs to the
  previous day. Cash movements in and out, with reason and authorisation.
  Split in two (one venue, one drawer, one open shift at a time):
  - [x] **1a - The shift itself.** Migration for `shifts`, `drawer_counts`,
    `shift_events`; `internal/domain/shift` (lifecycle, trading day, cash
    movements, the drawer's running balance); a shift service to open with a
    float, move cash with reason and authorising manager, and close.
  - [x] **1b - Everything attached to it.** Orders and payments carry the
    open shift; opening an order or taking a payment with no open shift is
    refused; the shift API, with `shift.manage` and `cash.move` permissions.

- [ ] **Task 2 — Blind drawer count.**
  Staff enter counted denominations **before** the system reveals the expected
  figure. This is non-negotiable; showing expected first defeats the control.
  Variance computed after submission, over or short, with a mandatory reason
  above a configurable threshold. Recorded immutably.
  Split in two:
  - [x] **2a - The count on the server.** Expected cash from the float,
    movements and the shift's cash tenders, voids and refunds; one closing
    count per shift, recorded with its expected figure, variance and the
    threshold then in force; a reason recorded once when the variance is
    over it; a counted shift takes no more sales or cash; close needs the
    count and, over the threshold, the reason. The API reveals expected only
    in the answer to the count.
  - [ ] **2b - The close-of-day screen.** Write
    `docs/design/screens/terminal-close.md` first. Open the shift with a
    float; count the drawer by denomination with nothing expected on
    screen; then the reveal, over or short, the reason when asked, and
    close.

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

# Phase 07 — Payment, invoice, print

## Goal
A ticket becomes money, a numbered invoice and a printed receipt. After this
phase the product is a real POS.

## Depends on
Phase 06.

## Tasks

- [ ] **Task 1 — Tender screen.**
  Spec first. Cash with quick-tender buttons and change due in large Instrument
  Serif, card-as-tender with a reference field, UPI. The charge button is 58px
  and unmistakable. Tips captured separately, never folded into the tender.
  Amount due always visible.

- [ ] **Task 2 — Splits.**
  By amount, by item, by guest, evenly. Driven by the Phase 03 split package —
  no arithmetic in the UI. Partial payment leaves the ticket open with a correct
  remaining balance. The three-way-split-of-an-odd-total case must be visibly
  correct, to the paise, on screen.

- [ ] **Task 3 — Invoice issue.**
  Full settlement allocates an invoice number inside the settling transaction
  and emits `invoice.issued` with a complete tax and price snapshot. Invoice
  preview on screen. Reprint is a separate, logged, permissioned action that
  never allocates a new number.

- [ ] **Task 4 — ESC/POS driver.**
  `internal/device/escpos`. Command builder: text, alignment, sizes, bold,
  barcode, QR, cut, drawer kick (`DLE DC4`). Layout templates for the customer
  receipt, the kitchen ticket and the Z-report. A `--test-print` flag on the
  binary so a cafe owner can verify their printer before service.

- [ ] **Task 5 — Durable print queue.**
  Jobs persisted in SQLite: queued → printing → done | failed. Retries with
  backoff, a dead-letter state, and a visible queue in the UI with a manual
  retry. Printers jam mid-service and a lost invoice is a compliance problem,
  not an annoyance. Transport: TCP:9100 primary, Windows RAW spooler fallback.
  Printer config in back office, assignable per station.

- [ ] **Task 6 — Refunds and voids after payment.**
  Full and partial refunds against a specific payment, with reason and
  authorisation. Post-payment voids. Both emit corrective events and appear on
  the timeline and in the Z-report. Never a destructive edit.

## Done when
A ticket is split three ways across cash, card and UPI, a gapless tax-compliant
invoice is issued, a receipt prints, the drawer kicks, and a partial refund
against one of those payments is correctly reflected everywhere.

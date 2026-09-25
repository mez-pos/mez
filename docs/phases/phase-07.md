# Phase 07 — Payment, invoice, print

## Goal
A ticket becomes money, a numbered invoice and a printed receipt. After this
phase the product is a real POS.

## Depends on
Phase 06.

## Tasks

- [x] **Task 1 — Tender screen.**
  Spec first. Cash with quick-tender buttons and change due in large Instrument
  Serif, card-as-tender with a reference field, UPI. The charge button is 58px
  and unmistakable. Tips captured separately, never folded into the tender.
  Amount due always visible.

- [x] **Task 2 — Splits.**
  By amount, by item, by guest, evenly. Driven by the Phase 03 split package —
  no arithmetic in the UI. Partial payment leaves the ticket open with a correct
  remaining balance. The three-way-split-of-an-odd-total case must be visibly
  correct, to the paise, on screen.

- [x] **Task 3 — Invoice issue.**
  Full settlement allocates an invoice number inside the settling transaction
  and emits `invoice.issued` with a complete tax and price snapshot. Invoice
  preview on screen. Reprint is a separate, logged, permissioned action that
  never allocates a new number.

- [x] **Task 4 — ESC/POS driver.**
  `internal/device/escpos`. Command builder: text, alignment, sizes, bold,
  barcode, QR, cut, drawer kick (`DLE DC4`). Layout templates for the customer
  receipt, the kitchen ticket and the Z-report. A `--test-print` flag on the
  binary so a cafe owner can verify their printer before service.

- [x] **Task 5 — Durable print queue.**
  Jobs persisted in SQLite: queued → printing → done | failed. Retries with
  backoff, a dead-letter state, and a visible queue in the UI with a manual
  retry. Printers jam mid-service and a lost invoice is a compliance problem,
  not an annoyance. Transport: TCP:9100 primary, Windows RAW spooler fallback.
  Printer config in back office, assignable per station.

  Split into three sub-tasks (too large for one session). Task 5 is done
  when all three are.

  - [x] **Task 5a - Queue core.** A migration for `printers` (name,
    transport tcp or windows, address or Windows printer name, paper 80 or
    58, active) and `print_jobs` (venue, printer, kind receipt, kitchen,
    z_report or test, the ESC/POS bytes, state, attempts, next attempt,
    last error, the order and event it came from). A job state machine on
    `internal/domain/fsm`: queued, printing, done, failed (will retry),
    dead (retries used up). A worker owned by a struct with `Close()`, one
    job at a time per printer, exponential backoff with a cap, and jobs
    left `printing` by a crash put back to queued on start. Transports: TCP
    9100 through `escpos.Send`, and the Windows RAW spooler through
    `winspool.drv` system calls (no cgo), built only on Windows and refused
    clearly elsewhere. Started and stopped with the server. Tests with a
    temp-file DB and a fake transport: retry, backoff, dead letter, the
    crash recovery.
  - [x] **Task 5b - Jobs from the till, and the queue on screen.** Jobs
    written in the same transaction as the event that causes them, so
    none is lost: the receipt when an invoice is issued (with the drawer
    kick when cash was taken), a kitchen ticket per station on
    `ticket.sent`, a DUPLICATE on `invoice.reprinted`. Which printer:
    per station for kitchen tickets, the venue's receipt printer for
    receipts, and a job with no printer configured is kept as `dead` with
    that reason, never dropped. API to list jobs, retry one (a dead job
    back to queued) and cancel one; the stream announces changes. On the
    terminal: a print-queue sheet with each job's state as a chip and a
    word, its error, and Retry; a header warning while any job is failed
    or dead.
  - [x] **Task 5c - Printers in the back office.** API to list, add,
    change and retire printers, assign one to each station and one as the
    receipt printer, and send a test page (`escpos.TestPage`) through the
    queue. The back office's first real screen: the printers list with
    each one's last result, an add and edit form, the station assignment,
    and the test button. Screen spec first.

- [ ] **Task 6 — Refunds and voids after payment.**
  Full and partial refunds against a specific payment, with reason and
  authorisation. Post-payment voids. Both emit corrective events and appear on
  the timeline and in the Z-report. Never a destructive edit.

## Done when
A ticket is split three ways across cash, card and UPI, a gapless tax-compliant
invoice is issued, a receipt prints, the drawer kicks, and a partial refund
against one of those payments is correctly reflected everywhere.

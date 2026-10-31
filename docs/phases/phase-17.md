# Phase 17 - Every screen at the venue's own size

## Goal
The fixes from the UI audit of 2026-10-31
(`docs/design/ui-audit-2026-10-31.md`, ADR-381). First the two bugs that
can cost money: a paid bill that still reads "Open", and a paid order
that never reached the kitchen. Then every screen fits the venue's real
devices (a 1366x768 counter PC, a 1024x768 tablet, a kitchen screen, a
guest's phone) without losing the content it is for. Then contrast,
words, the kitchen display's behaviour, the guest app, first run and the
shared kit.

## Depends on
Nothing. It runs before Phase 16, whose tasks are mostly `mez-cloud`'s.

## Where the code lives
This repo: `web/` almost entirely, `internal/` for the two P0s, the
close of day and the guest table response. A change in `web/shared` is a
change to `mez-cloud`'s account area too (STATE.md): build it there.

## What stays true
- The mez look in `docs/design/tokens.md` is binding (ADR-381). Fixes
  work inside it; a new value goes into tokens.md first.
- **At 768px tall, chrome gives way, text never does.** `short:` trims
  padding, gaps, bands and headers; type sizes stay. A side column takes
  `w-side`, `w-rail`, `w-panel` or a new token, never a fixed width.
- Every screen is checked by eye at 1024x768 and 1366x768 (and 1920x1080;
  390x844 for the guest) before its task is ticked. The audit's walk is
  the method: a demo venue, a scripted browser, a screenshot per size.
- Each finding's id (TILL-2, KDS-7...) is named in the commit that closes
  it. A screen's spec in `docs/design/screens/` changes in the same task.
- A reversal of a recorded decision is ADR-381's; nothing else is
  reversed without its own ADR.

## Tasks

- [x] **Task 1 - ADR-381, this phase, the venue's devices in `00-product.md`, and the audit.** Written
  2026-10-31.

### Money and state

- [x] **Task 2 - Paid means closed (TBS-1, P0).** Done 2026-10-31
  (ADR-383): the payment that settles a counter sale closes it; a table's
  ticket paid in full reads Paid. A payment that brings
  the balance to zero closes the order in the same transaction, or until
  then every reader treats a zero-balance `settling` order as paid: the
  Orders list and its counts, the order panel (a primary "Close order"),
  the table turn (To clear, not Paying), the waiter's home (not Needs
  Bill). New Order becomes "Next order". Go tests on the projection and
  the day orders; vitest on the readers.
- [x] **Task 3 - Nothing paid goes unsent (TILL-1, P0).** Done 2026-10-31
  (ADR-383): that same payment sends what waits; a table's unsent lines are
  a warning with Send now. Pay on a
  counter order sends what is waiting, then opens the tender screen; the
  tender foot and the paid panel say "2 items have not gone to the
  kitchen" with Send now while any line is unsent, and Next order asks
  before closing such an order. Lines with no station are not "unsent".

### Fit the venue's devices (started first, at the human's word)

Tasks 4 to 12 were built on 2026-10-31 by six workers in parallel and
seen by the audit's walk at 1024x768, 1366x768 and 1920x1080 before they
were ticked (ADR-381, ADR-382). What the walk did not reach is in
`KNOWN.md` under "Phase 17's device-fit pass".

- [x] **Task 4 - The shared frame.** Modal scrolls its body and keeps its
  footer in view (TECH-2; ApprovalSheet passes `footer`). Toasts in the
  terminal at top centre, the body pass-through (TILL-11). An `--aside`
  token for the back office's 380px column (TECH-17). The terminal
  shell's right cluster gives way below xl (TBS-3, the shell half).
- [x] **Task 5 - Register.** Four order lines in view at 1366x768
  (TILL-2); six dishes at 1024 (TILL-3); no photo pane without a photo
  (TILL-13, TECH-16); a charge label that fits, "Pay" on an empty order
  (TECH-3, TILL-12); the waiter's rail header (TBS-13); the service
  control at 48px with its reason visible (TILL-10).
- [x] **Task 6 - Tender and split.** The change due in the sticky foot
  and the numpad beside the amount below 2xl (TILL-4); totals pinned in
  the summary and a single crumb on the paid step (TILL-16); split by
  item lists the items first (TILL-5).
- [x] **Task 7 - Lock and open shift.** The brand panel from xl; profiles
  per row by width (TILL-6, TILL-21); the float total and Open shift in a
  sticky foot (TILL-24).
- [x] **Task 8 - Tables and orders.** The floor fits or lists at 1024 and
  the panel overlays only when a table is picked (TBS-4); Table List keeps
  Balance (TBS-5); Orders keeps Items (TBS-6, TECH-4); the order panel
  shows Paid in its header and fewer buttons (TBS-16).
- [x] **Task 9 - Waiter and close of day.** The waiter's order bar stops
  overlapping (TBS-3); waiter home trims its chrome under `short:`
  (TBS-12); drawer rows labelled and scrolled into view (TBS-15).
- [x] **Task 10 - Kitchen: fit and read at a distance.** `short:` trims on
  the board (KDS-7, TECH-1); ADR-381's sizes and a clamp on
  `--text-kitchen` and `--text-display` (KDS-6, KDS-5); stats off the board
  below 2xl (KDS-10); the station as the title, no script lines (KDS-11);
  ink by default (KDS-8).
- [x] **Task 11 - Back office widths.** Fixed columns to tokens (TECH-17);
  the item editor's Save always in view (BO-2, the layout half); the
  chosen table's card beside the grid (BO-7).
- [x] **Task 12 - Setup at 768.** A trimmed head; scroll and focus on each
  step (SET-3, SET-4).

### Money and state, the rest

- [x] **Task 13 - Close the day knows what is open (TBS-2).** An open
  orders card with a link, the count in the confirm, "Open orders
  carried" on the Z-report. Done 2026-10-31 (ADR-384).
- [x] **Task 14 - One venue clock (BO-1, BO-3, BO-4, TBS-17).** Shared
  `tradingDay`, `venueDate`, `venueDateTime` from `day_start` and the
  venue's zone, at every site the audit lists. Done 2026-10-31 (ADR-385).
- [x] **Task 15 - Tax words (GST-2, TILL-18, GST-15).** `tax_included` on
  the guest table response; "before tax" only when prices exclude it. Done
  2026-10-31 (ADR-386): the field is `tax_added`, on the guest's venue and
  the till's list of requests; the guest's receipt says "GSTIN".
- [x] **Task 16 - No silent loss of work (BO-2, BO-5, TECH-9, BO-6).** Done
  2026-10-31 (ADR-387): one leave guard in the back office's shell; Save
  names what it saves; a Sheet holding a form ignores its scrim; Retire is
  asked with a count, an item's is undone.

### Readable for everyone

- [x] **Task 17 - Contrast tokens (TECH-5, TECH-6, TECH-7, TILL-7, BO-14).** Done
  2026-10-31 (ADR-388): `--muted` `#6E6359`, faint drawn in it, terracotta text
  the pressed shade, the key letters explained, and a test on tokens.css.
- [x] **Task 18 - Diet marks by shape (GST-1, TILL-15).** Done 2026-10-31
  (ADR-389): non-veg is a triangle in its square, veg a dot; the till's
  gluten-free mark says "GF".
- [ ] **Task 19 - Sold out without opacity; warning toasts as chips
  (TECH-13).**

### The kitchen display's behaviour

- [ ] **Task 20 - Hear it and trust it (KDS-1, KDS-2, KDS-3, KDS-14,
  KDS-15, KDS-9).**
- [ ] **Task 21 - What changed (KDS-12, KDS-13, KDS-16, KDS-17, KDS-18,
  TECH-19).**

### Words staff can say

- [ ] **Task 22 - Names and labels.** TILL-8, TBS-7, TILL-9, TBS-8,
  TBS-10, TBS-18, TBS-9, TBS-11, TBS-14, TBS-19, TILL-17, TILL-19,
  TILL-20, TILL-22, TILL-23, TILL-14.

### The guest app

- [ ] **Task 23 - Order status that tells the truth (GST-3, GST-4, GST-5).**
- [ ] **Task 24 - Phone ergonomics (GST-6, GST-7, GST-8, GST-9, GST-12,
  GST-13, GST-14).**
- [ ] **Task 25 - The venue's menu, light on 3G (GST-10, GST-11, TECH-18).**

### First run and empty states

- [ ] **Task 26 - Setup (SET-1, SET-2, SET-5, SET-6, SET-8, SET-9).**
- [ ] **Task 27 - Back office onboarding (BO-8, BO-9, BO-10, BO-11,
  BO-15).**
- [ ] **Task 28 - GST composition scheme (SET-7).** A product decision
  and its ADR first.

### The shared kit

- [ ] **Task 29 - Error boundaries (TECH-8).**
- [ ] **Task 30 - One focus rule, one field rule (TECH-10, TECH-11,
  TECH-12, TECH-20, TECH-14).**
- [ ] **Task 31 - Kit cleanup (TECH-21 to TECH-24, BO-12, BO-13, BO-16 to
  BO-18).**
- [ ] **Task 32 - Split the terminal bundle (TECH-25).**
- [ ] **Task 33 - The audit again**, same walk and sizes; the scores go in
  this file.

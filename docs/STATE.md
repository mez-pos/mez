# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Phase 10 - Back office**
File: `docs/phases/phase-10.md`. Phase 09's tasks are all done; its
done-when (a human checks a Z-report against a hand-computed day, and an
export a bookkeeper would use) is still owed, so it stays unticked below.

## Last completed task

**Phase 10, Task 3a: stock items and the stock ledger** (ADR-111). Task 3
is split into 3a to 3d in the phase file. Migration 00023 adds
`stock_items` and an append-only `stock_movements` ledger, and grants the
new `stock.manage` to owner and manager roles. On hand is the sum of the
ledger; received, adjusted, wasted (with a reason code) and counted are
entered by hand under `/api/v1/stock` (history cursor-paginated).
Quantities are thousandths of g, ml or each. The Stock nav entry now needs
`stock.manage`. Smoke-tested on the real binary: 5 kg received, 3.2 kg
spoiled, 1.8 kg on hand and low.

## Next task

**Phase 10, Task 3b: recipes and deduction on sale.** Which stock each
menu item uses, and a `sold` movement written when an order is invoiced,
in the same transaction. Decide what a void, a comp and a credit note do
to stock, and whether modifiers carry recipes too.

## Known broken / in progress

- Stock (ADR-111): server only until 3c. Nothing is sold out of stock yet
  (3b). A movement after the shift's closing drawer count records no
  shift (3d). No low-stock alert anywhere yet (3d).
- Photos (ADR-110): `<data dir>/photos/` is not backed up by Litestream
  (phase 12 must add it); unreferenced photo files are never swept; the
  guest app does not show photos yet.
- Sold out (ADR-109): only the back office can mark it, not the till; the
  server does not refuse a sold-out item rung up (AddLine is unchecked);
  a till with a wrong clock ends it at the wrong time.
- Menu editor (ADR-106, ADR-107): no `Idempotency-Key` on
  its routes, as with printers. Duplicate item names are allowed. No
  write path yet for stations, courses or tax classes (Task 6). An item's
  groups save in a second request after the item (ADR-108). A negative
  choice price is not checked against the items offering it. A venue
  whose tax profile changes to one needing
  HSN keeps items without one until each is edited (Task 6 must handle
  this). The table scrolls sideways below about 1100px of content width.
  Retiring an item shows no "open tickets keep it" note. Form controls
  use the shared `inputClass`, which does not set the 16px control size
  tokens.md asks for (the printers form has the same gap).

- Shell (ADR-105): only tried as the owner. A cashier or manager view and
  the "not yours to open" screen are tested in vitest, not by hand: no API
  adds staff until Task 4. Printers, floor and Z-reports still carry their
  own "Back office" link above the title, now redundant with the
  navigation. Permissions changed mid-session reach the shell at the next
  sign-in or reload. Stock is gated on `menu.edit` for now.

- Carried from phase 04, owned by no task yet:
  - The done-when script against the real binary. The same loop now runs
    in process on the real stack (ADR-092); the binary itself is left for
    phase 12's installer smoke test.
  - A `float64` lint rule for `internal/domain` (`docs/02-domain.md`).
  - A guard that fails the build when `order.State` changes without a bump
    to `order.ProjectionVersion`; today a forgotten bump serves stale
    snapshots.
- Exports (ADR-103, ADR-104): not yet imported into a real TallyPrime;
  its ledgers are mapped by name once in Tally. Shifts closed before
  migration 00020 have no files. A shift whose files cannot be written
  does not close, as with its Z-report.
- Z-report (ADR-097 to ADR-100): categories are the menu's when the
  report is built, not when the item was sold. The printout has no
  by-item lines; the screen does. The till shows only the report its own
  close issued, until reload; the back office list was typechecked and
  API-tested, not tried in a browser. No screen issues a correction yet
  (the API does). A report that cannot be built blocks the close; there is no
  way round that short of fixing the data.
- Tips (ADR-101, ADR-102): hours are typed in at pooling (no timesheets
  until phase 10); a pool is once per shift with no correction; the rule
  and weights have no back office setting (phase 10). The browser run had
  one staff member: no API adds staff yet, so a multi-person or by-hours
  pool was tested in Go and vitest, not by hand.
- Shifts (ADR-093 to ADR-096): terminals learn of a shift opening,
  counted or closed by polling every 30 seconds, not the stream. Opening
  and closing need shift.manage: a cashier is told to ask a manager, with
  no override sheet. No till screen for cash in or out yet (the API has
  it). The day start is a constant. A payout larger than the drawer is
  not refused. Closing leaves open tickets to carry into the next shift.
  A counted drawer cannot be recounted. The variance threshold has no
  back office setting yet (phase 10); set it in `settings`. Denomination
  tables exist for INR, USD, EUR and GBP only; others count one total.
- `AddLine` is not checked against the menu (price, tax, modifier min/max);
  the menu is now written over HTTP (ADR-106), so this matters more. GST place of supply is always
  the venue's state, so no IGST. `nofiltr setup` cannot finish a US venue.
- `orders.table_id` has no foreign key (ADR-085). Guests left closes and
  clears in two steps, not one transaction; a retry repairs it (ADR-087).
- Nothing stops seating a dirty table over the API; the floor only offers
  Seat on a free one. The floor has no URL of its own (ADR-087).
- A table with two tickets cannot be moved from the floor; the timeline
  names tables by number (ADR-088).
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
- Token gaps (ADR-062): five paper roles are stand-ins; `--vermilion-on`
  on the paper fill is 3.99:1, under AA, and shows on the paper primary
  button in the gallery.
- A bill printed before payment has no number: numbers come with the
  settling payment (ADR-046, ADR-078). Task 4's pre-payment bill layout
  must say it is not a tax invoice.
- A split lives only on the tender screen (ADR-077); by guest picks seats
  there until lines carry them (phase 08).
- The server does not cap a tender's `reference`; the till caps it at 40.
- Printing is ASCII only (ADR-079): non-Latin item names print as "?".
  No physical printer has run the driver yet.
- The Windows spooler's write path is untested on a real printer (ADR-080).
- The cash drawer opens only with the settling receipt, not on a cash part
  payment (ADR-081).
- The ticket rail marks a line Ready but not sent or held by course. No
  `collected` step (ADR-090). A display's station is per device; a void
  on a sent line reaches it within 20 seconds, not at once (ADR-091).
- The till refunds to one payment per credit note (ADR-084); past bills
  search only exact numbers and invoice sequences.
- `make build` does not build under cmd or PowerShell; for the browser,
  `CGO_ENABLED=0 go build -tags frontend -o bin/nofiltr ./cmd/nofiltr`.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `nofiltr setup -demo-menu` prints a pairing code; sign
  in as the owner and count a float to open the till
  (`-device-kind back_office` for the back office); pair in the browser. A
  code for the wrong app is refused but its device stays (ADR-068).
- No endpoint returns the venue's name or a staff member's role; the lock
  screen shows the device name and no roles until one does.
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works, at
  `C:/Program Files (x86)/GnuWin32/bin/make.exe`). `make check` does not
  rebuild the web bundles: run `make build-web` before trying the browser. `make
  generate` needs sqlc v1.31.1 and can dirty `go.sum` with goose's test
  modules: restore it with `git checkout go.sum`. `bin/nofiltr` has no
  `.exe`, so kill a stray server with `taskkill /F /IM nofiltr`.

## Open questions for the human

None.

---

## Phase progress

- [x] 00 — Repo skeleton, docs bootstrap, design tokens
- [x] 01 — Go server skeleton, config, logging, embed, cross-compile
- [x] 02 — SQLite, migrations, sqlc, money type, IDs
- [x] 03 — Domain core: order events, projections, invoice series, tax
- [x] 04 — HTTP API, WebSocket hub, TypeScript type codegen
- [x] 05 — Frontend shell: Vite, Tailwind tokens, router, primitives, auth
- [x] 06 — Terminal: register, modifiers, cart, open tickets
- [x] 07 — Payment, split, invoice, print queue, cash drawer
- [x] 08 — Floor plan, tables, kitchen display, station routing
- [ ] 09 — Shift, blind drawer count, Z-report, tips
- [ ] 10 — Back office: menu editor, inventory, reports
- [ ] 11 — Relay service, outbound tunnel, guest QR ordering
- [ ] 12 — Installer, Windows Service, Docker, Litestream, docs

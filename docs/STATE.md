# STATE

> This is the handoff file between sessions. Rewrite the top four sections at
> the end of every session. Keep it under 60 lines — it is read first, every
> time, and long state files waste the context the work needs.

## Current phase

**Rebuild - the screens follow the mez design** (ADR-143)
File: `docs/phases/rebuild.md`. It runs before phase 11. The owner ran the
build on 2026-10-03 and rejected the screens: the redesign had recoloured the
old layouts and kept their flow. Read that file's flow map, words and rules
before any screen. Phases 09 and 10 still owe their human done-when checks.

## Last completed task

**Rebuild, Task 6: The item modal** (ADR-153). `ItemModal.tsx` is D04 and replaces
`ModifierSheet.tsx`: a centred dialog with the photo or category icon on the left and,
on the right, the name and price, radio tiles for a choose-one group and checkbox tiles
for a choose-several, the course, a kitchen note, and quantity, Cancel and "Add to
Order" with the line's price. A new line's note rides the outbox as a second command
on the line the add made. Changing a line saves its choices, quantity and a note.
Seen in a browser at 1920 and 1366x768 as the cashier: a required choice, quantity 2
with a note, and a line changed (choices, quantity 3, note). Tests: the queue's note,
`groupHint`, `lineTotal`, `addLineFor`. `make check` and `make build-web` pass. The
owner has not yet looked.

Before it: Task 5 (ADR-151, the order screen), Task 4 (ADR-150, Open the shift; a
cashier opens it with `shift.open`, ADR-152, migration 00033, and closing is
`shift.manage`), Task 3 (ADR-149), Task 2 (ADR-148), Task 1 (ADR-145 to ADR-147),
Task 0 (ADR-144), the plan (ADR-143).

## Next task

**Rebuild, Task 7** (tables): D05 (line 363 of `mez Desktop POS.dc.html`) and the
table list from D06 (line 478). Areas, the five states, the selected table's panel:
Add Items, Send to Kitchen, More Actions, Settle Bill. Reservations and the waitlist
are out of scope. It rebuilds `FloorView.tsx` at `/tables`. Read
`docs/design/tokens.md` and those screens only. Phase 11 waits for this phase. Phase 10's
tasks are all done (6l moved to phase 12, ADR-141); its done-when is a human check and
stays unticked below.

## Known broken / in progress

- Audit (2026-10-03): `FormModal` was typechecked and tested, not tried in a
  browser; look at one save, one refusal and Enter in a two-field modal. `mez
  setup` pairs a till by default, so a venue set up with defaults reaches the
  back office with `mez pair -device-kind back_office` (ADR-144); phase 12's
  wizard should pair it. Setup's closing line and expiry now follow the
  device kind and the venue's zone.

- Redesign (ADR-128): screens are restyled through the primitives only.
  The specs' layouts were not re-read against the mez screen files,
  only their vocabulary and rules. Not seen by eye: card and
  UPI tender, split, the invoice panel, a menu card with a photo, the
  discount, note, approval and tickets sheets, a past bill and its
  refund sheet, the print queue, the reconciliation after a count,
  closing stock, floor states other than free and ordered, move and
  join, the kitchen's waiting tone, order and line notes, a voided
  line and Recall, a toast. In the back office: menu
  choices, recipes, roles, Z-reports, tips, audit, printers, and the
  sales screen after the charts turned green. Charts are one colour;
  the design's green, terracotta, mustard order waits for a chart with
  more than one series. The
  drawer count's face values sit right-aligned in their column, as
  before the redesign. NumPad keys use `sunken`, a stand-in on night that matches
  the canvas: no night screen has a keypad yet. Menu table cells wrap at
  1366px (Task 7).

- Devices (ADR-140): revoking a device does not close its open live stream; it
  is refused at its next connect, and every request it makes is refused at once.
  A revoked till's name in the hub is the one it connected with. Rename and
  revoke carry no `Idempotency-Key`. A device is "last seen" at most once a
  minute. The pairing code is shown once and cannot be shown again from the
  list (only its hash is kept).
- Stations and courses view (ADR-139): it cannot show whether a station has a
  printer (that is `settings.edit` data), so a retire says the printer follows
  and the toast says whether it did. A kitchen display set to a retired station
  still shows its board and is set to another by hand (ADR-134). Ordering is by
  arrows, not drag.
- Shift, tips and receipt screen (ADR-138): the receipt preview is of a made-up
  bill and shows no credit note; it was checked against the layout, not a
  printer. A tip weight for a renamed or removed role is not shown and drops at
  the next save. The preview asks the server on every pause in typing.
- Tax and numbering screen (ADR-137): the live preview of the next numbers
  needs a second ask per pause in typing; the rounding step is saved but nothing
  applies it (ADR-132). The profile card's "leaves N items without an HSN" shows
  only after a save, not as a draft. A tax class's retire refusal names eight
  items. The shell's navigation still drops a dirty draft.
- Venue settings screen (ADR-136): the "Leave without saving?" prompt is not
  seen by eye (only a second section can raise it, from the rail), and the
  shell's navigation drops a draft without asking. Timezone suggestions come
  from the browser's list.
- Receipt template (ADR-135): no screen yet (6i). The till's on-screen invoice
  preview does not show the header and footer, and no logo (ADR-079). A
  duplicate of an old receipt prints with today's template. Not tried on a
  printer: tested through the layout decoder at both paper widths.
- Stations and courses (ADR-134): no screen yet (6j). A retired station's
  kitchen display keeps its board and must be set to another by hand. Retiring
  a course leaves a gap in the numbers until the next ordering. Server and
  item form only, not tried in a browser.
- Rules (ADR-133): no screen yet (6i). A renamed role orphans its tip
  weight, and nothing checks. A day start change re-buckets past days in
  the reports, audit trail and CSV (events are placed by time); only
  shifts keep the day they opened under. Not tried in a browser: server
  only, Go tests and the HTTP round trip.
- Numbering (ADR-132): the cash rounding step is saved but no sale or
  payment applies it yet (a payment path owes that). The financial year
  start cannot move once a number is issued. A tax profile change does
  not start a new invoice series; one series spans it (ADR-131).
- Audit (ADR-117): discounts and refunds recorded before Task 4c show
  "Not recorded" for who authorised them. The Person filter lists active
  staff only (it reads `GET /api/v1/staff`); a deactivated person's rows
  show by name but cannot be filtered on. The audit screen has no CSV
  button yet (the route exists, ADR-119). The analysis (ADR-122) sums
  what each correction was worth to its bill, not money out of the drawer.
- CSV (ADR-119): each sheet reads the range again. The Tally files skip
  the formula guard and the byte order mark the report files have.
- Sales screen (ADR-120, ADR-121): tried with one day of sales and no
  credit notes, refunds or discounts, so the heatmap was seen with one
  cell only (its layout over many hours is tested in vitest, not by eye).
  A CSV is in the server's order, not the table's sort.
- Corrections (ADR-123): the browser run had one person, so a row for a
  guest, "not recorded" or someone removed was not seen by eye. Back from
  the trail returns `/sales` to seven days. A deactivated person's link
  shows "Someone no longer active" in the Person box (not seen by eye).
  A sales download and a corrections download can run at once. The range is not in the URL, so a reload returns to
  seven days. A CSV has not been opened in Excel itself. Left open past
  04:00, the range stays yesterday's and no quick range shows pressed.
- Reports (ADR-118): the screen lacks 5d to 5f. A range projects every order
  it touches from its first event; a year at a busy venue may take seconds.
  Items and categories are named as the menu stands now. The day start is
  the venue's setting as it stands when asked (ADR-133).
- Stock cost (ADR-124): movements from before migration 00028 have no
  cost. Nothing shows a cost's history; a new cost is for stock moved
  from then on.
- Closing stock (ADR-125, ADR-126): a shift with no closing count is not
  listed. Items are named as they are now. The browser run had one close
  on one day; several shifts and a partly costed item were tested in Go
  and vitest, not by eye. No subtotal per shift on screen.
- Stock (ADR-111 to ADR-114): The
  closing count cannot be skipped while items are marked. A close uses today's
  recipes, not the ones when the line was rung up. A sold movement needs a
  real staff id: the guest channel (phase 11) must supply one. A movement
  entered by hand after the closing drawer count records no shift (the
  closing count names its own, ADR-114).
- Photos (ADR-110): `<data dir>/photos/` is not backed up by Litestream
  (phase 12 must add it); unreferenced photo files are never swept; the
  guest app does not show photos yet.
- Sold out (ADR-109): only the back office can mark it, not the till; the
  server does not refuse a sold-out item rung up (AddLine is unchecked);
  a till with a wrong clock ends it at the wrong time.
- Menu editor (ADR-106, ADR-107): no `Idempotency-Key` on
  its routes, as with printers. Duplicate item names are allowed.
  Stations, courses and tax classes have a write path (ADR-134). An item's
  groups save in a second request after the item (ADR-108). A negative
  choice price is not checked against the items offering it. A venue
  whose tax profile changes to one needing
  HSN keeps items without one until each is edited; the tax settings list
  them (ADR-131). The table scrolls sideways below about 1100px of content width.
  Retiring an item shows no "open tickets keep it" note. Form controls
  use the shared `inputClass`, which does not set the 16px control size
  tokens.md asks for (the printers form has the same gap).

- Shell (ADR-105): tried as owner and manager (Task 4b); a cashier sees
  the dashboard alone (vitest). Printers, floor and Z-reports still carry
  their own "Back office" link above the title, now redundant with the
  navigation. Permissions changed mid-session reach the shell at the next
  sign-in or reload.

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
  and weights are set through the rules route (ADR-133), with no screen yet (6i). The browser run had
  one staff member, so a multi-person or by-hours pool was tested in Go
  and vitest, not by hand; staff can now be added (Task 4a).
- Shifts (ADR-093 to ADR-096): terminals learn of a shift opening,
  counted or closed by polling every 30 seconds, not the stream. Opening
  needs shift.open (ADR-152) and closing shift.manage: a cashier who cannot
  close is told to ask a manager, with no override sheet. No till screen for cash in or out yet (the API has
  it). The day start is a venue setting (ADR-133). A payout larger than the drawer is
  not refused. Closing leaves open tickets to carry into the next shift.
  A counted drawer cannot be recounted. The variance threshold is set
  through the rules route (ADR-133), with no screen yet (6i). Denomination
  tables exist for INR, USD, EUR and GBP only; others count one total.
- `AddLine` is not checked against the menu (price, tax, modifier min/max);
  the menu is now written over HTTP (ADR-106), so this matters more. GST place of supply is always
  the venue's state, so no IGST. `mez setup` cannot finish a US venue.
- `orders.table_id` has no foreign key (ADR-085). Guests left closes and
  clears in two steps, not one transaction; a retry repairs it (ADR-087).
- Nothing stops seating a dirty table over the API; the floor only offers
  Seat on a free one. The floor has no URL of its own (ADR-087).
- A table with two tickets cannot be moved from the floor; the timeline
  names tables by number (ADR-088).
- Never swept: idempotency keys, expired staff sessions, sign-in attempts.
  The pairing rate limiter is per process.
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
  `CGO_ENABLED=0 go build -tags frontend -o bin/mez ./cmd/mez`.
- A data dir path containing `%` cannot be opened: `internal/db/open.go`
  does not escape the path in its `file:` DSN.
- To try the apps: `make dev` (ADR-144) prints a pairing code for each; pair
  in the browser, sign in as the owner or a demo person and count a float to
  open the till. A code for the wrong app is refused but its device stays
  (ADR-068).
- The item modal (ADR-153): one tap still rings up an item whose defaults satisfy its
  groups, so its modal is reached from the line's Modify; say if every item with
  choices should open it. A note refused after its line was added is lost with a
  toast. Not seen by eye: a photo, the Popular pill, a refusal, a screen under 1024px.
- The order screen (ADR-151): not seen by eye are a photo card, a sold-out card, a
  held order's banner, a delivery order, a discounted order's totals and a line the
  kitchen has seen (its bin and plain quantity). The diet filters reset on a reload.
- The open-shift screen's sold-out card was not seen by eye, and its float panel and
  buttons sit below the fold at 768px high (ADR-150).
- `go test -race` needs cgo, which is banned; `make check` runs without it.
- The domain import test checks direct imports only (ADR-016).
- Tooling: `make` needs `sh` (Git Bash; GnuWin32 make 3.81 works, at
  `C:/Program Files (x86)/GnuWin32/bin/make.exe`). `make check` does not
  rebuild the web bundles: run `make build-web` before trying the browser. `make
  generate` needs sqlc v1.31.1 and can dirty `go.sum` with goose's test
  modules: restore it with `git checkout go.sum`. `bin/mez` has no
  `.exe`, so kill a stray server with `taskkill /F /IM mez`.

## Open questions for the human

- A data directory from before the rename: move it to the new default
  and rename `nofiltr.db` to `mez.db`. Paired browsers pair again.

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

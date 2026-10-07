# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 10 - Back office**, `docs/phases/phase-10.md`, reopened by the
human on 2026-10-08 for **Task 7: import the menu from a spreadsheet**
(ADR-288). Every other task in every phase is built; what those owe is
to be seen working: the walks, a release, and a printer.

## Last completed task

**Phase 10, Task 7a: read and check the file** (ADR-288).
`internal/domain/menu/import.go`: `ReadImport(data, ImportVenue)`
answers an `ImportPlan` (rows to add, new categories, rows already
there, problems by row and column) and writes nothing. CSV as a
spreadsheet saves it: a byte order mark, semicolons or tabs, headers in
any case and order. Prices through `money.Parse`; a comma that does not
group digits is a problem, not a guess. An empty tax class, station or
course is the only active one, or a problem listing the names. Adding
only: an item already there is skipped. 81 tests, 96% of the package.
Nothing calls it yet.

## Next task

**Phase 10, Task 7b: the import on the server.** Build `ImportVenue`
from the database in `internal/service/menus` (`Editor` already reads
all of it; a course's `Also` is its number, a station's its code), then
a dry run that answers the plan and an import that writes it: new
categories and every item inside one `s.edit`, so one version bump and
one nudge. Refuse unless `plan.OK()`. Read the file again inside the
transaction, so a menu edited since the preview cannot be imported
over. `menu.edit`, an idempotency key, a size limit on the body. Then
7c (the template) and 7d (the screen, spec first).

Apart from Task 7, for the owner, in the order that unblocks the most:
1. **A printer and a drawer.** `mez --test-print <ip> -drawer`, then a
   sale. Nothing has ever been printed (`hardware.md`, ADR-282).
2. **A release.** Push a tag (`docs/releasing.md`); expect the first to
   need a second try. The workflow has never run.
3. **Phase 12's "done when"**: a clean Windows machine, the downloaded
   installer, a first printed invoice in under ten minutes. "Open mez
   now" opening the setup link (ADR-270) and the window it opens in
   (ADR-272) have not been watched since they changed.
4. The human checks phases 09 to 11 owe, and the questions below.
5. Decide the guest app's licence: the README says it was meant to be
   MIT and is under the AGPL.

**Do not move Litestream past v0.5.15 without running the tests on
Windows.** `make check` embeds whatever `web/*/dist` holds: run `make
build-web` before walking a screen against `bin/mez`.

Still owed from phase 11, its human check and:
- **No real charge has been made yet** (ADR-221): with test keys, on a phone, Pay
  online on a bill, a share from two phones, and Pay on an order paid first.
- **Ordering to collect, end to end, with a gateway's test keys**: make the code
  on the Floor plan, scan it, order, pay, see the code, find it under To collect,
  Handed over. None of 5f2 to 5f4 has been seen against the real binary.

## Known broken / in progress

- **Between phases (ADR-256 to ADR-262): the first security review and
  what the owner decided.** A till's line is priced from the menu on the
  server; a relay must be TLS; revoking a device cuts its stream; the log
  masks table codes; the toolchain is go1.26.8 (ADR-256 to 258). Then:
  printers on the LAN only and the data directory the owner's on Windows
  (259); a failing PIN hard-locks until a manager or `mez unlock` clears it,
  six digits for powerful roles (260); a tip over the venue's ceiling needs
  a manager (261); a guest's phone holds a session for the sitting (262).
  Only the LAN's plain HTTP is left, for phase 12. Walked in a browser
  against the dev venue on 2026-10-07: a first phone lands, orders, is
  accepted and reads the bill; a second phone at the open table sees "Ask a
  member of staff", its order is a "New phone at Table T3" request, and
  acceptance hands it a session; Kabir hard-locked shows Locked on People
  and Unlock lets him sign in; a ₹100 tip on ₹150 warns, opens the approval
  sheet, Priya approves, the event carries her id and Audit lists it. No
  console errors on any screen.

- **Between phases (ADR-255): the screens now fit 1024 to 2560 wide and 768
  tall.** Fluid `w-side`, `w-rail`, `w-panel` widths, auto-fill card grids, a
  `short:` variant, script lines out of the working chrome (tokens.md,
  "Layout"). Checked in a browser at 1024, 1366, 1920 and 2560 on the main
  screens; the back office's menu, stock, settings, people and printers
  screens were not walked at 1024 and may still show a tight column.

- A dev server left running blocks `make check` from rebuilding `bin/mez`: stop it first.
- The relay: no load test, no backup of its signing secret, a `democafe` test key. Its
  1200 a minute per venue is under what a full room's phones ask (ADR-246).

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):
1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. `jskin.in` is for testing, `mez.ac` for production: move before codes are printed.
8. The firewall rule is for Private and Domain networks (ADR-265). Public too?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (built; owes its human check)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs (built; owes its human check)

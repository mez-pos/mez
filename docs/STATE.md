# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3
and 4a to 4d done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4d: the bill on the phone** (ADR-214).
`GET /order/api/t/{code}/bill` is the table's open ticket with the till's parts
taken out; `#bill` shows its lines, tax, total, paid and left to pay, and says to
pay staff or at the counter. `web/guest/src/Bill.tsx`, `bill-logic.ts`; spec
`guest-bill.md`. The prepay setting moved to 4e.

## Next task

**Phase 11, Task 4e: paying from the phone by UPI.** Several sessions: split it
first, in the phase file, before building. Read `docs/02-domain.md`, ADR-209 to
ADR-214, `internal/service/settings/rules.go`, `backoffice-settings.md`, M08.
- First decide how a UPI payment is confirmed. A UPI intent link and a QR carry
  the venue's address and the amount, and tell the venue nothing when paid: either
  staff confirm it on the terminal (as the till's own UPI tender does), or it waits
  for 4f's gateway. Read how the till tenders UPI (`terminal-tender.md`) first.
- A likely first sub-task: the venue's UPI address as a setting the owner edits,
  and the link and QR on `#bill` for the amount left to pay, with staff confirming.
  A QR needs an encoder inside the 60 KB budget (38.8 used). Then the mode, and a
  round held until paid.

## Known broken / in progress

- A dev server left running blocks `make check` from rebuilding `bin/mez`: stop it first.
- The relay: no load test. Its signing secret has no backup. A `democafe` test key exists.

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):
1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. `jskin.in` is for testing, `mez.ac` for production: move before codes are printed.

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3 and 4a to 4d done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

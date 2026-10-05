# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a and 4b1 done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4b1: answering a guest's request, on the server** (ADR-211).
`GET /api/v1/guest-requests`, and `POST .../{id}/accept` and `.../decline`, asking
`order.open`. Accept finds or opens the table's ticket and appends the lines as
stored. A request arriving or answered moves the floor version, so terminals hear
`floor_changed`. A request lasts as long as its shift.

## Next task

**Phase 11, Task 4b2: the request on the terminal.** Read ADR-209 and ADR-211,
`docs/design/tokens.md`, `terminal-shell.md` and `terminal-floor.md`, and the
design's D13 only if the shell has nowhere for it.
- What is waiting has to show on every terminal screen, not only Tables, with a
  sound when one arrives (ADR-209): the shell is where. Read
  `GET /api/v1/guest-requests` with React Query and read it again on
  `floor_changed`. Types are generated (`GuestRequestsResponse`).
- Accept and Decline, each request on its own, with its table, lines, subtotal and
  how long it has waited. `request_decided` and `request_lapsed` mean read again.
- The table on the floor says it wants to order; the waiter's home too, or say why not.
- A browser plays no sound before the page is touched: decide what then. Write the spec.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a and 4b1 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3 and
4a done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4a: a guest's cart becomes an order, on the server** (ADR-210).
`POST /order/api/t/{code}/orders` with ids, counts and a required `Idempotency-Key`;
`menu.Menu.Ring` prices each line. A "round" (`guest_rounds`, migration 00039,
`svcorder.PlaceRound`) joins the table's open ticket and is sent, or with none open
is kept as a request: no order and no event. Task 4 is split in seven.

## Next task

**Phase 11, Task 4b: the request on the terminal.** Read ADR-209 and ADR-210,
`terminal-floor.md` and `terminal-shell.md`, and the design's D13 only if needed.
- A waiting round has to show on the terminal with a sound (ADR-209), and arrive
  over the stream: a hub message, as `MsgFloorChanged` is one.
- Accept: open a `qr_table` ticket at the table, append the round's lines as stored
  (not priced again), send them as `sendRound` does, mark the round `sent`, in one
  transaction. Decline: `declined`, with who and when (`decided_by`, `decided_at`).
- Decide: a permission for it; what Accept does with the table's other waiting
  rounds; when an unanswered round lapses. Split it if it is two sessions.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3 and 4a done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

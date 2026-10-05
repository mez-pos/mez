# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d and 4e1 to 4e4 done. (The rebuild, 09 and 10 still owe the walk.)

## Last completed task

**Phase 11, Task 4e4b: "I have paid" on the screens** (ADR-219). The phone's bill
has the button, and says staff were told from the bill's `paid_told`. The
terminal's pill, sheet ("From the tables") and chime carry the notice, with Take
payment opening `/pay/{order}` and No payment found putting it away.

## Next task

**Phase 11, Task 4e5: the prepay mode.** Several sessions: split it first. Read
`docs/02-domain.md`, ADR-209, ADR-210, ADR-211, ADR-215, ADR-218.
- A venue setting, prepay or pay at the table, beside the UPI address on Settings.
- In a prepay venue a round is not sent until paid (ADR-209). With staff
  confirming payments (ADR-215) there is no ticket to tender against before a
  round is accepted: decide whether Accept in a prepay venue opens the ticket and
  takes the payment in one step, or whether prepay waits for 4f's gateway, which
  can confirm by itself. Write the case in the ADR before building either.
- Decide who closes a bill nobody on staff touched: a sold stock movement needs a
  real staff id (KNOWN.md, Stock).

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 to 4e4 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

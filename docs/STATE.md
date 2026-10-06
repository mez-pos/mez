# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4, 4e5a and 4f1 to 4f5 done. (The walks are owed.)

## Last completed task

**Phase 11, Task 4e5a: a round held until it is paid** (ADR-225). `payment.mode`
is `pay_at_table` or `prepay`. Where guests pay first and can, `PlaceRound` keeps
the round `unpaid` with `Due`, what it adds to the bill by `order.Adding`. It is no
order and nothing for staff. **Not in effect in `cmd/mez`: `CanCharge` is unwired.**

## Next task

**No real charge has been made yet** (ADR-221): Razorpay test keys, Pay online on a
phone, a test card. Building 4e5b on an adapter nobody has seen work is a risk.

**Phase 11, Task 4e5b: charging an unpaid round.** Read ADR-210, ADR-211, ADR-220,
ADR-225, `internal/service/charges/charges.go`. Likely two sessions: split first.
- A charge is for a ticket today (`gateway_charges.order_id NOT NULL`). It has to
  be for a round too: a migration that rebuilds the table, a second way to start.
- Applying: open the ticket if the table has none, append the round's lines as
  stored and send them (as `AcceptRound` does), then tender. Each step idempotent.
- Decide what is tendered when the bill moved and the rise is off `due_minor`:
  never more than was charged. Then, and not before, wire `CanCharge` in `cmd/mez`.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (4a to 4d, 4e1 to 4e4, 4e5a, 4f1 to 4f5 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

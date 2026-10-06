# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4, 4e5a, 4e5b and 4f1 to 4f5 done. (The walks are owed.)

## Last completed task

**Phase 11, Task 4e5b2: a guest pays for a round** (ADR-227).
`POST /order/api/t/{code}/orders/{round}/charge`; its status is the bill charge's
route, which now says `round`. `CanCharge` is wired: paying first is in effect
where a gateway is set and `payment.mode` is `prepay` (API only, off by default).

## Next task

**No real charge has been made yet** (ADR-221): Razorpay test keys, Pay online on a
phone, a test card. 4e5c is the last thing built on an adapter nobody has seen work.

**Phase 11, Task 4e5c: the phone pays for a round.** Read ADR-213, ADR-222,
ADR-227, `docs/design/screens/guest-order.md` and `guest-bill.md`, and
`web/guest/src/Order.tsx`, `use-charge.ts`, `charge-logic.ts`, `Bill.tsx`.
- An unpaid round shows its `due` and a Pay button: start the round's charge, go
  to the gateway, and on return ask how it stands, as the bill's Pay online does.
- Paid, the round reads sent. Failed, it can be tried again. Unapplied, show staff.
- A phone that comes back to its unpaid round (the tab was closed) can still pay.
- Hold the 60 KB budget. Update the screen spec. See it at 390 wide.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (4a to 4d, 4e1 to 4e4, 4e5a, 4e5b, 4f1 to 4f5 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4 but 4f6b (Cashfree, question 8), 5 but 5f2 on, and 6 done. (Walks owed.)

## Last completed task

**Phase 11, Task 5f1b: paying for an order to collect, and sending it** (ADR-249).
A charge may have no table (migration 00050). Paid, the order is a ticket of its
own on the `web` channel, sent, with a pickup code no order still out shares.

## Next task

**No real charge has been made yet** (ADR-221): with test keys, on a phone, Pay
online on a bill, a share from two phones, and Pay on an order paid first.

**Phase 11, Task 5f2: ordering to collect over the guest API.** Read ADR-247 to
ADR-249, ADR-246, `internal/api/guest.go`, `guest_order.go`, `guest_charge.go`.
- Routes under the venue's pickup code, `/order/api/p/{code}/...`: the venue, the
  menu, place an order (`PlacePickup`), pay for it (`StartRound` with no table),
  how the charge stands, and track it with its `pickup_code`.
- `tablecodes.ResolvePickup`. Offered only where a gateway is set: say so plainly.
- Limits (ADR-246) are by table: key them by the venue here. The return address
  after paying is `<public_url>/order/p/<code>`. A receipt (ADR-240) asks by a table.

## Known broken / in progress

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
8. Cashfree needs each payer's phone number (ADR-230). Ask guests for one, or leave it out?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 4, 5 but for 4f6b, 5f; 6 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

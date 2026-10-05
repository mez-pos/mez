# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3
and 4a to 4c done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4c: Place Order on the phone** (ADR-213). The cart sends itself
with one key for one cart; the answer is kept by table code and shown at `#order`
as waiting, with the kitchen, or not accepted; a waiting order asks again every 8
seconds. `web/guest/src/order-logic.ts`, `use-order.ts`, `Order.tsx`; spec
`guest-order.md`. A guest can now order from a phone, end to end, unpaid.

## Next task

**Phase 11, Task 4d: how the venue is paid.** Read `docs/02-domain.md`, ADR-209 to
ADR-213, the settings service's rules (`internal/service/settings/rules.go`), and
M08 in the design. It is likely two sessions: split it first.
- A venue setting: prepay required, or pay at the table. Where the owner sets it.
- The bill on the phone, with its tax as the server works it out: the guest's menu
  carries no tax rates, so this needs the table's ticket projected, read by code.
  Decide what one guest may see of a ticket the whole table shares.
- In a prepay venue a round is not sent until paid (ADR-209): decide what a round
  is between Place Order and payment, since 4e brings the payment itself.
- 37.9 KB of 60 in the guest bundle before this.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3 and 4a to 4c done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

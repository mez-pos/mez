# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4, 4e5a to 4e5c and 4f1 to 4f5 done. (The walks are owed.)

## Last completed task

**Phase 11, Task 4e5c: the phone pays for an order** (ADR-228). An unpaid order's
screen shows what to pay and **Pay**; back from the gateway it asks how the charge
stands and then reads "Paid, and with the kitchen". Guest bundle 41.7 KB of 60.

## Next task

**No real charge has been made yet** (ADR-221): Razorpay test keys, then on a
phone Pay online on a bill, and Pay on an order with `payment.mode` set to `prepay`.

**Phase 11, Task 4e5d: the owner's choice of mode in the back office.** Read
ADR-216, ADR-225, `docs/design/screens/backoffice-settings.md`,
`web/backoffice/src/settings/PaymentsCard.tsx`, `payments-logic.ts`.
- Two choices beside the UPI address: pay at the table, or pay first. The server
  takes `mode` on `PUT /api/v1/settings/payments` and answers `can_charge`.
- With `can_charge` false, pay first is shown and says why it has no effect: no
  gateway is set, which is the venue's configuration and not this screen's.
- Say what pay first does to a guest: nothing reaches the kitchen until paid.
- Then consider the cart saying so before Place Order (KNOWN.md).

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (4a to 4d, 4e1 to 4e4, 4e5a to 4e5c, 4f1 to 4f5 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

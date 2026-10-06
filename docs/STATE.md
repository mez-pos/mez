# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4 but for 4f6b (Cashfree, question 8), and 5a to 5d done. (The walks are owed.)

## Last completed task

**Phase 11, Task 5d: the e-receipt on the server** (ADR-240).
`GET /order/api/t/{code}/receipts/{bill}`: the counter's own invoice, by the
table's code and the bill's id. The open bill and a tracked order say `bill`.

## Next task

**No real charge has been made yet**, at Razorpay (ADR-221) or at Stripe: test
keys, then on a phone Pay online on a bill, a share of one from two phones, and
Pay on an order with paying first chosen. The payments card says so itself.

**Phase 11, Task 5e: the receipt on the phone, and ordering the same again.**
Read ADR-240, ADR-239, `docs/design/screens/guest-bill.md`, `guest-order.md`,
`docs/design/tokens.md`, and M09 in the reference. Likely two: split it first.
- The phone keeps `bill` from the bill and from a tracked order, and when the bill
  reads "nothing open" or paid, offers the receipt. A screen of its own, and a spec.
- Times in the venue's `time_zone`, not the phone's. No figure worked out here.
- Re-order puts an order's dishes back in the cart from its own stored request,
  held against today's menu, as the cart already does. It places nothing.

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
8. Cashfree needs each payer's phone number (ADR-230). Ask guests for one, or leave it out?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Task 4 but for 4f6b, 5a to 5d done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

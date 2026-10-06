# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4 but for 4f6b (Cashfree, question 8), 5a and 5b done. (The walks are owed.)

## Last completed task

**Phase 11, Task 5b: tracking over the guest API** (ADR-238).
`GET /order/api/t/{code}/orders/{round}` answers a round as it stands, with
`progress` once it is sent. Another table's code finds nothing. No screen.

## Next task

**No real charge has been made yet**, at Razorpay (ADR-221) or at Stripe: test
keys, then on a phone Pay online on a bill, a share of one from two phones, and
Pay on an order with paying first chosen. The payments card says so itself.

**Phase 11, Task 5c: the order's screen follows the kitchen.** Read ADR-236,
ADR-238, `docs/design/screens/guest-order.md`, `docs/design/tokens.md`, and the
reference only if the spec names lines. `web/guest/src/`: `Order.tsx`,
`order-logic.ts`, `use-order.ts`. Budget 60 KB gzipped; it is at 43.3.
- Ask the new route for a placed order, in place of sending it again (ADR-210).
- Words for each `progress`, and for none: an order sent before 5a has none.
- How often to ask, and when to stop: a recall takes `ready` back (ADR-238's cost).
- See it at 390 wide, moving an order through the kitchen display.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Task 4 but for 4f6b, 5a and 5b done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3 done.
(The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 3c: a dish with its choices, and the cart** (ADR-208). The sheet
uses the till's own rules (`web/shared/modifier-selection.ts`, moved). The cart is ids
and counts in `localStorage` by table code, priced from the menu each time and held
against it. Nothing is sent yet. Specs: `guest-item.md`, `guest-cart.md`. 36.1 KB of 60.

## Next task

**Phase 11, Task 4: guest checkout.** It is several sessions: split it first. Read
`docs/02-domain.md`, ADR-206 and ADR-208, and M04's checkout, M08 and M09.
- First sub-task: a guest's cart becomes an order. `POST /order/api/t/{code}/orders`
  with the cart's lines (dish id, choice ids, count) and an `Idempotency-Key`; the
  server prices it from the menu, never from the phone, and it is the same order,
  events and kitchen ticket as a counter sale. Decide who the actor is, how an open
  shift is required, and whether a table's guests share one order.
- **Before guests can order, decide what ties an order to someone being at the
  table** (ADR-206's cost): a photographed sticker works from anywhere.
- The cart's foot becomes Place Order; "See the menu" and "Add to cart" go back to the
  design's words; decide special instructions. Then payment, UPI, the gateway, splits.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

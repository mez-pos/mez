# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a and 4b done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4b2: the request on the terminal** (ADR-212). A pill in the top
bar on every screen while a request waits, a sheet with Accept and send and
Decline, the kitchen's chime on arrival and every 30 seconds, and a mark on the
table. `web/terminal/src/requests/`, spec `terminal-requests.md`. Task 4b is done.

## Next task

**Phase 11, Task 4c: Place Order on the phone.** Read ADR-207 to ADR-211,
`guest-cart.md`, `guest-item.md`, and M04's checkout in the design.
- The cart's foot becomes Place Order: `POST /order/api/t/{code}/orders` with a key
  made once for each cart and kept with it, so a retry is the same order.
- What the guest sees next: waiting, sent or declined. Until Task 5 the phone
  learns the answer by sending the same round again with its key (ADR-211).
- Each refusal in its own words (`dish_unavailable`, `choices_invalid`,
  `table_not_ordering`, `requests_waiting`, `no_open_shift`); the first two mean
  read the menu again.
- "See the menu" and "Add to cart" go back to the design's words. Decide special
  instructions: the server takes none yet. 36.1 KB of 60 before this.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a and 4b done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

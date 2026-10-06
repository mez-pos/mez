# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4 but for 4f6b (Cashfree, question 8), and 5 but for 5f, done. (Walks owed.)

## Last completed task

**Phase 11, Task 5e2: ordering the same again** (ADR-242). "Order this again" on
a sent order adds its dishes to the cart, held against today's menu, and opens
the cart. It places nothing. Task 5 is done but for 5f, pickup.

## Next task

**No real charge has been made yet**, at Razorpay (ADR-221) or at Stripe: test
keys, then on a phone Pay online on a bill, a share of one from two phones, and
Pay on an order with paying first chosen. The payments card says so itself.

**Phase 11, Task 6: QR codes and table codes.** Read the phase file's Task 6,
ADR-206, ADR-237, `internal/service/tablecodes`, `internal/domain/qr`, and the
back office's floor screen spec. Larger than a session: split it first.
- Codes exist and renew (`tablecodes`); a QR is drawn on the server (`qr`).
  What is missing is the owner's side: see, print as a sheet, renew, revoke.
- A rate limit per table: guests' routes have none. Tracking asks every 10
  seconds (ADR-239), so the limit must not catch a phone doing only that.
- The venue's pickup code (ADR-237) is made here. Then 5f, which wants both.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 4 and 5 done but for 4f6b and 5f)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

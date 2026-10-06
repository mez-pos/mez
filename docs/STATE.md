# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4e, 4f1 to 4f5, 4f6a and 4f6c done. (The walks are owed.)

## Last completed task

**Phase 11, Task 4f6c: the back office says how paying online stands** (ADR-231).
The "Paid from a phone" card names the gateway, when a payment last landed through
it, whether it calls back, and where guests return to. Read only; never a key.

## Next task

**No real charge has been made yet**, at Razorpay (ADR-221) or at Stripe: test
keys, then on a phone Pay online on a bill, and Pay on an order with paying
first chosen. The payments card now says so itself, in the warning tone.

**Phase 11, Task 4g: splitting the bill among the guests at a table.** Read
ADR-214, ADR-220, ADR-226, `docs/02-domain.md` on split payment,
`docs/design/screens/guest-bill.md`. Larger than a session: split it first.
- Decide what a share is: an amount of the balance (equal parts, or a figure the
  guest types), not lines. The bill does not know whose phone ordered what.
- A share is a charge for part of the balance: `charges.Start` asks for all of
  it today. Two phones paying at once must not over-pay. 4f6b waits on question 8.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (4a to 4e, 4f1 to 4f5, 4f6a, 4f6c done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

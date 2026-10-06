# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4e, 4f1 to 4f5, 4f6a, 4f6c, 4g1 and 4g2 done. (The walks are owed.)

## Last completed task

**Phase 11, Task 4g2: a share over the guest API** (ADR-233). The bill answers
`split` (the rest, what this phone can take, equal parts); a body on
`POST .../bill/charge` is a share, by amount or among N people. No screen.

## Next task

**No real charge has been made yet**, at Razorpay (ADR-221) or at Stripe: test
keys, then on a phone Pay online on a bill, and Pay on an order with paying
first chosen. The payments card now says so itself, in the warning tone.

**Phase 11, Task 4g2b: giving up a payment page.** Read ADR-232, ADR-233,
`internal/service/charges/share.go`. A guest who looks at Pay online and comes
back holds the whole bill for half an hour, so nobody can take a share.
- Letting go is a mark on the charge, not its end: the page can still be paid,
  and then it settles as any charge does (unapplied if the bill cannot take it).
- A charge let go is no claim. Who may let go: the table, by the charge's id.
- A charge for the rest is shared by phones: one letting go lets go for all.
Then 4g3, the phone. 4f6b waits on question 8.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (4a to 4e, 4f1 to 4f5, 4f6a, 4f6c, 4g1, 4g2 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

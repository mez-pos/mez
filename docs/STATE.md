# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4 and 4f1 to 4f5 done. 4e5 waits for 4f. (The walks are owed.)

## Last completed task

**Phase 11, Task 4f5: the gateway's webhook** (ADR-224).
`POST /order/api/hook/{gateway}` takes a gateway's signed call as a hint to
`Settle` now; what it says is never read. `gateway_webhook_secret` unset, it is 404.

## Next task

**The owner should first make one real charge**, which nothing has yet: Razorpay
test keys in `MEZ_GATEWAY_KEY_ID` and `MEZ_GATEWAY_KEY_SECRET`, `MEZ_GATEWAY=razorpay`,
Pay online on a phone, a test card. If it does not land, that is the next task.

**Then Phase 11, Task 4f6: Cashfree, Stripe, and the owner's choice.** Read
ADR-220 to ADR-224 and each provider's own documentation, not memory.
- Two more adapters behind `gateway.Gateway`, each tested against a stand-in as
  Razorpay's is: a hosted page for one amount, and its status. `config.validGateway`
  and `newGateway` in `cmd/mez` name them.
- Three sessions at least: split first. Ask the owner whether both are wanted now:
  one gateway with a real charge behind it is worth more than three without.
  **4e5, the prepay mode, may come first.**

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 to 4e4, 4f1 to 4f5 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

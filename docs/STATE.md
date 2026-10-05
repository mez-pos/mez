# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4 and 4f1 done. 4e5 waits for 4f. (The walks are still owed.)

## Last completed task

**Phase 11, Task 4f1: the gateway interface, and a charge becoming one payment**
(ADR-220). `internal/gateway` is two questions, Start and Status.
`internal/service/charges` starts a charge for what a table owes and, when the
gateway says paid, tenders it once (`charge:<id>` as the key); money a bill cannot
take is kept as `unapplied`. Tested against a fake. Nothing is wired.

## Next task

**Phase 11, Task 4f2: Razorpay, its keys, and the sweeper.** Read ADR-220,
`internal/gateway/gateway.go`, `internal/config`, and Razorpay's Payment Links API
from its own documentation (Context7 or the vendor's pages), not from memory.
- An adapter, `internal/gateway/razorpay`, on `net/http` with timeouts: Start makes
  a payment link with our reference; Status reads it. Test it against an
  `httptest` server speaking Razorpay's JSON; no call to the real one in a test.
- Where the key id and secret are kept: config and environment (as `relay_key` is)
  or the settings table. Say why. Never in a log, an error or an API answer.
- A sweeper owned by a struct with `Close`, calling `charges.Sweep`, wired in
  `cmd/mez`. It must fail quietly. Split in two (adapter; keys and sweeper) if so.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 to 4e4, 4f1 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

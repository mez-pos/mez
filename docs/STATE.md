# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 and 2a done.
Started before the owner's walk: the rebuild, 09 and 10 still owe their human checks.

## Last completed task

**Phase 11, Task 2a: the tunnel client** (ADR-202). `internal/tunnel` dials the relay
with `relay_key`, serves guests through the server's own handler, reconnects with
backoff and a heartbeat, and is `checks.tunnel` in health. Every guest path lands under
`/order/`, out of reach of the staff API. Proved by a test that kills the relay
mid-service (`internal/api/tunnel_test.go`) and by hand with both binaries.

## Next task

**Phase 11, Task 2b: tunnel status in back office.** `tunnel.Client.Status()` already
has the state, since when, the last error and the next retry.
- Add an endpoint that returns it (`make generate` for the types), for an owner or
  manager. No relay set is not a fault. States: `connecting`, `connected`, `waiting`,
  `refused` (wrong key), `replaced` (another machine has the key), `stopped`.
- Show it where the owner looks (`docs/design/screens/README.md`: Settings or
  Devices), as colour plus a word.
- Task 3: mount what guests need under `/order/`, and build the guest bundle with
  relative URLs (it names `/order/assets/...`, wrong behind `/v/<venue>/`).

## Known broken / in progress

- A dev server may be running from an old binary: a Go change needs `make dev` run
  again, and `make check` cannot rebuild `bin/mez` while one runs from it.
- The relay: no load test, no run behind a real TLS proxy, and open question 7.

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):
1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. Relay (ADR-201): venues share one origin, unsafe for a hosted relay. A subdomain each?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 and 2a done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

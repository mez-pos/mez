# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 and 2 done.
Started before the owner's walk: the rebuild, 09 and 10 still owe it.

## Last completed task

**Phase 11, Task 2b: the tunnel in the back office** (ADR-205). `GET /api/v1/tunnel`
and a card on Devices, "Ordering from phones": a chip, a sentence saying what guests
can do, when it tries next and why it failed. Health reads the same status. Seen in
the browser reconnecting and connected (to the live relay), at 1366 and 400 wide.

## Next task

**Phase 11, Task 3: guest menu and cart**, `web/guest`. Read `phase-11.md`, ADR-202 and
ADR-203 first. What the tasks before it settled:
- The relay is live: `https://relay.jskin.in`, a venue at `<venue>.jskin.in`
  (`build/relay/README.md`). A venue is the root of its own hostname.
- A guest reaches only what is under `/order/` on the server. Mount the guest API
  there, for example `/order/api/...`; it then works on the venue's wifi too.
- The guest bundle names `/order/assets/...`, which the tunnel maps to
  `/order/order/assets/...`: build it with relative URLs, or fix the tunnel's end.
- A request by the tunnel is not marked as one, and a LAN device can send
  `X-Forwarded-*` too. No specs exist for M01 to M11: write this task's first.

## Known broken / in progress

- A dev server may be running from an old binary: a Go change needs `make dev` run
  again, and `make check` cannot rebuild `bin/mez` while one runs from it.
- The relay: no load test. Its signing secret has no backup. A `democafe` test key exists.

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):
1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. Answered: `jskin.in` is for testing, `mez.ac` for production. Move before codes print.

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 and 2 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

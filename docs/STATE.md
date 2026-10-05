# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Task 1 (with 1b,
1c) and 2a done. Started before the owner's walk: the rebuild, 09 and 10 still owe it.

## Last completed task

**Phase 11, Task 1c: the relay is live** (ADR-204) at `https://relay.jskin.in`, venues
at `<venue>.jskin.in`: the owner's Hetzner server behind Cloudflare, Caddy for TLS,
the files in `build/relay/`. Proved with `bin/mez` on the owner's PC and a guest page
fetched over the internet. Update it with `build/relay/deploy.sh` (see its README).

## Next task

**Phase 11, Task 2b: tunnel status in back office.** `tunnel.Client.Status()` has the
state, since when, the last error and the next retry.
- Add an endpoint returning it (`make generate` for the types), for an owner or
  manager. No relay set is not a fault. States: `connecting`, `connected`, `waiting`,
  `refused` (wrong key), `replaced` (another machine has the key), `stopped`.
- Show it where the owner looks (`docs/design/screens/README.md`: Settings or
  Devices), as colour plus a word.
- Task 3: mount what guests need under `/order/`. The guest bundle names
  `/order/assets/...`, which the tunnel maps to `/order/order/assets/...`: fix one end.
  A venue is now the root of its own hostname; there is no path prefix.

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
7. `jskin.in` is on every table's code once printed. Is it the permanent domain?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (1 and 2a done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

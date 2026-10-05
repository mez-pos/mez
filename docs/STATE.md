# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. 1, 1b and 2a done.
Started before the owner's walk: the rebuild, 09 and 10 still owe their human checks.

## Last completed task

**Phase 11, Task 1b: a hostname per venue** (ADR-203). `RELAY_VENUE_DOMAIN=jskin.in`
serves each venue at `<venue>.jskin.in`, the whole path its own, and stops serving
`/v/<venue>/`. Venues dial `relay.<domain>`. Reserved names are never venues. Without
the setting the relay routes by path as before.

## Next task

**Phase 11, Task 1c: deploy the relay**, when the owner hands over access. The owner's
Hetzner server "Gatezzy" (wipe, rename `mez`), `jskin.in` on Cloudflare (proxied,
`relay` and `*`), Caddy in front, the relay from `build/relay.Dockerfile` with
`RELAY_VENUE_DOMAIN=jskin.in` and `RELAY_TRUST_PROXY=true`. Commit the files that do it
to `build/`. No secret goes in the repo or the log. Then **Task 2b**:
- An endpoint returning `tunnel.Client.Status()` (`make generate`), owner or manager.
  No relay set is not a fault. Show it in back office as colour plus a word.
- Task 3: mount what guests need under `/order/`. The guest bundle names
  `/order/assets/...`, which the tunnel maps to `/order/order/assets/...`: fix one end.

## Known broken / in progress

- A dev server may be running from an old binary: a Go change needs `make dev` run
  again, and `make check` cannot rebuild `bin/mez` while one runs from it.
- The relay: no load test and no run behind a real TLS proxy yet (Task 1c).

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (1, 1b and 2a done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

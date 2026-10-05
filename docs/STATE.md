# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Task 1 of 6 done.
Started before the owner's walk: the rebuild, 09 and 10 still owe their human checks.

## Last completed task

**Phase 11, Task 1: the relay service** (ADR-201). `cmd/relay` over `internal/relay`:
a venue dials `GET /agent` with a signed key, guests reach it at `/v/<venue>/...`, and
requests ride one socket as frames (`internal/relay/wire`). It stores nothing. `make
build` makes `bin/relay` too. Tested over real sockets; nothing in mez talks to it yet.

## Next task

**Phase 11, Task 2: the tunnel client**, `internal/tunnel`. What Task 1 leaves it:
- Speak `internal/relay/wire`: dial `/agent` with `Authorization: Bearer <key>`, read
  limit `wire.MaxFrame`, body pieces of `wire.MaxChunk` or less. A 401 is a wrong key
  and 4001 another machine with the same one: back off for long on both.
- Config has `relay_url` and no key yet: add one (a secret, env or file, never logged)
  and hold `venue_id` to `relay.ValidVenueID` when a relay is set.
- **The client chooses what a guest may reach.** Forward guest routes only, never the
  staff API under `/api/v1`. Paths arrive without the `/v/<venue>` prefix; it is in
  `X-Forwarded-Prefix`. Guest pages must use relative URLs.

## Known broken / in progress

- A dev server may be running from an old binary: a Go change needs `make dev` run
  again, and `make check` cannot rebuild `bin/mez` while one runs from it.
- The relay has had no load test and no run behind a real TLS proxy.

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):

1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. Relay (ADR-201): venues share one origin. A subdomain each on the hosted relay?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Task 1 of 6 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

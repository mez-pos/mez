# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. 1, 2 and 3a done.
(The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 3a: the guest's way in** (ADR-206). Each table has a code
(`internal/service/tablecodes`, migration 00038, `mez table-codes`). The guest API is
under `/order/api/` (`t/{code}`, `t/{code}/menu`, `photos/{name}`), with no device or
session. The bundle's files now load through the relay; proved at `democafe.jskin.in`.

## Next task

**Phase 11, Task 3b: the landing and the menu**, `web/guest`, M01 and M02 of
`docs/design/reference/mez/mez Mobile Customer.dc.html` (this task says to read it).
- Write `docs/design/screens/guest-landing.md` and `guest-menu.md` first, and add
  them to that folder's README. Read `docs/design/tokens.md`.
- The app is at `/order/t/<code>` on wifi and `/t/<code>` or `/order/t/<code>` through
  the relay: read the code from the last path segment. Call the API by its absolute
  path, `/order/api/t/<code>/menu`. Types are in `web/shared/api-types.ts` (`Guest*`).
- 60 KB gzipped, 36.6 left. No shadcn, no icon pack, no router library: check what
  `web/shared/ui` costs before importing from it. Photos lazy; fonts must not block.
- A code that leads nowhere is a 404 `table_code_unknown`: a screen of its own (M11d).
- To try it: `mez setup -demo-menu`, `mez table-codes`, then `build/relay/README.md`.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (1, 2 and 3a done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

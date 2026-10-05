# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 and 4e2 done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4e2: the UPI address in the back office** (ADR-216). A card,
"Paid from a phone", under the venue's details on Settings: the address, the name
a guest sees, a standing warning that mez cannot check the address, and what a
guest's bill now offers. `web/backoffice/src/settings/PaymentsCard.tsx`.

## Next task

**Phase 11, Task 4e3: the QR on the phone's bill.** Read ADR-215, `guest-bill.md`.
- For paying from another phone: the same `upi://pay` link the bill already
  carries, as a QR, under the Pay by UPI button.
- The guest's bundle is 39.1 KB of 60. Measure before choosing: a small QR encoder
  written for this (byte mode, one error level, versions enough for a 150
  character link) against a library, by its gzipped cost. Or draw it on the
  server as SVG, in Go with no dependency, and serve it with the bill: nothing is
  added to the bundle, and the encoder gets Go tests. Say which and why in the ADR.
- A QR that does not scan is worse than none: test the encoder against known
  codes, and scan one with a real phone before calling it done, or say it was not.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 and 4e2 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

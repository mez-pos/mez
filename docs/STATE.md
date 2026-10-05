# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Rebuild - the screens follow the mez design** (ADR-143), `docs/phases/rebuild.md`.
Every task is done. What is left is its done-when, which is the owner's walk.
Phases 09 and 10 still owe their human done-when checks too.

## Last completed task

**Rebuild, Task 17h3: this file after the walk.** Task 17 is done: 28 screen specs
written from the code (`docs/design/screens/README.md`), the old words off the
screens, one door for closing the day, and a walk as each role (ADR-196 to ADR-199).
This file is short again: each task's history is in `docs/HISTORY.md` and the
known-broken list in `docs/KNOWN.md`, moved whole. There, "not seen by eye" lost
only what the walk saw, in fifteen places, and three statements no longer true.

## Next task

**The owner's walk, then phase 11.** `make dev`, sign in as Meera (3333), Kabir
(5555), Ravi (4444) and Arjun (2222), take an order from each side through to a
paid bill and a Z-report, and say whether each screen matches its design
(`rebuild.md`, "Done when"). The questions below come first: the first two change
what that walk sees. On a yes, tick the phase and start `phase-11.md`, Task 1.

## Known broken / in progress

- A dev server may be running from an old binary: a Go change needs `make dev` run
  again, and `make check` cannot rebuild `bin/mez` while one runs from it.
- The walk saw each role's spine once, at 1920x1080 and 1366x1024. Not walked: card
  payment on the desktop, split by item, voids, comps, discounts, transfer and
  merge, delivery, reprints, tips, and Menu, Inventory, Staff and Settings.

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):

1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

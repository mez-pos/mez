# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. 1, 2, 3a, 3b done.
(The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 3b: the guest's landing and menu** (ADR-207), `web/guest/src`: the
table and venue, then every dish with a search, categories and diet filters, and a
screen for a code that leads nowhere or a venue that cannot be reached. Specs:
`guest-landing.md`, `guest-menu.md`. 31.9 KB gzipped of 60.

## Next task

**Phase 11, Task 3c: an item with its choices, and the cart**, M03, M04's cart and M11a
of `docs/design/reference/mez/mez Mobile Customer.dc.html` (lines 142 to 250, 524).
- Write `guest-item.md` and `guest-cart.md` first; add them to the screens README and
  update `guest-menu.md` (Add on a card, the View Cart bar: both listed as left out).
- The menu's `modifier_groups` have `min_select`, `max_select` and `is_default`. The
  till's rules for choices are in `web/shared/menu.ts`: read it before writing any.
- The cart is the phone's own (no server until Task 4): keep it by table code, in
  `localStorage`, and drop lines whose dish has gone or is sold out when the menu is
  read again. Money stays integer minor units; total with `shared/money.ts`.
- 28.1 KB left. Import by file, not from `shared/ui`'s index; check `Sheet`'s cost.
- Not ordering yet: the cart's button must not say it sends anything (Task 4).

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (1, 2, 3a and 3b done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

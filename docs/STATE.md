# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4 but 4f6b (Cashfree, question 8), 5 but 5f3b on, and 6 done. (Walks owed.)

## Last completed task

**Phase 11, Task 5f3a: ordering to collect on the phone** (ADR-251). 5f3 split in
two. `/order/p/<code>` is the table's screens with a `collect` flag, and the
pickup code in large type on the order. `guest-collect.md`. Seen with a mocked API.

## Next task

**No real charge has been made yet** (ADR-221): with test keys, on a phone, Pay
online on a bill, a share from two phones, and Pay on an order paid first.

**Phase 11, Task 5f3b: the pickup code in the back office.** Read ADR-247,
ADR-244, ADR-245, `backoffice-floor.md`, `internal/api/table_codes.go`.
- On the Floor plan's "Ordering from the table" card: the venue's pickup code
  and address (`pickup` on `GET /api/v1/table-codes`), Show the QR code
  (`/api/v1/pickup-code/qr`), Renew and Turn off behind a confirm, Make one.
- On the sheet to print: a card for the door. Say where no gateway is set that
  the code leads to "not taking orders" (`can_charge` on the payment settings).
- After a renew a paid guest keeps their code but cannot ask how the order stands.

## Known broken / in progress

- A dev server left running blocks `make check` from rebuilding `bin/mez`: stop it first.
- The relay: no load test, no backup of its signing secret, a `democafe` test key. Its
  1200 a minute per venue is under what a full room's phones ask (ADR-246).

## Open questions for the human

From the walk (ADR-199; each in full at the top of `docs/KNOWN.md`):
1. The default manager cannot edit the menu, against the flow map. Which gives way?
2. "Not printed" counts kitchen tickets at a station with no printer. Should it?
3. The Z-report slip mixes before and after credit notes. Add a credit-note line?
4. Tax-inclusive totals read as if the rows add up. Say "incl." on the tax rows?
5. The desktop's cash quick amounts never offer ₹500. Show it?
6. Older: move a data directory from before the rename and rename `nofiltr.db`.
7. `jskin.in` is for testing, `mez.ac` for production: move before codes are printed.
8. Cashfree needs each payer's phone number (ADR-230). Ask guests for one, or leave it out?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 4, 5 but for 4f6b, 5f3b, 5f4; 6 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

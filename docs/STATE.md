# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d and 4e1 to 4e3 done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4e3: the QR on the phone's bill** (ADR-217).
`GET /order/api/t/{code}/bill/upi.svg` draws the bill's UPI link as a QR code, by
an encoder of our own (`internal/domain/qr`, no dependency), checked module for
module against python-qrcode and read back by a decoder. The phone shows it under
"Pay from another phone". No phone has scanned one yet.

## Next task

**Phase 11, Task 4e4: "I have paid".** Read ADR-209, ADR-211, ADR-212, ADR-215,
`terminal-requests.md`, `guest-bill.md`. Likely server then screens: split if so.
- A guest who paid by UPI taps "I have paid" on the bill. The terminal says so, as
  it says a request is waiting (the same pill, sheet and chime, or beside them),
  with the table and the amount, and leads staff to that table's payment.
- It records nothing as paid: staff check their own phone and record the UPI
  payment on the tender screen, as now (ADR-215). Say that on both screens.
- Decide where the notice is kept (a row, as `guest_rounds` is, or on it), how one
  is cleared (the payment recorded, the bill closed, or dismissed), and what
  stops a guest tapping it forty times.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 to 4e3 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d and 4e1 done. (The rebuild, 09 and 10 still owe the owner's walk.)

## Last completed task

**Phase 11, Task 4e1: a UPI pay link on the phone's bill** (ADR-215). The venue's
UPI address is a setting (`GET`/`PUT /api/v1/settings/payments`,
`payment.upi_address`). The server builds the `upi://pay` link for the balance
(`payment.UPIAccount.Link`) and the bill carries it; the phone shows "Pay ₹540.00
by UPI". Staff confirm the payment on the terminal, as at the till. 4e split in five.

## Next task

**Phase 11, Task 4e2: where the owner sets the UPI address.** Read ADR-215,
`backoffice-settings.md`, `docs/design/tokens.md`, and one existing card as the
pattern (`web/backoffice/src/settings/ReceiptCard.tsx`, `use-section-draft.ts`).
- A card on Settings: the UPI address and the payee name, saved with
  `PUT /api/v1/settings/payments` (`PaymentSettingsResponse` is generated).
- Say beside the field that the address cannot be checked: the owner should pay
  it a rupee from their own phone before guests do.
- A venue not in INR keeps the card and is told UPI pays in rupees only.
- Update the spec in the same task. Then 4e3, the QR.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d and 4e1 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

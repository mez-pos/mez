# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 11 - Relay and guest ordering**, `docs/phases/phase-11.md`. Tasks 1 to 3,
4a to 4d, 4e1 to 4e4, 4f1 and 4f2 done. 4e5 waits for 4f. (The walks are owed.)

## Last completed task

**Phase 11, Task 4f2: Razorpay, its keys, and the sweeper** (ADR-221).
`internal/gateway/razorpay` uses payment links. `gateway`, `gateway_key_id` and
`gateway_key_secret` are configuration (`MEZ_GATEWAY_KEY_SECRET`), not settings.
`charges.Sweeper` settles open charges every 20 seconds. No real call made yet.

## Next task

**Phase 11, Task 4f3: paying online from the phone.** Read ADR-214, ADR-215,
ADR-220, ADR-221, `guest-bill.md`. Likely server then screen: split if so.
- `charges.Service` reaches the API as an option, as the orders do. With no gateway
  set the routes are not there and the bill says nothing of paying online.
- `POST /order/api/t/{code}/bill/charge` starts a charge and answers its address;
  the return address is the guest's own bill, built on the server from the
  request's host, never taken from the phone.
- `GET .../bill/charge/{id}` settles and answers how it stands. The bill says
  whether the venue takes payment online, so the phone knows to offer it.
- The phone: "Pay online", the gateway's page, and coming back to paid, waiting,
  failed, or taken and not on the bill. Then try a real charge with test keys.

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
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (Tasks 1 to 3, 4a to 4d, 4e1 to 4e4, 4f1, 4f2 done)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 13 - The back office from anywhere**, `docs/phases/phase-13.md`,
opened 2026-10-08 (ADR-306). The owner reaches the back office through
the relay from a phone; the relay carries it only for a venue whose key
was issued with `--remote`; the API holds a relayed request to tighter
rules. Five tasks. Phases 00 to 12 are built; what they still owe is in
`docs/KNOWN.md` and in the "Still owed" list below.

## Last completed task

**Phase 13, Task 3a - The server's rules for a relayed request** (ADR-308).
Task 3 was split in two. `internal/api/remote.go`: a gate in front of the
routes that acts only on `X-Mez-Via`. Relayed, a request reaches `/order/`,
`/backoffice`, and the API routes on an allow list (areas `menu`, `stock`,
`people`, `audit`, `reports`, and about sixty named routes); anything else
is 403 `remote_not_allowed`. Back office devices only; only staff holding
`menu.edit`, `settings.edit`, `reports.view` or `stock.manage`, checked at
sign-in and on every request. `via=relay` in the access log. A new route
needs a verdict or `TestEveryRouteHasARemoteVerdict` fails.

## Next task

**Phase 13, Task 3b - The back office knows it is away.** Front end only.
Read `X-Mez-Via: relay` off the probe's answer in
`web/shared/use-realtime.tsx`; a realtime state that stops dialling the
stream; the pill says "Away from the venue"; a `resync` notice on focus;
a plain sentence for a 403 `remote_not_allowed`; hide or disable what
would only be refused. The list is in the phase file.

Still owed from before, in the order that unblocks the most:
1. **The drawer.** `mez --test-print <ip> -drawer`, then a cash sale; no
   drawer was at the venue on 2026-10-08 (`hardware.md`, ADR-282).
2. **A release.** `v0.1.0-alpha.2` is tagged on the human's machine only
   and is behind `main`: tag again (`docs/releasing.md`); expect the first
   workflow run to need a second try.
3. **Phase 12's "done when"**: a clean Windows machine, the downloaded
   installer, a first printed invoice in under ten minutes.
4. The human checks phases 09 to 11 owe, and the questions below.
5. Decide the guest app's licence: the README says it was meant to be
   MIT and is under the AGPL.

**Do not move Litestream past v0.5.15 without running the tests on
Windows.** `make check` embeds whatever `web/*/dist` holds: run `make
build-web` before walking a screen against `bin/mez`.

Still owed from phase 11, its human check and:
- **No real charge has been made yet** (ADR-221): with test keys, on a phone, Pay
  online on a bill, a share from two phones, and Pay on an order paid first.
- **Ordering to collect, end to end, with a gateway's test keys**: make the code
  on the Floor plan, scan it, order, pay, see the code, find it under To collect,
  Handed over. None of 5f2 to 5f4 has been seen against the real binary.

## Known broken / in progress

- **`remote_backoffice` works and is held to ADR-308, but is rough from
  outside until Task 3b**: no stream, so the pill says "Reconnecting", and
  buttons for what stays at the venue answer 403. Unseen on a real phone.
  The live relay at relay.jskin.in is not redeployed with Phase 13 yet.
- **Between phases (ADR-256 to ADR-262): the first security review and
  what the owner decided.** A till's line is priced from the menu on the
  server; a relay must be TLS; revoking a device cuts its stream; the log
  masks table codes; the toolchain is go1.26.8 (ADR-256 to 258). Then:
  printers on the LAN only and the data directory the owner's on Windows
  (259); a failing PIN hard-locks until a manager or `mez unlock` clears it,
  six digits for powerful roles (260); a tip over the venue's ceiling needs
  a manager (261); a guest's phone holds a session for the sitting (262).
  Only the LAN's plain HTTP is left, for phase 12. Walked in a browser
  against the dev venue on 2026-10-07: a first phone lands, orders, is
  accepted and reads the bill; a second phone at the open table sees "Ask a
  member of staff", its order is a "New phone at Table T3" request, and
  acceptance hands it a session; Kabir hard-locked shows Locked on People
  and Unlock lets him sign in; a ₹100 tip on ₹150 warns, opens the approval
  sheet, Priya approves, the event carries her id and Audit lists it. No
  console errors on any screen.

- **Between phases (ADR-255): the screens now fit 1024 to 2560 wide and 768
  tall.** Fluid `w-side`, `w-rail`, `w-panel` widths, auto-fill card grids, a
  `short:` variant, script lines out of the working chrome (tokens.md,
  "Layout"). Checked in a browser at 1024, 1366, 1920 and 2560 on the main
  screens; the back office's menu, stock, settings, people and printers
  screens were not walked at 1024 and may still show a tight column.

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
8. The firewall rule is for Private and Domain networks (ADR-265). Public too?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (built; owes its human check)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs (built; owes its human check)

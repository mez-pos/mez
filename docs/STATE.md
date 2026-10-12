# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 14 - mez Cloud: the account, the pairing and the switch**,
`docs/phases/phase-14.md`, opened 2026-10-12 at the human's word (ADR-312,
ADR-313). Nine tasks; Tasks 2, 3, 5, 6 and 9 are this repo's, 4 and 7 are
both, 8 is `mez-cloud`'s. Tasks 1 to 3, 4a and 4b are done (ADR-313, ADR-315 to ADR-320). Phase 13's five tasks
are built and it owes its human check: the walk on a real phone below.
Phases 00 to 12 are built; what they still owe is in `docs/KNOWN.md` and
in the "Still owed" list below.

## Last completed task

**Phase 14, Task 4b2 - Pairing while running** (ADR-320), which closes 4b.
`POST /api/v1/cloud/pair {code}`, for `settings.edit`, at the venue only,
six tries a minute. `internal/service/cloudpair` refuses a venue with a
relay in its settings file, a code that could not be one, before anything
is sent; once mez Cloud accepts the code the pairing is kept first and
stands, and a tunnel or a backup that failed is reported beside it
(`restart_needed`, `backup: set | kept | failed | none`). It takes hold
with no restart: `cmd/mez`'s `venueLink` holds the pairing, the tunnel
and the venue's address, and the API reads each through it. A venue's own
backup is kept. **No screen calls the route yet, and it has not been run
in one piece from a typed code to a guest's request.**

## Next task

**Phase 14, Task 4c - The screens.** The setup wizard's "Connect to mez
Cloud" step and the same entry on the Devices screen's "Back office from
anywhere" card, calling `POST /api/v1/cloud/pair` and showing `paired_to`
from `GET /api/v1/tunnel`. Read `docs/design/tokens.md`,
`docs/design/screens/backoffice-devices.md` and the wizard's screen file
first. Walk it in a browser against a stand-in pairing server and a local
relay: it is the first time the whole of pairing runs in one piece. Each
refusal's words are already in `internal/api/cloud_pair.go`.

**The live relay** needs redeploying only if entitlements are wanted on
it: with `RELAY_ENTITLEMENTS` unset it carries every key, as before. A
venue must run this build or later to show where it stands.

**The live relay's venues are safe to give a `--remote` key now**: the
venue's binary must be one built from this commit or later, since the
stricter sign-in is the venue's and not the relay's.

**Before or beside it, the human's:** the live relay at relay.jskin.in was
deployed with Phase 13 on 2026-10-08 (`v0.1.0-alpha.4`, health ok, no
venue connected at the time). Still to do: choose the friend's cafe its
name, `relay issue-key <name> --remote` on the box, the key and
`remote_backoffice = true` in the cafe's `config.toml` with the new
installer, and Task 4's walk on a real phone over mobile data.

Still owed from before, in the order that unblocks the most:
1. **The drawer.** `mez --test-print <ip> -drawer`, then a cash sale; no
   drawer was at the venue on 2026-10-08 (`hardware.md`, ADR-282).
2. **A release.** `v0.1.0-alpha.4` is tagged on this machine at Task 4's
   commit and built: `dist/` has the four binaries, the Windows installer
   and the checksums. **The tag is not pushed**: pushing one runs the
   release workflow, which has never run and publishes a Docker image
   (`docs/releasing.md`). `main` is pushed.
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

- **The back office from away is built and unseen on a real phone**
  (ADR-306 to ADR-311, ADR-314). The live relay runs it; no venue has a
  `--remote` key yet.
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
9. ADR-313 took the cost paper's prices (₹999 and ₹9,999; founders ₹699 and
   ₹6,999, first hundred). Are they right, and are they shown with GST or before it?
10. After a restore, an invoice number printed in the half minute the backup had
   not yet sent is given again to the next sale (ADR-315). Is that acceptable for
   GST, or should a restore skip numbers ahead?

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (built; owes its human check)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs (built; owes its human check)
- [ ] 13 - The back office from anywhere (built; owes its human check)
- [ ] 14 - mez Cloud: the account, the pairing and the switch (Tasks 1 to 3, 4a and 4b of 9)

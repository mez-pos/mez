# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 14 - mez Cloud: the account, the pairing and the switch**,
`docs/phases/phase-14.md`, opened 2026-10-12 at the human's word (ADR-312,
ADR-313). Nine tasks; Tasks 2, 3, 5, 6 and 9 are this repo's, 4 and 7 are
both, 8 is `mez-cloud`'s. Every task that is this repo's is done: 1 to 3,
4a to 4c, 5, 6, 7a to 7d, 7f and 9 (ADR-313, ADR-315 to ADR-340). 4d, 7e
and 8 are `mez-cloud`'s, a private repository with a skeleton (ADR-344). Phase 13's five
tasks are built and it owes its human check: the walk on a real phone below.
Phases 00 to 12 are built; what they still owe is in `docs/KNOWN.md` and
in the "Still owed" list below.

## Last completed task

**`mez-cloud`'s Task 7e4b, its part here** (ADR-349). The checks the
venue's client made of a hand-over's answer are `cloud.CheckGrantAnswer`,
handed out as `cloudkit.CheckGrantAnswer`, so that the cloud asks it
before it spends the owner's opened link and commits the hand-over, as
it does at pairing (ADR-347). With it `mez-cloud`'s Task 7e is done
(its C-022): a hand-over gives a key that reads for an hour and a
pairing code for the venue's id alone. Before it, 7e3a (ADR-348): the
grant's waiting (202) and held (423) answers.

## Next task

**Nothing in this repository is next by itself.** Phase 14's work is now
in `mez-cloud`: open that repository and follow its `docs/STATE.md`
(Task 8c3b there, with 7e, 8g, 8c1, 8c2 and 8c3a done). This repository is touched again when:
- `mez-cloud` needs a shape cloudkit does not have: add it here first,
  with an ADR, then use it there (its `CLAUDE.md` says so);
- Task 8e there begins, by moving the mirror out of `cmd/mez`'s
  `package main` to where cloudkit can hand it out;
- 7e there is built: the wizard's wait for the email link (ADR-342):
  ADR-348 has the shape, and until then the wizard shows its words as a
  refusal and the owner chooses the venue again by hand;
- the service opens: take "**mez Cloud is not taking venues yet**" out
  of `deploy.md`;
- this module is tagged with cloudkit in it, so that `mez-cloud`'s
  `go.mod` can name the tag and drop its `replace`. Tagging and pushing
  are the human's.
Here, by itself: the walk of ADR-343's seven changes (below), a
rotation walked with `pairstub -rotated` (pair, sign in from away with
`GET /dev/assertion`, see `public_key` in `cloud.json` change), and a
restore that skips invoice numbers ahead, which waits on the accountant.
**To walk "The PC died" on one machine:** `RELAY_SECRET=<48 hex>
bin/pairstub.exe -listen 127.0.0.1:7812 -bucket-fake 127.0.0.1:7813`
(build it with `go build -o bin/pairstub.exe ./build/dev/pairstub`);
`MEZ_OWNER_PIN=246813 bin/mez setup -data-dir OLD -name ... -owner Priya
-timezone Asia/Kolkata -demo-menu`; `bin/mez -data-dir OLD -listen-addr
127.0.0.1:7791 -cloud-url http://127.0.0.1:7812`; the tablet is a
browser profile at `127.0.0.1:7791`, paired with setup's code; pair the
venue with the code WXYZ2345 (`POST /api/v1/cloud/pair`); wait 45
seconds; kill it; `bin/mez -data-dir NEW` with the same two flags, so
the same address; the wizard in another profile at
`localhost:7791/setup#token=<NEW/setup-token>`, which is another origin
and holds nothing. The account is +919876500000 and its code 123456.
Run `make build-web` first.

**A mirror killed and not stopped leaves `mez-mirror-*` in the system's
temporary folder, and one left there fails
`TestAMirrorNobodyIsLookingAtAsksNothing`.** After walking a mirror, stop
it with Ctrl-C, or remove what it left. To walk one: `mez setup
-demo-menu` into a scratch data directory, a `backup.json` there naming a
folder, `bin/mez -data-dir` long enough to back up, then `bin/mez mirror
-from <folder> -state-dir <dir>`.

**Task 4d is `mez-cloud`'s**: that repository does not exist yet, and
its first commit copies this phase's cloud tasks there. ADR-318 has the
request and the answer `POST /v1/pair` must give, and ADR-323 the
assertion it signs (`cloud.NewAssertion`, `cloud.SignAssertion`) and
ADR-324 how it is presented and what is sent after. To
walk pairing: `RELAY_SECRET=... bin/relay`, `go run
./build/dev/pairstub`, and `bin/mez -cloud-url http://127.0.0.1:7802`.

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
5. The product's domain: the human buys `mezpos.com` later, and the relay
   moves to it before any venue that sells prints its table codes
   (ADR-342). `jskin.in` until then.

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

None is open. The fourteen that stood here were answered on 2026-10-13
(ADR-341 for where `mez-cloud` lives, ADR-342 for the rest). Two answers
wait on the human's accountant before anything is published or built on
them: prices shown before GST, and a restore skipping invoice numbers
ahead. One waits on real tablets: whether the server should answer to a
name.

**Decided and not built** (ADR-342): a restore skips invoice numbers
ahead and audits the gap. ADR first, and the accountant's word before
it. The other seven were built on 2026-10-13 (ADR-343), **and none was
looked at in a browser**: the "incl." tax rows on the rail, the cash row
on a 200.00 bill, the Z-report slip after a credit note, the Menu pill
for a manager, the mirror's pill, and a station with no printer sending
a ticket with the Print queue left at nothing. Walk them when next at a
screen. Migration 00056 gives `menu.edit` to existing managers.

---

## Phase progress

- [x] 00 to 08 - Skeleton, server, SQLite, domain, API, shell, terminal, payment, floor and kitchen
- [ ] 09 - Shift, blind drawer count, Z-report, tips (built; owes its human check)
- [ ] 10 - Back office: menu editor, inventory, reports (built; owes its human check)
- [ ] Rebuild - the screens follow the mez design (built; owes the owner's walk)
- [ ] 11 - Relay service, outbound tunnel, guest QR ordering (built; owes its human check)
- [ ] 12 - Installer, Windows Service, Docker, Litestream, docs (built; owes its human check)
- [ ] 13 - The back office from anywhere (built; owes its human check)
- [ ] 14 - mez Cloud: the account, the pairing and the switch (this repo's tasks are done; 4d, 7e and 8 are `mez-cloud`'s)

# STATE

> The handoff file between sessions. Rewrite the top four sections at the end of
> every session. Keep it under 60 lines: it is read first, every time. The task
> before the last goes to the top of `docs/HISTORY.md`; what is broken, left out
> or unseen is in `docs/KNOWN.md`, by screen. Neither is read at the start.

## Current phase

**Phase 17 - Every screen at the venue's own size**,
`docs/phases/phase-17.md`, opened 2026-10-31 (ADR-381): the fixes from
the UI audit (`docs/design/ui-audit-2026-10-31.md`). Tasks 1 and 4 to
12 are done: at the human's word the device-fit pass came first, built
and seen at 1024x768, 1366x768 and 1920x1080 on 2026-10-31 (ADR-381,
ADR-382; what was not seen is the top of `KNOWN.md`). The two P0s,
Tasks 2 and 3, are done too (ADR-383), and Tasks 13 to 33 (ADR-384 to ADR-408; ADR-400 was out of phase): the phase is done, 2026-11-01. It runs before Phase 16, which is mostly
`mez-cloud`'s.

Before it, **Phase 15 - The owner's WhatsApp**, `docs/phases/phase-15.md`, opened
2026-10-22. Seven tasks; Task 1 (the ADR) and Task 4 (the health summary
over the relay connection) are this repo's, Task 7 is words in both, and
Tasks 2, 3, 5 and 6 are `mez-cloud`'s. Phase 14's tasks that were this
repo's are all done (ADR-313, ADR-315 to ADR-359); its 4d, 7e and 8 are
`mez-cloud`'s and live there. Phase 13's five tasks are built and it owes
its human check: the walk on a real phone below. Phases 00 to 12 are
built; what they still owe is in `docs/KNOWN.md` and in the "Still owed"
list below.

## Last completed task

**TBS-20, the waiter's payment**, 2026-11-01, after Phase 17. The
waiter's bill let go of a payment by calling itself
(`releaseUnsettled`, a find-and-replace in 2724b4ca, ADR-376): every
recorded payment threw "Maximum call stack size exceeded", which is no
refusal, so the share was then held as "may have been recorded" and its
check threw again. One share showed the error over "Paid in full"; on a
split the next share could not be taken. The pay screen and the waiter's
bill now share one hold, `unsettledHold` in
`terminal/src/tender/unsettled-store.ts`, tested there. Walked on a demo
venue at 1024: the old build stuck on Guest 2, the fixed one took a split
by UPI and cash to "Paid in full" and closed the table; a till card
payment as well. The Windows installer is built again from this commit.

## Next task

**Here: the re-run's other two P1s** (TBS-20 is fixed), TBS-21 (the approval sheet: scrolled, the cashier
chosen, the waiter and kitchen listed) with TILL-26, and TILL-25 (a
percentage discount keyed from the right, the approval never restating
it). The 20 P2s and 35 P3s wait on the human (Open questions). Phase 17 is
done; Phase 16 is mostly `mez-cloud`'s. Each screen is checked by eye at
1024x768 and 1366x768 before it is ticked: the audit's walk (a demo venue
from `mez setup -demo-menu` in a scratch data directory, a scripted
Chromium) is how. Tailwind's variant order bites (ADR-382): a pixel
breakpoint never shares a property with `xl:` or `short:`.

**The human's, at the venue:** the walk of ADR-366 and ADR-367 on the venue PC (the first
item in `KNOWN.md`): the installer is
`dist/mez_v0.2.0-beta.1-32-g<commit>_windows_amd64_setup.exe`, built
2026-11-01 from the TBS-20 fix with all of Phase 17 in it, untagged
(`git describe` names it). Not `mez_v0.2.0-beta.1_windows_amd64_setup.exe`
(ADR-380, the beta tag, 2026-10-26), which has TBS-20 and none of Phase 17,
nor the untagged `-16-gc21c4389` one of 2026-10-31, which has TBS-20 too.
Install it over the old one
(no uninstall: `C:\ProgramData\mez` and `config.toml` are kept), take
`relay_url`, `relay_key` and `public_url` out of
`C:\Program Files\mez\config.toml` so the venue leaves relay.jskin.in,
restart the PC, click the shortcut, then pair the venue to mez Cloud with
a code from app.mezpos.com (the venue `nofiltr`, so
`nofiltr.mezpos.com`; Devices, **Connect to mez Cloud**), then the walks
ADR-372 to ADR-379 list there. mez Cloud's box runs its 8i3b2b build:
pairing, backup and the back office from away work; Phase 15's messages
are built there and not deployed. Every item the audits owed in code is built (ADR-372
to ADR-379); what remains in `KNOWN.md` is small and waits on a walk.
Otherwise **nothing in this repository is next by itself.** Phase 15's Tasks 2,
3c's cloud half, 5 and 6 are `mez-cloud`'s: open that repository and
follow its `docs/STATE.md`. Task 7, the words, waits on a real
close-of-day message. Here, when next at a screen: walk the Shifts
card's **Open on** rows (tick a day, set 17:00 to 02:00, see "next
morning", save, reload), **not yet looked at in a browser** (`KNOWN.md`).
Phase 14's remainder is in `mez-cloud` too (mez Cloud runs on its
Contabo box since 2026-10-21; 8i3b2b and 8h3 there wait on a sender of
codes, the human's to set up). This repository is also touched when:
- **a change to `web/shared`** is a change to `mez-cloud`'s account area too:
  its bundle reads mez's tokens, preset, fonts, `ui/Button`, `ui/Card`,
  `ui/Logo`, `ui/Money`, `ui/StatusChip` (its `cloud` vocabulary, ADR-351),
  `screens/setup/fields`, `api` and `screens/setup/recover-logic`
  from this checkout (its C-031), and its build breaks if they do;
- `mez-cloud` needs a shape cloudkit does not have: add it here first,
  with an ADR, then use it there (its `CLAUDE.md` says so);
- the back office is handed a session in its fragment (ADR-355): a
  change to `web/shared/auth` is a change to what the cloud hands it;
  and the cloud's build must embed mez's bundles (`-tags frontend`)
  for a mirror there to show a screen (ADR-353);
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
2. **A release.** `v0.2.0-beta.1` is tagged locally at ADR-380's commit
   (2026-10-26, **not pushed**) and built: `dist/` has the four binaries
   and the Windows installer. `v0.1.0-alpha.4` (2026-10-08) is the last
   tag pushed.
   **The repository is github.com/mez-pos/mez since 2026-10-20**, private,
   its history rewritten (every commit id changed). Its tags were pushed
   there that day, which started the release workflow three times; all
   three were cancelled before anything was published, and **the Release
   workflow is disabled** in that repository until a release is decided
   (`gh workflow enable Release -R mez-pos/mez`). Pushing a tag publishes a
   Docker image and a release (`docs/releasing.md`). `main` is pushed.
3. **Phase 12's "done when"**: a clean Windows machine, the downloaded
   installer, a first printed invoice in under ten minutes.
4. The human checks phases 09 to 11 owe, and the questions below.
5. The product's domain is `mezpos.com` (ADR-313): mez Cloud's relay
   answers for `*.mezpos.com` since 2026-10-21. `relay.jskin.in` is the
   human's own relay from Phase 13, kept until the venue is paired to mez
   Cloud and its table codes reprinted (ADR-342); then it can go.

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

**Where do the re-run's findings go?** (Phase 17 Task 33, 2026-11-01.)
3 P1, 20 P2 and 35 P3 are in `docs/design/ui-audit-2026-10-31.md`,
"Re-run, 2026-11-01". The P1s are put next above. For the rest: tasks
added to Phase 17, a new phase before Phase 16, or after it.

None other is open. The fourteen that stood here were answered on 2026-10-13
(ADR-341 for where `mez-cloud` lives, ADR-342 for the rest). Two answers
wait on the human's accountant before anything is published or built on
them: prices shown before GST, and a restore skipping invoice numbers
ahead. The one that waited on real tablets, whether the server should
answer to a name, is built: `mez.local` (ADR-379), to be seen on a phone.

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
- [ ] 15 - The owner's WhatsApp (Tasks 1, 3a and 4, this repo's, done: ADR-360 to ADR-362; 3c is this repo's next; 2, 3b, 5 and 6 are `mez-cloud`'s; 7 waits on them)
- [x] 17 - Every screen at the venue's own size (done 2026-11-01; the re-run's new findings wait, above)

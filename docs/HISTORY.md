# History

> What each task did, newest first. Moved out of `STATE.md` on 2026-10-05 (rebuild
> Task 17h3) so that file stays short. **Not read at the start of a session.** Read
> an entry here only when a task needs to know what an earlier one did; the
> reasons are in `DECISIONS.md` under the ADR each entry names.
>
> When a task is done, its entry in `STATE.md` under "Last completed task" replaces
> the one before it, and the one before it is put at the top of this file.

**The venue PC's three complaints, a stability audit, the dead ends, and
every owed fix from the audits, 2026-10-24 to 26** (ADR-366 to ADR-379),
outside the phase plan at the human's word. ADR-376 to ADR-379 (the
26th): a stuck print job frees itself, a paid table takes a waiter's new
ticket, the waiter's bill keeps a lost payment, retries replay under one
key, six small doors; cash rounds once at the end to the venue's step;
a shift's cash is kept as it happens; the server answers for
`mez.local`. ADR-372 to ADR-375, done
at the human's "go ahead": a paid ticket rolls when the party orders
again, `discount.removed`, an order carries the taxes it opened under,
the shift close projects the day once, the Windows service reports
checkpoints while stopping and has a minute at shutdown, and daily
housekeeping. ADR-371: the back office opened from the till leads back
to it (the Till pill), a till that cannot close the day is told so
instead of being bounced between two redirects, and nine smaller ways
out. The
shortcut now runs `mez open`, which starts or waits for the service and
then opens the till; every user may start the service; a double-clicked
`mez.exe` opens the till where the service is installed; the firewall
rule is for every profile; a `listen_addr` the computer does not have yet
is waited for. Pair a device shows each address with a QR code that lands
the device paired. Three audits (orders and money, server and queues, the
terminal) found no critical defect; the high and medium ones are fixed
(ADR-368 to ADR-370) and the rest are listed at the top of `KNOWN.md`.
**None of it seen on a venue PC or in a browser yet**: the walk is the
first item in `KNOWN.md`. Before it, Phase 15 Task 6d (ADR-364), the
last report at the venue, **not looked at in a browser** either.

**Phase 15, Task 3c, this repo's part: the watched events** (ADR-363).
`internal/service/watched.Since` reads the venue's event stream past an
id and answers the voids, comps, refunds, credit notes and discounts in
it, each worth what the order's projection says (as the audit trail
values them), with the actor and the authoriser by name and whether
their role holds `shift.manage`, the table and the line; and the id of
the last event read. cloudkit hands out `Replica.Watched`,
`Replica.LastEventID`, `WatchedEvent`, `WatchedPerson` and the kinds.

**Phase 15, Task 3a: the replica and the close-of-day figures**
(ADR-362), mez's part of `mez-cloud`'s reader. `mirror.Replica` is a
mirror less its door, its server and its photos: `OpenReplica` restores
the newest copy, `Refresh` takes a newer write and puts the old copy
away at once, `Close` puts it away; one goroutine's.
`internal/service/closeofday.Since` reads ADR-360's thirteen lines'
figures off each `z_reports` body past a row id (`ZReportsAfter`),
nothing recomputed, a correction a row of its own. cloudkit hands out
`Replica`, `ReplicaOptions`, `OpenReplica`, `CloseOfDay`, `CloseTally`
and `MoneyText`, the slip's rendering without its symbol.

**Phase 15, Task 4: the venue's health over the relay connection**
(ADR-361), in three parts. 4a: `wire.TypeHealth` and `wire.Health`; a
relay that reads them says `X-Mez-Tunnel-Health: 1` on the dial's
answer; `relay.HealthSink`, which an `Entitlements` may also be, gets
each venue's newest summary on the venue's own goroutine; cloudkit hands
it out. 4b: opening hours are the setting `venue.opening_hours`
(`internal/domain/hours`, a JSON week, `mon` to `sun`, a close before
the open is the next morning), on the Rules API as `opening_hours` and
on the Shifts card as **Open on**, a row per weekday. 4c:
`internal/service/health` composes the summary (printers' last success
and failure from `PrintOutcomesForVenue`, the backup's status, the open
shift, the hours); the tunnel sends it on connecting, within a minute of
a change and every five minutes, to a relay that asked. Wired in `main`.

**Phase 15, Task 1: ADR-360, the owner's WhatsApp** (ADR-360; the
phase file had said ADR-314, a number already spent). The close-of-day
message is thirteen fixed lines read from the `z_reports` row, never
recomputed, one message per row and a second for a correction. Six
alerts on one template, with trigger, threshold and default each: void
or refund over ₹500 (on), discount by a role without `shift.manage`
(off), shift open two hours past closing (on), venue not heard from for
three hours inside opening hours (on), printer or backup failing for an
hour (on). Quiet hours 00:00 to 07:00 hold alerts into one digest and
never hold the close. Once per event, keyed on the row that caused it.
Twelve alerts a trading day, then one "more at app.mezpos.com". The
owner's number plus two confirmed recipients; STOP removes one.

**`mez-cloud`'s Task 8e1, its part here** (ADR-353). The mirror is
`internal/mirror`'s now, not `cmd/mez`'s `package main`: `mirror.Open`
restores a backup into a directory the caller names, and the `Mirror`
it answers is an `http.Handler` that follows its backup while it is
asked things and removes every copy it made when closed. `mez mirror`
is the command round it and says what it said. cloudkit hands it out
over a bucket (`OpenMirror`), tested by letting the owner in on an
assertion through the door alone. The health check's mapping is
`internal/dbhealth`'s, shared by both servers. Before it, 7e4b
(ADR-349): the cloud checks a hand-over's answer with the venue's check.

**`mez-cloud`'s Task 7e4b, its part here** (ADR-349). The checks the
venue's client made of a hand-over's answer are `cloud.CheckGrantAnswer`,
handed out as `cloudkit.CheckGrantAnswer`, so that the cloud asks it
before it spends the owner's opened link and commits the hand-over, as
it does at pairing (ADR-347). With it `mez-cloud`'s Task 7e is done
(its C-022): a hand-over gives a key that reads for an hour and a
pairing code for the venue's id alone.

**`mez-cloud`'s Task 7e3a - the shape, here** (ADR-348). A grant now
has two answers that are neither a hand-over nor a refusal: `202` with
`{"link": {"email", "sent_at"}}` while the link sent to the account's
email is unopened, and `423` with `{"hold": {"reason", "until"}}` while
recovery is held (`email_changed` for a week, `no_email` until one is
set). The client reads them as `*cloud.LinkSent` and `*cloud.Held`,
checked whole; cloudkit hands out the statuses, reasons and bodies, and
the order the cloud decides in (410, hold, link); the restore route says
them as `recover_awaiting_link` and `recover_held`, 409, in words.
`pairstub -link-after 30s` and `pairstub -held` walk each (walked by
curl, not in the wizard).

**`mez-cloud`'s Task 8b3a - the chain, here** (ADR-345). The cloud's
signing key can now be rotated without pairing again, as the human chose
on 2026-10-15: a key link (`k1.<key>.<signature>`) is the next public key
signed by the one before, and an assertion is presented as
`<assertion>~<link>~...`. The venue walks the links from the key its
pairing holds, checks the assertion with the newest key it reaches, and
once the sign-in is done keeps that key in `cloud.json` (by compare and
swap, under pairing's lock) and then in memory. `internal/cloud/keychain.go`;
`VerifyAssertion` returns the key that signed; cloudkit hands out
`SignKeyLink`, `PresentAssertion` and `MaxKeyChain`; `pairstub -rotated`
signs with a second key. Not walked in a browser.

**The door, and `mez-cloud`'s first commit** (ADR-344). `cloudkit/` is
this module's one public package: names for the wire's shapes, the
assertion, the relay and its keys, handed out to `mez-cloud` and to
nobody here. The shapes of `/v1/pair` and `/v1/recover/*` are exported
from `internal/cloud` and used by its own client. `cloudkit_test.go`
plays the cloud against the venue's real client. `mez-cloud`
(`I:\mez-cloud`, two commits, private, not pushed) builds against it
and has its own `CLAUDE.md`, STATE and phase file. Before it, seven of
the answered questions were built (ADR-343) and none seen in a browser.

Before it, **Phase 14, Task 9 - Words** (ADR-340). `deploy.md` has **mez Cloud, or
your own relay** before the two sections it sets up, and says mez Cloud
is not taking venues yet. The landing brief's pricing is two columns
(₹999 and ₹9,999, the founders' price as a badge). `00-product.md`'s
licence section is ADR-312's three layers. After it, on the same day:
`mez-cloud` decided as a private repository (ADR-341), the open
questions answered (ADR-342), and seven of the answers built (ADR-343).

Before it, **Phase 14, Task 7f - A till keeps its pairing through a computer with
no venue** (ADR-339). A token that is no device's, where there is no
venue, is answered 503 `no_venue` and not 401 (`unknownToken`,
`internal/api/devices.go`). The till has a state for it
(`AuthStore.venueMissing` and `recheck`) and a screen, `NoVenueScreen`,
shown by the Gate and the kitchen display over whatever path it is at,
asking again every five seconds. Walked: 7d's procedure with the tablet
left on and signed in, reloaded while waiting; back on its order screen
by itself, a sale rung. `deploy.md` lost its first step.

Before it, **Phase 14, Task 7d - "The PC died"** (ADR-338). `deploy.md`, under
Backups: what is needed, the steps, what the tablets do, what comes back
and what is done again, and what each sentence of a page that did not
get to "is back" means. Done on one machine against `pairstub`: a venue
with a printer, a paired tablet and a sale, killed; a new install at the
same address brought back by number and code; the tablet, off in
between, on Staff Login with the token it held, invoice two rung on it.
Two things the task took for granted were not so: nothing finds a server
for a tablet, and a tablet left on forgot its pairing (cured by 7f). No
code changed.

Before it, **Phase 14, Task 7c2 - A bucket for the stand-in, and the whole walk**
(ADR-337), which closed 7c. `internal/backup/s3fake` is the S3 stand-in
the backup's tests used, moved, with a second key that only reads.
`pairstub -bucket-fake ADDR` serves it, and the stub's account has
whichever venue last paired. `runOptions.source` is gone. **Walked in a
browser**: a venue that backed up to the stub, its server killed, a new
install, **I had mez before** through the server's own start to the last
card, Sign in with the old PIN, a shift opened, a sale rung
(INV/26-27/000001). Nothing in the screen needed changing. Not seen: a
reload during the wait, the last card while "pairing", that card at
400px.

Before it, **Phase 14, Task 7c1 - The screen of "I had mez before"** (ADR-336). 7c
was split in two. `web/shared/screens/setup/recover-logic.ts` has the
checks, the four requests, every sentence, `awaitVenue`, which waits
through the server's start, and `pairThisBrowser`; `RecoverFlow.tsx`
draws it; the wizard's first step has a band, **I had mez before**. Four
steps: Your number, The code, Your venue, Bring it back. Logic tested
(33). **Walked in a browser** at 1366 and 400 wide against `pairstub` as
far as a restore that fails for want of a bucket.

Before it, **Phase 14, Task 7b2b - The routes of "I had mez before"** (ADR-335),
which closed 7b. `internal/api/setup_recover.go`: `POST
/api/v1/setup/recover/start` (`{phone}`), `/verify` (`{attempt, code}`)
and `/restore` (`{venue_id}`, 202 with the claim) behind what first run
is behind (this machine, JSON, `X-Setup-Token`); `/outcome` behind this
machine and `X-Recover-Claim`, answering `{venue_name, cloud, hostname}`
and, when the body names a device, its `pairing_code`. None crosses the
relay. Refusals are `recover_*` codes in an owner's words. Tested as a
page's requests against one server, from the phone number to a paired
browser.

Before it, **Phase 14, Task 7b2a - The server brings a venue back by itself**
(ADR-334). 7b2 was split in two. `runUntilStopped` loops `run`, which
serves under a context that can be asked to start again (`main` and `mez
service run` both use it). `internal/service/cloudrecover`: `Service`
signs the owner in (`Start`, `Verify`) and `Restore` takes the cloud's
grant, makes a claim, stages the venue (`restore.Stage`, with the pairing
code and the claim's hash) and asks for the start; `Arrival`, begun by
the start that adopted a venue, pairs it in the background with the code
that waited and answers `Outcome` and `DeviceCode` to whoever shows the
claim.

Before it, **Phase 14, Task 7b1 - A running server takes a restored venue**
(ADR-333). 7b was split in two. `internal/restore` holds the checks `mez
restore` makes (`EmptyInstall`, `NotRunning`, `MoveAside`, moved from
`cmd/mez`) and two new steps: `Stage` restores the newest backup to
`mez.db.restored` beside the database the server has open, checks it
holds the venue asked for, brings back the photos and leaves
`restore.json` (with the pairing code); `Adopt`, called by `run` once it
holds its port and before the database is opened, moves the empty
database aside and puts the staged one in its place. Never over a venue,
asked at both. Tested by staging under a running server and starting it
again.

Before it, **Phase 14, Task 7a - What a venue that is gone asks mez Cloud**
(ADR-332). Task 7 was split in five. `internal/cloud/recover.go`: a phone
number buys an attempt (`RecoverStart`), the six digits sent to it buy a
`Recovery` (`RecoverVerify`: the owner, the account's venues, a session
held in memory), and `Recovery.Grant` buys, for one venue, a bucket key
that only reads and an ordinary pairing code. Recovery issues no relay
key. What cannot work is not sent, each answer is checked whole, and no
secret prints. `build/dev/pairstub` is one account as well (`-phone`,
`-otp`, `-venue-id`, the `-bucket` flags), walked with curl.

Before it, **Phase 14, Task 6c - mez Cloud's sign-in on a mirror** (ADR-331), which
closed Task 6. `mez mirror -cloud-key <public_key from cloud.json>` lets
the account's owner in on the cloud's word: the mirror's door checks the
assertion, reads who the cloud may be from the copy, remembers the nonce
on disk beside its state file and keeps the session in memory. Nothing is
written to the copy; a PIN's session is never the cloud's. Tested end to
end through `serveMirror` over a real backup. No screen changed and
nothing here presents an assertion from a browser: that is `mez-cloud`'s
(8e).

Before it, **Phase 14, Task 6d2c - The menu on a mirror** (ADR-330), which closed
6d2 and 6d. Each part of the menu editor reads `useOnMirror()`: nothing
adds, renames, moves, retires or imports; an item opens to be read, its
fields disabled and its two tabs live; Export stays, being a read.
**Walked in a browser** on a mirror of the demo menu.

Before it, **Phase 14, Task 6d2b - Staff and Inventory on a mirror** (ADR-330).
Staff has no Add, no row menus, and a role panel whose switches are
disabled with the sentence where Save was. Inventory has no count,
wastage, import or add; a row's menu is History alone and a recipe has no
Edit. Tips and a Z-report needed nothing, a mirror being away. The
dashboard's shortcuts say "Menu" and "Staff" there. **Walked in a browser.**

Before it, **Phase 14, Task 6d2a - Settings, the Floor plan and Devices on a mirror**
(ADR-330). 6d2 was split by screen. `useOnMirror()` is read where a change
is drawn and `OnAMirror` is the sentence in its place. Every settings
section is read-only through `SectionCard`; tax classes, the floor plan
and the device list have nothing left to press but tabs and tables. The
two reads that answered wrongly are not made: Backup and the table codes
each say why in a card. `useAway()` is true on a mirror from its first
answer. **Walked in a browser.**

Before it, **Phase 14, Task 6d1 - The page knows it is on a mirror, and says so**
(ADR-329). 6d was split in two. `web/shared/mirror.ts`: a `MirrorWatch`
reads `X-Mez-Mirror` off every answer, and `useMirror()` is null anywhere
but on a mirror. The bar says "Mirror, as of 23:10" in the venue's zone (a
line under the bar below 768px), and the pill says "Venue offline" when
the copy was over two minutes old as the mirror last answered. A mirror in
view is asked something every 20 seconds, which keeps it following, and
every query is read again when its copy is newer. **Walked in a browser.**

Before it, **Phase 14, Task 6b2 - Following** (ADR-328), which closed 6b. A mirror
in use looks at its backup once a minute (`-follow`), and when the venue
has written since, restores a new copy beside the old, opens it, and
puts it in the old one's place between two requests, with the same
keeper: whoever is signed in stays signed in. The old copy is closed and
removed when its requests are done. A mirror nobody is asking anything
asks its backup nothing. `X-Mez-Mirror` is exact. Tested against a venue
that trades again after the mirror started.

Before it, **Phase 14, Task 6b1 - The mirror's own door** (ADR-327). 6b was split
in two. `internal/mirror`: a `Keeper` remembers the browsers paired to a
mirror and the locks wrong PINs set, in a file beside the copy that a
restart finds, and sessions in memory; `mirror.Auth` is the API's
devices and staff sign-in over one copy. A PIN is judged by the read-only
`auth.StaffAuth.CheckPIN`. Three wrong in a row lock that person on the
mirror for a day. The venue's own devices open nothing on a mirror, and
the copy is not written after it is migrated.

Before it, **Phase 14, Task 6a - A mirror over one restore** (ADR-326). Task 6 was
split in four. `mez mirror -from FOLDER` restores a backup into a
directory of its own, serves the back office over it on
`127.0.0.1:8090`, prints a pairing code, and removes the copy when
stopped. `api.WithMirror`: every request is taken as one from away, every
write but six named is 409 `mirror_read_only` before its handler, and
every answer says when the copy was last written (`X-Mez-Mirror`). The
print queue is never started. The relay's gate now judges a bundle's path
as sent as well as decoded. **Walked in a browser.**

Before it, **Phase 14, Task 5b2 - The People and Devices screens say what these
are** (ADR-325), which closed Task 5. The person mez Cloud signs in as
reads "Signs in from mez Cloud" under the name and is offered no new
PIN; the device reads "Your mez Cloud account's way in. Nobody holds
it", is not renamed, and its Revoke is **Shut mez Cloud's sign-in**.
`staffActions` and `deviceActions` in the two logic files. **Walked in a
browser** at 1366 by 768 on a venue paired against the stand-in.

Before it, **Phase 14, Task 5b1 - The route, and being the cloud's device after
it** (ADR-324). 5b was split in two. `POST /api/v1/sessions/cloud`,
through the relay only, takes `Authorization: Mez-Cloud <assertion>` and
answers a session and its device; the session's token is then presented
as both the device's and the session's. `internal/service/cloudsignin`
holds the order. Only the tunnel says a request was relayed: the server's
own socket takes `X-Mez-Via` off. A pairing made again ends the sessions
the last one opened. `by=cloud` in the access log, `cloud` on a person
and a device in the API, and `build/dev/pairstub` signs an assertion.
**Walked with curl** against a real relay, the stand-in and `bin/mez`.

Before it, **Phase 14, Task 5a - The assertion, the person and the sign-in**
(ADR-323). Task 5 was split in two. `internal/cloud/assertion.go` signs
and checks what mez Cloud vouches with. Migration 00055 marks one staff
row and one device row as the cloud's and keeps spent nonces. Pairing
makes both rows (`auth.PairCloudOwner`), a start makes them for a venue
paired by an older build, and `auth.SignInCloud` spends an assertion
once and opens ADR-314's twelve-hour session. The person has no PIN and
cannot be given one. A session on a revoked device is no session.

Before it, **Phase 14, Task 4c2 - The wizard's mez Cloud step** (ADR-322), which
closes 4c. The wizard has six steps: mez Cloud is the fifth, after the
printer, when the venue exists and the owner is signed in, so it calls
`POST /api/v1/cloud/pair` as the Devices card does and the server gained
nothing. It can be skipped. The words both screens say are in
`web/shared/cloud-pair.ts`. **Walked in a browser** on a venue made in
the wizard, against `build/dev/pairstub` and a local relay: a code that
could not be one refused under the field, the right one paired, a
guest's request through the relay answered, then on to More devices.

Before it, **Phase 14, Task 4c1 - Pairing from the Devices screen** (ADR-321). 4c was
split in two. At the foot of "Back office from anywhere": who the venue
is paired to, and **Connect to mez Cloud** or **Pair again**, opening a
modal for the code (`CloudPairModal.tsx`, `cloud-logic.ts`).
`build/dev/pairstub` is a stand-in for mez Cloud's one route, for walking
it. **Walked in a browser** against the stand-in and a local relay, which
was also the first time pairing ran in one piece: a mistyped code refused
before it was sent, a code the cloud refused, then the right one; the
tunnel connected with no restart, both cards said so, and a guest's
request through the relay was answered by the venue. The bucket in the
walk was one that is not there, so what was seen of the backup is
"could not be started", after eight seconds.

Before it, **Phase 14, Task 4b2 - Pairing while running** (ADR-320), which closes 4b.
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

Before it, **Phase 14, Task 4b1 - A paired venue at start** (ADR-319). 4b was split
in two. The server reads `cloud.json` before anything asks whether there
is a relay, and takes from it the relay's address and key, the back
office across the relay, and `public_url`. A relay in the settings file
wins whole and the pairing is set aside, with a warning in the log. A
pairing that cannot be read stops the start. The backup to the pairing's
bucket goes no faster than 30 s (`ManagerOptions.SyncFloor`); any other
destination is as the setting says. `GET /api/v1/tunnel` has `paired_to`.
Tested by starting the whole server from a pairing against a real relay.
**There is still no way to pair but by writing the file.**

Before it, **Phase 14, Task 4a - The pairing itself** (ADR-318). Task 4 was split in
four. `internal/cloud`: a pairing code cleaned as typed (eight of an
alphabet with no 0, O, 1 or I), one POST to `{cloud_url}/v1/pair` with the
venue's `public_id`, name and version, the answer checked whole, and
`cloud.json` in the data directory to keep it, 0600, never printed with
its secrets. `cloud_url` is the new setting. The request and the answer
are in ADR-318 for `mez-cloud` to build to. **Nothing uses a pairing
yet**, and it is not in `config.toml` as the phase file had it.

Before it, **Phase 14, Task 3b - The venue's half of entitlement** (ADR-317), which
closes Task 3. The tunnel client asks for notices, reads where the venue
stands off the relay's answer to its dial and off `Notice` frames, and
keeps it on `Status.Standing`. Turned away with a good key (403
`not_entitled`, or close 4003) it is `not_entitled` and asks again in
half a minute to a minute. `GET /api/v1/tunnel` has `not_entitled` and
`cloud: {state, paid_until, until, back_office, reason}`. Devices: the
chip **Not carried** and its sentence, a line such as "Cloud: paid until
12 Nov 2026." or "Cloud: payment failed, 5 days left.", and "Back office
from anywhere" saying when the relay does not carry it. **The two cards
were not walked in a browser**: the words are tested, the screen unseen.

Before it, **Phase 14, Task 3a - The relay asks who is entitled** (ADR-316). Task 3
was split: this is the relay's half. `relay.Entitlements` (`Until`,
`Remote`, `State`, `PaidUntil`, `Reason`) is asked at the dial and again
each hour and at the date. Not carried is 403 `not_entitled` at the dial
and close code 4003 after, each with the reason. `StaticEntitlements`
reads `RELAY_ENTITLEMENTS`, a file of `venue until [remote|guests]` with
`*` for the rest, and reads it again when it changes. The venue is told
where it stands in the `X-Mez-Entitlement` header and, if it asked, in a
`Notice` frame. With no file every key is carried, as before. Nothing at
the venue reads any of it yet.

Before it, **Phase 14, Task 2 - The backup is paced on purpose** (ADR-315).
`backup_sync_interval` (`-backup-sync-interval`,
`MEZ_BACKUP_SYNC_INTERVAL`): `30s`, from `5s` to `10m`. It sets how often
Litestream looks at the database as well as how often it sends, since it
sends one file per look. Counting against the stand-in bucket showed
listings were most of the bill, so the merges, Litestream's tidying and
mez's own look at a bucket are paced too, all in
`internal/backup/pace.go`. `status.sync_interval_seconds` is on `GET
/api/v1/settings/backup`; no screen shows it. `TestAnHourAsksLittleOfABucket`
holds 200 billed requests in an hour of selling and 25 in an hour of
none (measured 183 and 16: ₹32 a month on R2, where ADR-313 assumed ₹24).
`deploy.md`, `running.md` and the cost paper say so.

Before it, **Phase 13, Task 5 - A remote session is harder to take than a LAN one**
(ADR-314), built on 2026-10-12 at the human's word, after Phase 14's Task
1. Where a sign-in came from is recorded (`via` on `sign_in_attempts` and
`staff_sessions`, migration 00054). `auth.SignInRemote`: wrong PINs from
away are counted apart from the venue's, and the third in a row
hard-locks the person with ADR-260's lock, at the till too, until someone
at the venue or `mez unlock` clears it. A session opened through the
relay lasts `auth.RemoteSessionTTL` (12 h) and a relayed request is
served on no other kind: one opened at the venue answers
`sign_in_required` from away. Tests in `internal/auth/remote_test.go` and
`internal/api/remote_test.go`; `deploy.md` has the owner's paragraph. No
screen changed and none was walked.

Before it, **Phase 14, Task 1 - ADR-313: the entitlement states and the Razorpay
events.** A document, no code. Three dates on the cloud's venue row,
written by one function: `paid_until` (the owner reads), `entitled_until`
= that + 7 days of grace (the relay reads), `backups_kept_until` = that +
30 days (mirror, download, restore). Six states, three made by the clock.
The table of six Razorpay events. Prices ₹999 and ₹9,999, founders ₹699
and ₹6,999. A venue under ₹100 a month. Task 3 in the phase file gained
`State`, `PaidUntil` and a once-a-minute retry after `not_entitled`.

Before it, **Phase 13, Task 4 - The owner turns it on and sees where to go**
(ADR-310). `GET /api/v1/tunnel` has `back_office_url`. The Devices screen
has a "Back office from anywhere" card (`RemoteCard.tsx`,
`remote-logic.ts`): how it stands, the address with Copy, and what to do.
It is not a switch. `deploy.md` has the owner's section, and
`build/docker-compose.yml` the commented setting. Walked in a browser
through a local relay with a hostname per venue. After it, at the human's
word on seeing it on a phone: under 768px the sections are a drawer behind
a menu button, and no page scrolls sideways (ADR-311).

Before it, **Phase 10, Task 12a and 12b** (ADR-304). `mez reset-pin`: with no
`-staff` it lists who works at the venue; with one it draws a new
six-digit PIN, shown once, clears the person's lock and signs them out.
For a venue whose only owner forgot the PIN, which had no way back in.
And `build/dev.sh` prints the owner's PIN only on the run that sets the
venue up.

Before them, **Phase 10, Task 11b: the choices dialog** (ADR-303):
Import choices in the Menu screen's Spreadsheet pill, and the items
preview showing what each new item will offer.

Before it, **Phase 10, Task 11a: groups of choices from a file, read and imported**
(ADR-302). `menu.ReadChoices` for a file of Group, Fewest, Most, Choice,
Price, Picked to start, checked whole by `CleanGroup`; routes under
`/api/v1/menu/choices/import`. The items file and the export gained a
**Choices** column naming the groups an item offers, read for a new
item.

Before it, **Phase 10, Task 10d: the recipes dialog** (ADR-301): Import
on Inventory's Recipes view, from a template that lists every dish with
no recipe.

Before it, **Phase 10, Task 10a to 10c: ingredients and recipes from a file**
(ADR-298 to ADR-300). Reading a file moved to `internal/domain/sheet`,
shared with the menu. `stock.ReadItems` (Name, Counted in, Low at, Cost,
Count at close; no stock from a file; a kilogram is a problem) and
`stock.ReadRecipes` (Dish, Ingredient, Amount; only for a dish with no
recipe; dishes only), each with template, preview and import routes
under `/api/v1/stock/import` and `/api/v1/stock/recipes/import`. Import
on Inventory's Stock levels opens the ingredients dialog; the file
dialogs' shared parts are in `web/backoffice/src/files/`.

Before them, **Phase 10, Task 9c: prices from a file, on the screen**
(ADR-297): one Spreadsheet pill in the Menu screen's head, and Change
prices behind it with a table of Now, New and Change, largest first.

Before it, **Phase 10, Task 9a and 9b: prices from a file, read and applied**
(ADR-295, ADR-296). `menu.ReadPrices` finds a row's item by category
and name and reads only its price; a row that is no item is left out
and listed. `POST /api/v1/menu/prices/preview` and `POST
/api/v1/menu/prices?version=N`: applied in one `db.EditMenu`, and
refused 409 if the menu has moved since the preview. A test shows an
open order keeps the price it was rung up at.

Before them, **Phase 10, Task 8b: the export on the screen** (ADR-294):
Export in the Menu screen's head, always the whole menu. It moved into
the Spreadsheet pill in Task 9c.

Before it, **Phase 10, Task 8a: the menu export on the server** (ADR-293).
`GET /api/v1/menu/export`: the whole menu as `mez-menu.csv`, in the
import's columns plus Status. `ReadImport` takes it back: it accepts
Status unread, and skips an item already there in a retired category.

Before it, **Phase 10, Task 7d: the import's screen** (ADR-291). **Import** in the
Menu screen's head opens a modal: template, choose the file, what it
would add or what is wrong by row and column, **Add N items**.
`ImportModal.tsx`, `import-logic.ts`. Walked in a browser and by the
human on the dev venue. After it, Tasks 8 to 12 were added (ADR-292),
and `v0.1.0-alpha.2` was tagged on the human's machine only.

Before it, **Phase 10, Task 7c: the template** (ADR-290).
`GET /api/v1/menu/import/template`: the eleven columns and two rows
named "Example: ..." that use the venue's own first active category, tax
class, station and course. A `CsvResponse`, through `writeCSV`.

Before it, **Phase 10, Task 7b: the import on the server** (ADR-289).
`POST /api/v1/menu/import/preview` answers what a CSV file would add;
`POST /api/v1/menu/import` writes it in one `db.EditMenu`, reading the
file again inside the write. A problem is a 422 `menu_import_refused`.
The same file twice adds nothing, so no idempotency key is stored.

Before it, **Phase 10, Task 7a: read and check the file** (ADR-288).
`internal/domain/menu/import.go`: `ReadImport(data, ImportVenue)` answers
an `ImportPlan` and writes nothing. CSV as a spreadsheet saves it; prices
through `money.Parse`; no guessing at a tax class, station or course;
adding only; one bad row refuses the file.

Before it, **the audit between phases** (ADR-287, 2026-10-07). Four reviews at once
(security, Go, TypeScript, docs against code) and a walk of the whole
spine against the demo venue from `bin/mez`. No critical or high finding;
what was fixed and what was left are under the ADR and in `KNOWN.md`.
After it, outside any task: the back office no longer scrolls as a page
(a screen-reader label below the fold escaped the screen's scroll), and a
write-off shows on "Wastage this week" at once.

Before it, **Phase 12, Task 7e: CONTRIBUTING, with the architecture** (ADR-285).
`CONTRIBUTING.md` gains "How it fits together": the layers, an order as
events, one tap followed from the till to the printer. Its versions,
targets and tree brought up to date; `01-architecture.md` corrected where
it had stopped being true.

Before it, **Phase 12, Task 7d: the relay guide** (ADR-284). The venue's side in
`deploy.md`, "Guests ordering from their phones": three settings
(`public_url` with the two), where they go, how to check.
`build/relay/README.md` for the operator: the key, the venue id, every
setting, what to back up. The compose file's example address corrected.

Before it, **Phase 12, Task 7c: troubleshooting** (ADR-283).
`docs/troubleshooting.md`: the printer not found or not printing, a tablet
that cannot reach the server, Windows's warning, guests who cannot order
from their phones, the drawer that will not open. Each from the screen's
own words, then what to check in order. Documentation only.

Before it, **Phase 12, Task 7b: hardware, printers and drawers** (ADR-282).
`docs/hardware.md`, from the print code: the commands a printer must take,
network and Windows USB, 80mm and 58mm, what prints where, ASCII only on
paper, the drawer's fixed pulse, the queue's retries. It says no printer
has been printed on, and names no model. Documentation only.

Before it, **Phase 12, Task 7a: the install guides, checked** (ADR-281). Task 7 split
in six. `deploy.md` and `running.md` read against the Makefile, the
installer, the unit, the compose file, the release script and the binary's
`-h`, and corrected: the relay, checksums, arm64, `-data-dir` under
systemd, the release's image, the PIN hard lock, upgrading by platform, a
table of every command. Documentation only.

Before it, **Phase 12, Task 6: release automation** (ADR-280).
`.github/workflows/release.yml`: a tag `v*` runs `make check`, `make dist`
(now with `linux/arm64`), the installer on a Windows runner, the image for
amd64 and arm64 to `ghcr.io`, and a draft release with checksums and notes
(`build/release.sh`). `docs/releasing.md` is the procedure. Every piece run
by hand; the workflow itself has never run.

Before it, **Phase 12, Task 5: Docker and Linux** (ADR-279). `build/mez.service`;
the image gains `/backup`; the compose file a backup's mount and a relay.
Both run in containers. Litestream is started when its destination
answers: started regardless it dropped the server's locks on Linux.

Before it, **Phase 12, Task 4f: restore, done and written down** (ADR-278).
`mez restore` brings a venue back from a folder or a bucket into a data
directory with no venue in it; `-list` shows the generations there. Old
generations tidy themselves a week after they were replaced. Done for real
on Windows: three sales, the server killed, restored, invoice four next.

Before it, **Phase 12, Task 4e: the Backups section of settings** (ADR-277).
Settings, Backup: a card for how the backup is doing and one for where it
goes. Walked in a browser at 1366 and 1024. "Back up now" looks at the
destination before it answers.

Before it, **Phase 12, Task 4d: the backup's write path and API** (ADR-276).
`backup.Manager` saves a destination after trying it and swaps the running
backup, at once on Windows and at the next start elsewhere. `GET`/`PUT
/api/v1/settings/backup`, `POST .../now`. The database's copies moved under
`db/<generation>/` and the database leaves its log alone while a backup
runs: both fixed backups that restored short. Run on Linux in a container.

Before it, **Phase 12, Task 4c: the photos folder** (ADR-275). `<data dir>/photos/`
copied to `photos/` beside `db/` at the destination: only what is missing,
on the probe's clock, on "back up now" and as the server stops.
`backup.RestorePhotos` brings them back, each held to its hash.

Before it, **Phase 12, Task 4b: S3 and R2** (ADR-274). `backup.KindS3` in
`backup.json`: bucket, path, region or endpoint, key and secret, through
Litestream's S3 client. No internet stops neither start, sales nor stop.
Tested against a stand-in S3 and a real MinIO; not Amazon's S3 or R2.

Before it, **Phase 12, Task 4a: Litestream in the binary, to a folder** (ADR-273).
`internal/backup` runs Litestream v0.5.15 as a library over `mez.db`,
started after the migrations and closed after the database; where to is
`<data dir>/backup.json`. `BackUpNow`, `Status` and `Restore` in Go only.
Tested on Windows against real files, the whole server included.

Before it, **Phase 12, Task 3f: a QR code for the address, and addresses by name**
(ADR-271). `GET /api/v1/devices/addresses` answers each address with its
adapter's name, whether it looks virtual, and a QR code for each kind of
device; the wizard's last step draws it. The printer search starts from
real adapters. Walked in a browser on a PC with one real adapter.

Before it, **Phase 12, Task 3e: the setup token** (ADR-270). `firstrun.Token` in
`<data dir>/setup-token`, asked for by `POST /api/v1/setup`; `mez
setup-link`; the installer's "Open mez now" opens the link; the wizard
takes it from the address's fragment. The installer was compiled, not run.

Before it, **Phase 12, Task 3d: the wizard's last screen, and the hand-off** (ADR-269).
A fifth step, More devices: the server's LAN address and a pairing code in
large type (`GET /api/v1/devices/addresses`). Pair and the docs stop naming
`mez setup` first. Walked in a browser: the code paired a second origin.

Before it, **Phase 12, Task 3c: printer discovery and a test print** (ADR-268).
`internal/device/discover` searches the server's private networks for port
9100 and lists Windows' printers; `POST /api/v1/printers/discover`; the
wizard's fourth step. Walked against a stand-in printer on loopback.

Before it, **Phase 12, Task 3b: the wizard's questions** (ADR-267, `setup-wizard.md`).
`web/shared/screens/setup/`, at the terminal's `/setup`, outside the guard.
Three steps, then Create in four resumable stages. Pair sends a new machine
there. Walked in a browser from an empty data directory.

Before it, **Phase 12, Task 3a: first run over HTTP** (ADR-266). `internal/service/
firstrun` is the one place a venue is made; `GET` and `POST /api/v1/setup`,
the post only for a browser on the server's own machine. Seen with curl.

Before it, **Phase 12, Task 2: the installer** (ADR-265). `build/installer/mez.iss`, by
`make dist && make installer` (Inno Setup 6). It calls `mez service` for
everything about the service, opens the firewall for the program, makes the
kiosk shortcut, and keeps the data on uninstall. Installed, upgraded and
removed silently on the developer's PC. Not signed.

Before it, **Phase 12, Task 1b: the service's own account** (ADR-264). The service runs
as `NT SERVICE\mez`, which `fsperm.Restrict` always names; install hands it the
data directory; restarts wait 5 seconds, 30, then 5 minutes.

Before it, **Phase 12, Task 1: the Windows service** (ADR-263). `mez service
install|uninstall|start|stop|status` over `internal/winsvc`; restart after it
dies; warnings and errors to the event log. Not run under the Service Control
Manager.

Before it, **Phase 11, Task 5f4: orders to collect at the counter** (ADR-253). The Orders
screen has a To collect tab, the pickup code on the row and in the panel, and
Handed over, which closes the ticket. Task 5 is done. Not seen in a browser.

Before it, **Phase 11, Task 5f3b: the pickup code in the back office** (ADR-252). In the Floor
plan's codes card: make, show, renew, turn off; a warning where no gateway is
set; and the door's card first on the sheet. Not seen in a browser.

Before it, **Phase 11, Task 5f3a: ordering to collect on the phone** (ADR-251). 5f3 split in
two. `/order/p/<code>` is the table's screens with a `collect` flag, and the
pickup code in large type on the order. `guest-collect.md`. Seen with a mocked API.

Before it, **Phase 11, Task 5f2: ordering to collect over the guest API** (ADR-250). Six
routes under `/order/api/p/{code}`, in `internal/api/guest_pickup.go`. Limits by
address and then by venue. With no gateway the venue answers `ordering: false`.

Before it, **Phase 11, Task 5f1b: paying for an order to collect, and sending it** (ADR-249).
A charge may have no table (migration 00050). Paid, the order is a ticket of its
own on the `web` channel, sent, with a pickup code no order still out shares.

Before it, **Phase 11, Task 5f1a: an order to collect, held unpaid** (ADR-248). A round may
have no table (migration 00049); `PlacePickup` prices and holds one where a
gateway is set. Nothing pays for one or sends one yet.

Before it, **Phase 11, Task 6e: the venue's pickup code** (ADR-247). `pickup_codes`
(migration 00048), in `tablecodes` and over the staff API. No screen. Task 6 done.

Before it, **Phase 11, Task 6d: limits on what a table's code may ask** (ADR-246). By table:
reads, changes and orders, each counted apart. By address: wrong codes, and a
ceiling a full room does not reach. Tested with the limits lowered.

Before it, **Phase 11, Task 6c: the sheet of codes to print** (ADR-245). Print the codes on
the Floor plan's card: a card for each table with a code, six to a page of A4.
`GET /api/v1/table-codes/sheet`. Seen under its print styles; nothing printed.

Before it, **Phase 11, Task 6b: the codes in the back office** (ADR-244). "Ordering from the
table" on the Floor plan: each table's code and address, the QR, Renew and Turn
off behind a confirm, Make the missing codes. Seen at 1366 and 1920.

Before it, **Phase 11, Task 6a: table codes over the staff API** (ADR-243).
`/api/v1/table-codes`, renew and revoke a table's code, and its address as a QR,
for whoever may edit settings. `tablecodes.Revoke`. No screen.

Before it, **Phase 11, Task 5e2: ordering the same again** (ADR-242). "Order this again" on
a sent order adds its dishes to the cart, held against today's menu, and opens
the cart. It places nothing. Task 5 is done but for 5f, pickup.

Before it, **Phase 11, Task 5e1: the receipt on the phone** (ADR-241). `#receipt`: the
invoice as issued, offered on the bill once it is paid or gone. The phone keeps
which bill was its table's (`mez.bill.<code>`). Seen at 390 wide.

Before it, **Phase 11, Task 5d: the e-receipt on the server** (ADR-240).
`GET /order/api/t/{code}/receipts/{bill}`: the counter's own invoice, by the
table's code and the bill's id. The open bill and a tracked order say `bill`.

Before it, **Phase 11, Task 5c: the order's screen follows the kitchen** (ADR-239). With the
kitchen, being made, ready, as the kitchen does each, on the screen and the
strip, asked at the order's own address. Seen at 390 wide in the real binary.

Before it, **Phase 11, Task 5b: tracking over the guest API** (ADR-238).
`GET /order/api/t/{code}/orders/{round}` answers a round as it stands, with
`progress` once it is sent. Another table's code finds nothing. No screen.

Before it, **Phase 11, Task 5a: a round is tracked through the kitchen, in the services**
(ADR-236). A sent round knows its lines (`first_seq`, migration 00047);
`order.State.Progress` and `TrackRound` read how far they have come. ADR-237, by
the owner: ordering for pickup is 5f, paid first, one code for the venue.

Before it, **Phase 11, Task 4g3: splitting the bill on the phone** (ADR-235). In the bill's
Pay online card: the rest, evenly among so many, or a figure typed; and "I am not
paying this" on a waiting payment. Seen at 390 wide. 4g is done.

Before it, **Phase 11, Task 4g2b: giving up a payment page** (ADR-234). `charges.LetGo` and
`POST .../bill/charge/{id}/let-go`: the charge stays open and payable, and is no
claim on the bill. A charge's answer says `let_go`. Migration 00046.

Before it, **Phase 11, Task 4g2: a share over the guest API** (ADR-233). The bill answers
`split` (the rest, what this phone can take, equal parts); a body on
`POST .../bill/charge` is a share, by amount or among N people. No screen.

Before it, **Phase 11, Task 4g1: a share of the bill, in the services** (ADR-232).
`charges.StartShare` is a charge for part of what a table owes, for one phone. An
open page is a claim; nothing is given past what is unclaimed. Migration 00045.

Before it, **Phase 11, Task 4f6c: the back office says how paying online stands** (ADR-231).
The "Paid from a phone" card names the gateway, when a payment last landed through
it, whether it calls back, and where guests return to. Read only; never a key.

Before it, **Phase 11, Task 4f6a: Stripe** (ADR-230). `internal/gateway/stripe`: a charge is a
Checkout Session, with its webhook. `gateway = stripe` needs `gateway_key_secret`
and `public_url`. Cashfree was left out: it needs each payer's phone number.

Before it, **Phase 11, Task 4e5d: the owner chooses when guests pay** (ADR-229). On the back
office's "Paid from a phone" card: when they are ready to leave, or before the
order is sent. With no gateway, paying first says it does nothing yet.

Before it, **Phase 11, Task 4e5c: the phone pays for an order** (ADR-228). An unpaid order's
screen shows what to pay and Pay; back from the gateway it asks how the charge
stands and then reads "Paid, and with the kitchen".

Before it, **Phase 11, Task 4e5b2: a guest pays for a round** (ADR-227).
`POST /order/api/t/{code}/orders/{round}/charge`; its status is the bill charge's
route, which says `round`. `CanCharge` is wired to the gateway being set.

Before it, **Phase 11, Task 4e5b1: a round's charge** (ADR-226). `charges.StartRound` asks
the gateway for a held round's `Due`. Paid, `Settle` sends the round
(`order.SendPaidRound`) and then tenders what was charged. Migration 00044.

Before it, **Phase 11, Task 4e5a: a round held until it is paid** (ADR-225). `payment.mode`
is `pay_at_table` or `prepay`. Where guests pay first and can, `PlaceRound` keeps
the round `unpaid` with `Due`, what it adds to the bill by `order.Adding`.

Before it, **Phase 11, Task 4f5: the gateway's webhook** (ADR-224).
`POST /order/api/hook/{gateway}` takes a gateway's signed call as a hint to
`Settle` now; what it says is never read. `gateway_webhook_secret` unset, it is 404.

Before it, **Phase 11, Task 4f4: unapplied charges on the terminal** (ADR-223). Money a
gateway took that no bill could is first in the terminal's sheet, in danger, and
turns the pill red, until a person says they refunded or recorded it
(`POST /api/v1/charges/{id}/resolve`). It moves no money.

Before it, **Phase 11, Task 4f3: paying online from the phone** (ADR-222).
`POST /order/api/t/{code}/bill/charge` and `GET .../bill/charge/{id}`; the bill
says `online`. The return address is `public_url`, a new setting, never a
request's. The phone has Pay online (`charge-logic.ts`, `use-charge.ts`).

Before it, **Phase 11, Task 4f2: Razorpay, its keys, and the sweeper** (ADR-221).
`internal/gateway/razorpay` uses payment links. `gateway`, `gateway_key_id` and
`gateway_key_secret` are configuration (`MEZ_GATEWAY_KEY_SECRET`), not settings.
`charges.Sweeper` settles open charges every 20 seconds.

Before it, **Phase 11, Task 4f1: the gateway interface, and a charge becoming one payment**
(ADR-220). `internal/gateway` is two questions, Start and Status.
`internal/service/charges` starts a charge for what a table owes and, when the
gateway says paid, tenders it once (`charge:<id>` as the key); money a bill cannot
take is kept as `unapplied`. Tested against a fake.

Before it, **Phase 11, Task 4e4b: "I have paid" on the screens** (ADR-219). The phone's bill
has the button, and says staff were told from the bill's `paid_told`. The
terminal's pill, sheet ("From the tables") and chime carry the notice, with Take
payment opening `/pay/{order}` and No payment found putting it away.

Before it, **Phase 11, Task 4e4a: "I have paid", on the server** (ADR-218).
`POST /order/api/t/{code}/bill/paid` keeps a notice (`paid_notices`, migration
00040): a message to staff, never a payment. The bill says `paid_told`. Notices
ride on `GET /api/v1/guest-requests` as `paid`; staff put one away with
`POST /api/v1/paid-notices/{id}/seen`, and a recorded payment puts it away itself.

Before it, **Phase 11, Task 4e3: the QR on the phone's bill** (ADR-217).
`GET /order/api/t/{code}/bill/upi.svg` draws the bill's UPI link as a QR code, by
an encoder of our own (`internal/domain/qr`, no dependency), checked module for
module against python-qrcode and read back by a decoder. The phone shows it under
"Pay from another phone". No phone has scanned one.

Before it, **Phase 11, Task 4e2: the UPI address in the back office** (ADR-216). A card,
"Paid from a phone", under the venue's details on Settings: the address, the name
a guest sees, a standing warning that mez cannot check the address, and what a
guest's bill now offers. `web/backoffice/src/settings/PaymentsCard.tsx`.

Before it, **Phase 11, Task 4e1: a UPI pay link on the phone's bill** (ADR-215). The venue's
UPI address is a setting (`GET`/`PUT /api/v1/settings/payments`). The server builds
the `upi://pay` link for the balance (`payment.UPIAccount.Link`) and the bill
carries it; the phone shows "Pay by UPI". Staff confirm the payment on the
terminal, as at the till. 4e split in five.

Before it, **Phase 11, Task 4d: the bill on the phone** (ADR-214).
`GET /order/api/t/{code}/bill` is the table's open ticket with the till's parts
taken out; `#bill` shows its lines, tax, total, paid and left to pay.
`web/guest/src/Bill.tsx`, `bill-logic.ts`; spec `guest-bill.md`. The prepay setting
moved to 4e.

Before it, **Phase 11, Task 4c: Place Order on the phone** (ADR-213). The cart sends itself
with one key for one cart; the answer is kept by table code and shown at `#order`
as waiting, with the kitchen, or not accepted; a waiting order asks again every 8
seconds. `web/guest/src/order-logic.ts`, `use-order.ts`, `Order.tsx`; spec
`guest-order.md`.

Before it, **Phase 11, Task 4b2: the request on the terminal** (ADR-212). A pill in the top
bar on every screen while a request waits, a sheet with Accept and send and
Decline, the kitchen's chime on arrival and every 30 seconds, and a mark on the
table. `web/terminal/src/requests/`, spec `terminal-requests.md`. Task 4b done.

Before it, **Phase 11, Task 4b1: answering a guest's request, on the server** (ADR-211).
`GET /api/v1/guest-requests`, and `POST .../{id}/accept` and `.../decline`, asking
`order.open`. Accept finds or opens the table's ticket and appends the lines as
stored. A request arriving or answered moves the floor version, so terminals hear
`floor_changed`. A request lasts as long as its shift.

Before it, **Phase 11, Task 4a: a guest's cart becomes an order, on the server** (ADR-210).
`POST /order/api/t/{code}/orders` with ids, counts and a required `Idempotency-Key`;
`menu.Menu.Ring` prices each line. A "round" (`guest_rounds`, migration 00039,
`svcorder.PlaceRound`) joins the table's open ticket and is sent, or with none open
is kept as a request: no order and no event. Task 4 split in seven.

Before it, **Phase 11, Task 3c: a dish with its choices, and the cart** (ADR-208). The sheet
uses the till's own rules (`web/shared/modifier-selection.ts`, moved). The cart is ids
and counts in `localStorage` by table code, priced from the menu each time. Nothing is
sent. Then photo frames once any dish has a photo, and the decision that a phone order
is a request staff accept (ADR-209). 36.1 KB of 60.

Before it, **Phase 11, Task 3b: the guest's landing and menu** (ADR-207), `web/guest/src`: the
table and venue, then every dish with a search, categories and diet filters, and a
screen for a code that leads nowhere or a venue that cannot be reached. No router:
the code is read from the end of the path and the screen is the hash. Specs:
`guest-landing.md`, `guest-menu.md`.

Before it, **Phase 11, Task 3a: the guest's way in** (ADR-206). Each table has a code
(`internal/service/tablecodes`, migration 00038, `mez table-codes`). The guest API is
under `/order/api/` (`t/{code}`, `t/{code}/menu`, `photos/{name}`), with no device or
session, 300 requests a minute an address. The tunnel leaves `/order/...` paths alone,
so the bundle's files load through the relay; proved at `democafe.jskin.in`.

Before it, **Phase 11, Task 2b: the tunnel in the back office** (ADR-205). `GET /api/v1/tunnel`
and a card on Devices, "Ordering from phones": a chip, a sentence saying what guests
can do, when it tries next and why it failed. Health reads the same status
(`api.WithTunnel`). Seen in the browser reconnecting and connected, at 1366 and 400
wide. Task 2 done.

Before it, **Phase 11, Task 1c: the relay is live** (ADR-204) at `https://relay.jskin.in`, venues
at `<venue>.jskin.in`: the owner's Hetzner server (wiped, renamed `mez`) behind
Cloudflare, Caddy for TLS with one wildcard certificate, the files in `build/relay/`.
Proved with `bin/mez` on the owner's PC and a guest page fetched over the internet.
`jskin.in` is for testing; the owner will buy `mez.ac` for production.

Before it, **Phase 11, Task 1b: a hostname per venue** (ADR-203). `RELAY_VENUE_DOMAIN` serves
each venue at `<venue>.<domain>`, the whole path its own, and stops serving
`/v/<venue>/`. Venues dial `relay.<domain>`. Reserved names are never venues. Unset,
the relay routes by path as before.

Before it, **Phase 11, Task 2a: the tunnel client** (ADR-202). `internal/tunnel` dials the relay
with the new `relay_key`, serves guests through the server's own handler, reconnects
with backoff (1 s to 60 s, 5 minutes after a refused key or a replaced socket) and a
heartbeat, and is `checks.tunnel` in health. Every guest path lands under `/order/`,
out of reach of the staff API. `TestTheCounterSellsWhileTheRelayIsDown` kills a real
relay mid-service; also done by hand with both binaries. Task 2 was split: 2b is the
back office view.

Before it, **Phase 11, Task 1: the relay service** (ADR-201). `cmd/relay` over `internal/relay`:
a venue dials `GET /agent` with a signed key (`relay issue-key`, an HMAC under
`RELAY_SECRET`, nothing stored), guests reach it at `/v/<venue>/...`, and requests
ride one socket as frames (`internal/relay/wire`). Per-venue token bucket, 256
streams, 1 MiB bodies, a slow guest cut before the socket goes unread, JSON logs in
UTC, `/healthz`, `build/relay.Dockerfile`, and `make build` makes `bin/relay`. Venues
share one origin: cookies and redirects are kept to the prefix, but that is not
isolation (open question 7). Tests over real sockets, 92% of the package.

Before it, **Rebuild, Task 17h3: STATE.md after the walk** (ADR-200). Task 17 is done: 28 screen
specs written from the code (`docs/design/screens/README.md`), the old words off the
screens, one door for closing the day, and a walk as each role (ADR-196 to ADR-199).
`STATE.md` is short again: each task's history moved here and the known-broken list to
`docs/KNOWN.md`, whole. There, "not seen by eye" lost only what the walk saw, in
fifteen places, and three statements no longer true. The owner started phase 11
before the walk; the rebuild and phases 09 and 10 still owe their human checks.

Before it, **Rebuild, Task 17h2: the walk** (ADR-199). On a fresh demo venue run from the current
build (the embedded bundles), at 1920x1080 and for the waiter 1366x1024: Meera opened
the shift and took a counter order through cash, a tip and its invoice; the wall
display started and readied its tickets; Kabir seated T3, ordered over two courses,
sent, fired, split the bill by UPI and cash, closed and cleared the table; Arjun
refunded an item, counted the drawer blind, explained ₹5.00 short, closed, and read
Z-report 1; Ravi landed on `/pass`; the back office showed the day on the dashboard,
the Z-report with its exports, the reports and Audit. Every figure agreed, the drawer
to the rupee. Two things it found are fixed: a refund now says who authorised it (Go
test), and the Z-report's Tax card says it is after credit notes (`taxNote`, vitest).
17h1's fixes were seen: no Close the day on the Orders sheet, the new words. `make
check`, vitest (1426) and `make build-web` pass.

Before it, **Rebuild, Task 17h1: the fixes before the walk** (ADR-197, ADR-198). Task 17h is split
in three. What writing the specs found is fixed: the Orders sheet's Close the day is gone,
so closing has one door, under the profile, for `shift.manage`; about thirty messages
still in the old words (till, ticket where an order is meant, "Refunds come with Task
6.") now say order, order screen and terminal, five of them the server's; "ticket"
stays for the kitchen's. `print/PrintStatus.tsx` and `lastBumped` are deleted. The
specs that quote those words follow. Tests: seven assertions moved to the new words.
`make check`, vitest (1425) and `make build-web` pass. Not tried in a browser.

Before it, **Rebuild, Task 17g2: the specs for Settings, Printers, Devices, Floor plan, Tips and
the Z-report.** Task 17g is done, and with it every spec: 28 files in
`docs/design/screens/`, each written from the code as it stands (ADR-196).
`backoffice-settings.md` (D22's frame and its three sections), `backoffice-printers.md`
and `backoffice-devices.md` (Task 16k's tables), `backoffice-floor.md` and `tips.md`
(their layouts were left as they were, ADR-195), and `zreport.md` (D23's report and
slip in both apps, the list and its exports). `screens/README.md` lists every spec
against its screen, path and design screen, and the design screens that have none.
Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17g1: the specs for Menu, Inventory and Staff.** Task 17g is split in
two (`rebuild.md`). Written from the code as it stands (ADR-196): `backoffice-menu.md`
(D18: the categories, the list and the inline editor, with the Choices and Stations and
courses views), `backoffice-stock.md` (Inventory, D19: the levels table, the forms, the
week's wastage, and Recipes) and `backoffice-people.md` (Staff and roles, D20: the
table, the role's switches, and what you may give). Each Reference was checked against
the design file's own words. Two paths in 17f's specs were put right (the stock read is
`GET /api/v1/stock`). Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17f: the specs for the back office's frame, the dashboard, the
reports and Audit.** Written from the code as it stands (ADR-196): `backoffice-shell.md`
(the bar, the grouped sidebar, who sees what, the `g` chords, the guard),
`backoffice-dashboard.md` (new, D12), `backoffice-reports.md` (the shared range and
Export, then Sales, D15, Items and inventory, D16, and Staff, D17, each with what sits
under "In detail") and `backoffice-audit.md`. Each Reference was checked against the
design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17e: the spec for the kitchen.** `kitchen-display.md`: the one
board and its two ways in (the wall display at `/kitchen`, a cook on a terminal at
`/pass`), the three columns and the ticket card, the all-day strip, today's stats,
Coming next, the foot's switches, espresso night, and who may move a ticket (D10, D11).
Its commit left this file and the phase file's tick behind; both were put right with
17f.

Before it, **Rebuild, Task 17d: the specs for the waiter's screens.** Five new files, written
from the code as it stands (ADR-196): `waiter-home.md` (T01, with seating and clearing
from a card), `waiter-order.md` (T02, Confirm and send, T04, and Course View; only what
differs from `terminal-register.md`), `waiter-table.md` (T03, with Transfer and merge,
T06), `waiter-bill.md` (T05) and `waiter-shift.md` (T08). Each Reference was checked
against the design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17c: the specs for payment, Orders and the print queue.** Written
from the code as it stands (ADR-196): `terminal-tender.md` (Payment, D07; Split Bill,
D08a; the waiting, not recorded and received cards, D08b; the invoice and the paid
panel), `terminal-past-bills.md` (Orders and the refund modal, D09, with void, reprint
and bills from before today) and `terminal-print-queue.md`. Each Reference was checked
against the design file's own words. Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 17b: the specs for the order screen and Tables.** Written from the
code as it stands (ADR-196): `terminal-register.md` (the order screen, D03, its sheets,
and the item modal, D04) and `terminal-floor.md` (Tables, D05: the plan, the list, the
panel, and Move, Join and Part). Each lists what the design draws and the screen leaves
out, checked against the design file's own words. Docs only: no code changed. `make
check` passes.

Before it, **Rebuild, Task 17a: the specs for the way in and the frame** (ADR-196). Task 17 is
split in eight (17a to 17h, `rebuild.md`): seven of specs, then the walk. Written from
the code as it stands: `terminal-lock.md` (Staff Login and pairing, D01),
`terminal-shell.md` (new: where each role lands, the desktop's and the waiter's frame,
the paths, the shift gate), `terminal-open-shift.md` (new, D02) and `terminal-close.md`
(Close the day in two steps, D23). Docs only: no code changed. `make check` passes.

Before it, **Rebuild, Task 16k: Printers, Devices and Audit on Task 16's patterns** (ADR-195).
Asked for by the owner before Task 17. Printers (`printers/PrinterTable.tsx`,
`PrinterRouting.tsx`) and Devices (`devices/DeviceTable.tsx`) are tables in a card
with a chip and a row menu; Audit has the reports' head and the range pill, now
`sales/RangePill.tsx`. Layout only: no server change. Seen in a browser as Priya at
1920x1080, and Printers at 1100. Tests: `printsText`, `printersSummary`, `stateOf`,
`listed`, `devicesSummary`, the `device` chips. `make check` and `make build-web`
pass.

Before it, **Rebuild, Task 16j: the Z-report and the close** (ADR-194). Task 16 is done. The
Z-report (`shared/zreport/ZReportView.tsx`, `ZPaper.tsx`, both apps) has D23's slip
beside the report in full, with "By how it was served", tips by who took the payment
and covers where a report has them. Close the day (`terminal/src/shift/
CloseDayScreen.tsx`) is two steps: count the drawer, then the Cash reconciliation card
and Close the day. Seen in a browser as Priya at 1920x1080: the count, the
reconciliation over its limit, the note, the close, and Z-report 1 on the order
screen's app and in the back office. Tests: `averageMoney`, `stampText`,
`serviceName`, `toleranceText`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16i: Settings** (ADR-193). `/backoffice/settings/*` has D22's frame
(`settings/SettingsScreen.tsx`): the title, the rail as a card with icons, links out to
Printers, Devices and Tables and floor plan, the version box, and the section's cards
across the rest. A section still saves itself. Venue's fields are two across. Seen in a
browser as Priya at 1920x1080: the three sections, a draft and the "Leave without
saving?" prompt from a link out, Discard. Test: `relatedScreens`. `make check` and
`make build-web` pass.

Before it, **Rebuild, Task 16h: Staff and roles** (ADR-192). `/backoffice/people` is D20, called
Staff in the sidebar (`people/PeopleScreen.tsx`, `RolePanel.tsx`): the staff table
with a state chip and a row menu, and beside it the roles as pills with the chosen
one's permissions as switches, its name, Reset and Save role. The role cards and the
role edit modal are gone. Seen in a browser as Priya at 1920x1080: a person picked, a
permission switched on, reset, saved and put back, and the row menus. Tests:
`peopleSummary`, `staffRows`, `appliesText`, `roleChanged`, `chosenRole`. `make check`
and `make build-web` pass.

Before it, **Rebuild, Task 16g: Inventory** (ADR-191). `/backoffice/stock` is D19, now called
Inventory (`stock/StockScreen.tsx`, `StockLevels.tsx`, `WastageCard.tsx`,
`levels-logic.ts`): the ingredients with what is in stock, the level each is low at,
its cost, a level bar and a Healthy, Low or Out chip; a row's actions under its "...";
Stock count and Log wastage beside the title; the week's wastage beside the table.
Recipes is still a view of its own. Seen in a browser as Priya at 1920x1080: the
table, Log wastage asking which ingredient, a row's menu, and stock received. Tests:
`levelOf`, `levelPercent`, `summary`, `tableRows`. `make check` and `make build-web`
pass.

Before it, **Rebuild, Task 16f: Menu management** (ADR-190). `/backoffice/menu` is D18
(`menu/MenuScreen.tsx`, `CategoryRail.tsx`, `ItemList.tsx`, `ItemEditor.tsx`,
`browse-logic.ts`): categories with counts, the items with a chip for where each
stands, and the item open in an inline editor with its photo and tags beside it; the
modal form and the table are gone. Search, a filter and an order on the list. Choices
and Stations and courses are still views of their own. Seen in a browser as Priya at
1920x1080 and 1366x768: an item edited and saved, a search, Sold out today and back
from a row's menu, a new item refused for its HSN and then added, and retired. Tests:
`scopeRows`, `itemState`, `listItems`, `canReorder`, `emptyLine`. `make check` and
`make build-web` pass.

Before it, **Rebuild, Task 16d3: an item's cost and margin** (ADR-189). Task 16d is done. The owner
chose the ledger over today's recipe. A sold stock movement keeps its dish (migration
00035) and the ledger keeps how many of a dish it costed (`stock_dish_sales`, 00036);
`GET /api/v1/reports/items` gives each item's net sales, cost and margin, and a margin
only when the ledger costed every unit sold. Item performance has Net sales, Cost and
Margin (`sales/ItemsScreen.tsx`). Seen in a browser as Priya at 1920x1080: a dish fully
costed with its margin, one part costed, several not costed, and a trend arrow. Go
tests in the domain, both services and over HTTP; vitest for the rows and words. `make
check` and `make build-web` pass.

Before it, **Rebuild, Task 16e: Staff reports** (ADR-188). `/backoffice/sales/staff` is D17's staff
half (`sales/StaffScreen.tsx`, `staff-logic.ts`), the third report tab: staff
performance (role, orders, sales, average, tips), Removed in error, the tips total, and
"Who sold it" in full, moved from Sales reports. The report carries `tips_by`, tips by
who took the payment (16e1, server). Seen in a browser as Priya at 1920x1080 over seven
days. Go tests in the domain; vitest for `staffLines` and `removedLines`. `make check`
and `make build-web` pass.

Before it, **Rebuild, Task 16d2: Items and inventory** (ADR-187). `/backoffice/sales/items` is D16
(`sales/ItemsScreen.tsx`, `items-logic.ts`), a tab beside Sales: item performance with a
trend against the days before, wastage as bars with its total, stock health, and under
"In detail" what sold in full and the closing stock, moved from Sales reports. The range
is shared by the report tabs (`report-range.tsx`, `use-range-sales.ts`). Seen in a
browser as Priya at 1920x1080: seven days and Today, the tabs keeping the days, four
write-offs keyed into the dev venue. Tests: `itemRows`, `wasteBars`, `stockHealth`.
`make check` and `make build-web` pass. Cost and Margin are split off as 16d3.

Before it, **Rebuild, Task 16d1: the wastage report** (ADR-186). `GET
/api/v1/reports/wastage`: write-offs over a range of trading days by stock item and by
reason, at the cost each had. Go tests in the service and over HTTP.

Before it, **Rebuild, Task 16c2: Sales reports** (ADR-185). Task 16c is done. `/backoffice/sales` is
D15 (`sales/SalesScreen.tsx`, `SalesCharts.tsx`, `SalesHead.tsx`, `overview-logic.ts`):
the title with the range pill and Export, four figures against the same number of days
before, the trend stacked by service, sales by category and orders by type as rings,
top sellers and payments; the old tables sit under "In detail". Seen in a browser as
Priya at 1920x1080 and 1366x768: seven days, Today picked from the range pill, the
Export list. Tests: `previousRange`, `rangeText`, `trend`, `namedBars`, `slices`,
`ringGradient`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16c1: the report by service type** (ADR-184). `by_service` on the
sales report's totals and on each period, and `sales.covers`, the guests on the bills.
Server only. Go tests in the domain.

Before it, **Rebuild, Task 16b: the dashboard** (ADR-183). Task 16 is split in ten (16a to 16j,
`rebuild.md`). `/backoffice/` is D12 (`dashboard/Dashboard.tsx` and its parts,
`dashboard-logic.ts`): today's sales, orders, active tables and average order beside
yesterday's, sales by the hour, top selling items, active tables, staff, low stock and
quick actions, each card for whoever may read it. No new endpoint; the sales report now
carries `roles` beside `names`. Seen in a browser as Priya at 1920x1080 and 1366x768
against the dev venue. Go test for the roles; vitest for `change`, `occupancy`,
`activeTables`, `topItems`, `staffRows`, `hourChart`, `axisTop`, `lowRows`,
`formatMoneyCompact`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 16a: the back office's frame** (ADR-182). The 72px bar (logo,
Online, the venue's date and time, who is signed in with Sign out under the name) and
the 240px sidebar with an icon on every entry (`shell/BackOfficeShell.tsx`,
`shell/SectionNav.tsx`). `ConnectionWord`, `ProfileMenu`, `use-disclosure` and
`status-logic` moved to `web/shared/shell`; `SignedInShell` is deleted. Seen in a
browser: the menu under the name, and Menu opened from the sidebar.

Before it, **Rebuild, Task 15f: Coming next** (ADR-181). Task 15 is done. Where D10 draws
"Upcoming", the board lists what the station will be sent when a later course is
fired: `order.Held` (`internal/domain/order/route.go`), `coming` and `coming_more` on
the board's response, and a card under Today's stats (`kitchen/BoardStats.tsx`,
`comingRows`). Seen in a browser at 1920 wide, on night: T2's starter sent and its
dessert listed as coming. Go tests in the domain, the service and over HTTP;
`comingRows` in vitest. `make check` and `make build-web` pass.

Before it, **A fix after 15e: moving a ticket is its own permission** (ADR-180, migration
00034). `kitchen.work` starts, finishes and recalls tickets; `kitchen.view` only reads
the board. The owner, the manager and the kitchen role hold it; a cashier and a waiter
do not, and the kitchen's API refuses them a move. On `/pass` someone without it sees
the board with no buttons. Seen in a browser: Ravi starting a ticket, Kabir's board
without actions. Go tests for the routes, the default roles and the migration.

Before it, **Rebuild, Task 15e: a cook signed in on a terminal has the board** (ADR-179). The
kitchen role lands on `/pass` (`kitchen/CookBoard.tsx`): the wall display's board with
the person's own session, outside the shell and the shift gate, with "Lock · name" in
the foot. The kitchen's API now also lets in a person signed in on any device who
holds `kitchen.view` (`kitchenAccess` in `internal/api/kitchen.go`), and their moves
carry their name. `/kitchen` is still the wall display with its own pairing. Seen in a
browser as Ravi: sign-in, the station chosen, a ticket started, night, and Lock back to
a paper sign-in. Go test over HTTP; `landingFor` in vitest. `make check` and `make
build-web` pass.

Before it, **Rebuild, Task 15d: today's stats** (ADR-178). The board's response carries
`stats` for the station's trading day: tickets sent, in the kitchen and ready, the
mean time from sent to ready, and the slowest by its dish (`kitchen.Stats` in
`internal/domain/kitchen/day.go`, the `StationDay` query, `Service.Board`). The board
shows them in a column from 1536px wide (`kitchen/BoardStats.tsx`), and as one line
under that. Seen in a browser as a kitchen display at 1920 wide against the dev venue.
Go tests in the domain, the service and over HTTP; `statRows` and `minutesText` in
vitest. `make check` and `make build-web` pass. The design's Upcoming is left out and
split off as 15f.

Before it, **Rebuild, Task 15c: espresso night and the board's foot** (ADR-177). The board
has a foot (`kitchen/BoardFoot.tsx`): Change station, Sound, Group by table and Night
mode, each kept on the device (`board-prefs.ts`, `mez.kitchen.prefs`). Night is the
same board on the ink surface (D11). Sound is a two-note chime for a new ticket
(`use-chime.ts`). Seen in a browser as a kitchen display at 1920x1080: night on and
off, the switches, and the choices kept across a reload. Tests: `readPrefs`,
`writePrefs`, `surfaceOf`, `groupByTable`, `arrivals`. `make check` and `make
build-web` pass.

Before it, **Rebuild, Task 15b: the kitchen's board** (ADR-176). `/kitchen` is D10 on paper
(`kitchen/KitchenBoard.tsx`, `TicketCard.tsx`, `boardColumns` in `kitchen-logic.ts`):
the header, the all-day strip, and three columns, New Orders with Start preparing,
Preparing with Mark ready and a bar towards late, Ready for pickup with Recall. Seen
in a browser as a kitchen display at 1920x1080 and 1366x768 against the dev venue: a
ticket started, marked ready and recalled. Tests: `boardColumns`, `metaText`,
`latePercent`, `agoText`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 15a: "Start preparing", from the server** (ADR-175). Task 15 is
split in five. A ticket a cook has picked up is in progress: `ticket.started`, `POST
/api/v1/kitchen/orders/{id}/tickets/{seq}/start`, and the board's tickets carry
`started_at`. A bump still works from fired; a recall puts a ticket back as new.
`ProjectionVersion` is 8. No migration. Server only: the display has no Start button
yet, and keeps showing a started ticket as one to make. Go tests in the domain, the
service, the hub and over HTTP. `make check` passes.

Before it, **Rebuild, Task 14c: the shift summary** (ADR-174). Task 14 is done. `/waiter/shift`
is T08 (`waiter/ShiftSummary.tsx`, `shift-summary-logic.ts`), under More, "Your shift":
who is signed in and how long they have been on, tables served, orders taken, sales
handled, tips collected, tips by table as bars, and End shift, which signs them out.
Seen in a browser at 1366x1024 as Kabir against the dev venue: the figures after the
bills of 14a, then a card payment with a tip at T3 and its bar. Tests: `shiftSpan`,
`onShiftText`, `tipBars`. `make check` and `make build-web` pass.

Before it, **Rebuild, Task 14b: one person's shift, from the server** (ADR-173). 14b was split
again: the server could say nothing per person. `GET /api/v1/shift/mine` answers for
whoever asks, in the open shift: the orders they opened, how many ended at a table,
what their payments settled, the tips on them, those tips by table, and when they first
touched an order (`shift.MineOf` in `internal/domain/shift/mine.go`, `Service.Mine`,
`MyShiftResponse` in `api-types.ts`). Server only, no screen. Go tests in the domain,
the service and over HTTP. `make check` passes.

Before it, **Task 14a: the waiter's bill** (ADR-172). Task 14 is split in two. For a
waiter `/pay/:id` is T05 (`waiter/WaiterBill.tsx`, `BillShares.tsx`, `BillCollect.tsx`,
`bill-logic.ts`): one column with the table and its total, Split equally with the
guests counted (it starts at one, "Whole bill"), a row for each share, the share being
collected by UPI, card or cash with a tip, what has been collected, Print receipt once
there is an invoice, and Close table, which returns to the tables. By item and Custom
open the desktop's split panel. Seen in a browser at 1366x1024 as Kabir: T5 split in
two, one share by UPI, one by cash with change, Paid in full, and Close table leaving
T5 to clear; and a bill already paid. Tests: `shareName`, `eachShare`, `shareRows`,
`tablePays`, `closeBlock`. `make check` and `make build-web` pass.

Before it, **Task 13b: transfer and merge** (ADR-170). Task 13 is done. Transfer and
Merge on the table's screen open T06 as a modal (`waiter/MoveTable.tsx`,
`move-logic.ts`): the party's items, total and guests, the two modes, every other
table as a tile (free ones can be taken, the rest say why not), and a button that says
what it will do. Seen in a browser at 1366x1024 as Kabir: T3 joined to T2 ("Table T2 +
T3", Merged), let go again, and the party moved to T5 with the screen following it.
Tests: `moveTiles`, `pickTile`, `moveLabel`, `moveActions`. `make check` and `make
build-web` pass.

Before it, **Task 13a: the table's own screen** (ADR-169). Task 13 is split in two.
`/waiter/table/:id` is T03 (`waiter/TableDetail.tsx`, `TableDetailOrder.tsx`,
`TableItems.tsx`, `OrderProgress.tsx`, `table-logic.ts`): the table with its guests and
how long it has sat, its notes and Add note, its items by course with each one's state
and Fire, Order Status in four steps, Bill and pay, Details, the totals and Send to
Kitchen (n), which opens the confirm. The top bar there is Back. A waiter's card with
guests at it leads here; a free table or one to clear still leads to the floor. Seen in
a browser at 1366x1024 as Kabir: the screen, a note added, Details, a send confirmed,
the list ungrouped, Add Item, and a free table's card. Tests: `detailOrder`,
`orderProgress`, `startedText`.

Before it, **Task 12c: Course View** (ADR-167). Task 12 is done. A waiter's rail has
two tabs, Current Order (n) and Course View (`register/CourseView.tsx`, `courseGroups`
in `send-logic.ts`).

Before it, **Task 12b: confirm and send** (ADR-166). A waiter's Send to Kitchen opens
T04 as a modal (`register/ConfirmSend.tsx`, `sendPreview` in `send-logic.ts`): how many
items go, how many the kitchen has already, how many wait for their course, the ticket
as the kitchen will read it, Back and "Send N items to kitchen"; then a toast. Seen in
a browser at 1366x1024 as Kabir: two new items beside one already sent, sent, the toast
and the button reading "Sent". Tests: `sendPreview`.

Before it, **Task 12a: the waiter's order screen** (ADR-165). Task 12 is split in three.
For a waiter `/pos` is T02, the order screen rearranged (`OrderScreen` and its parts
take a waiter's variant; `useWaiterFrame` says who): the top bar is the order's
(`waiter/WaiterOrderBar.tsx`: the way back, the table, the guests, how it is served),
the search sits over the categories, Popular is where it starts, three cards across,
and the rail has the table with its chips, each line's course, Clear, the totals,
Save and Send to Kitchen. Pay is "Bill and pay" under the rail's "...". Seen in a
browser at 1366x1024 as Kabir, beside the design: a seated table's order, items added,
Clear confirmed and carried through, Send, the "..." menu, and Save returning to the
tables. Tests: `clearableLines`, `courseNamer`. `make check` and `make build-web` pass.
The owner has not yet looked.

Before it: Task 11 (ADR-163, the waiter's home; ADR-164, the connection word in a
waiter's bar), Task 10c (ADR-162, the refund modal and the print queue on Orders), Task
10b (ADR-161, the Orders screen), Task 10a (ADR-160, the day's orders
from the server), Task 9 (ADR-159, split and the card and UPI states), Task 8 (ADR-156,
payment), Task 7 (ADR-155, tables), Task 6 (ADR-153, the item modal; 6b, ADR-154),
Task 5 (ADR-151, the order screen), Task 4 (ADR-150, Open the shift; `shift.open`,
ADR-152, migration 00033), Task 3 (ADR-149), Task 2 (ADR-148), Task 1 (ADR-145 to
ADR-147), Task 0 (ADR-144), the plan (ADR-143). The owner has not yet looked at 9, 10b,
10c, 11, 12 (a, b, c), 13 (a, b) 14 (a, c), 15 (b to f) or 16 (a to j).

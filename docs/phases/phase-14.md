# Phase 14 - mez Cloud: the account, the pairing and the switch

## Goal
An owner signs up with a phone number, pairs their venue with one code, and
opens their back office at `app.mezpos.com` from anywhere: live through the
relay while the venue is on, and read-only over the replica, marked "as of",
when it is off. A subscription in Razorpay decides, through one date in one
database, whether the relay carries the venue. A venue whose PC dies is back
on a fresh machine from the cloud by signing in. The boundary, the two
identities and the mirror are ADR-312.

## Depends on
Phase 12 (Litestream in the binary, `mez restore`), Phase 13 (the back office
across the relay, the remote rules). Phase 13 Task 5 (remote sessions harder
to take) lands before Task 5 here.

## Where the code lives
Two repositories. **This one** holds everything the venue binary and the open
relay do: pacing the backup, the `Entitlements` interface, pairing, the cloud
owner's sign-in, `mez mirror`, restore from the cloud. **`mez-cloud`**,
private, holds accounts, venues, keys, Razorpay, the account area, the admin
area and the mirror multiplexer: one Go service on one droplet, SQLite
underneath, importing this module (Task 8). A task below says which repo. Tasks in this repo
follow the session protocol; `mez-cloud` has its own STATE and phase file,
and its first commit copies this phase's cloud tasks there.

## What stays true
- The counter never needs any of it. No relay, no account, no subscription:
  the venue is exactly what Phase 12 shipped.
- Nothing in the binary checks a licence. A lapsed subscription is a relay
  that stops answering, and nothing else.
- The venue's staff, PINs and devices never leave the venue's database. The
  cloud's accounts never enter it. ADR-312 names the two meeting points, and
  there is no third.
- Money through the relay stays impossible (Phase 13). The mirror refuses
  every write.
- The owner can leave: cancel, export, and restore onto their own machine
  with no cloud at all.

## Tasks

- [x] **Task 1 - ADR-313: the entitlement states and what each Razorpay
  event does to them.** ADR before code. States: `trial`, `active`, `grace`,
  `suspended`, `cancelled`, `closed`. One column, `entitled_until`, that the
  relay reads; a second, `backups_kept_until`, that the mirror and restore
  read, thirty days later than the first. The table of Razorpay events
  (`subscription.charged`, `.pending`, `.halted`, `.cancelled`,
  `payment_link.paid`, `payment.failed`) to state and date. The manual lane:
  a yearly Payment Link, and an admin action that sets the date by hand with
  a note, for cash. Trials are ninety days from pairing. Grace is seven days.
  The ADR also states the per-venue cost model it protects
  (`docs/marketing/cloud-unit-economics.md`): a venue must cost under ₹100 a
  month to serve.
  *Done: ADR-313. It adds a third date, `paid_until`, for the owner to
  read; puts the grace into `entitled_until` when the date is written; and
  holds a venue paired to mez Cloud to a 30 s backup at the fastest.*

- [x] **Task 2 - The backup is paced on purpose.** *This repo.*
  As built (ADR-315): the setting is `backup_sync_interval`, flat like the
  rest, `30s`, from `5s` to `10m`. Counting against a stand-in bucket
  showed the writes were not most of the bill: Litestream's tidying lists
  the bucket 480 times an hour and mez's own look added 60, and a listing
  is billed as a write. So the merges, the tidying and the look are paced
  too, in `internal/backup/pace.go`. `status.sync_interval_seconds` is on
  `GET /api/v1/settings/backup`. The test counts 183 billed requests in
  an hour of selling and 16 in an hour of none: ₹32 a month, not ₹24.
  **The last sentence below is wrong**: the invoice numbers of the lost
  half minute are lost with it and given again after a restore;
  `deploy.md` says that. The task as it was set: Litestream's
  default syncs every second, which is about 1.5 million R2 writes a month
  for a busy cafe and ₹594 of a ₹699 subscription. A new setting,
  `backup.sync_interval`, default `30s`, floor `5s`; the part size raised to
  match. The engine reports the interval in `GET /api/v1/backup`. A test
  counts uploads over a simulated hour of writes at the default and asserts
  the ceiling. `deploy.md` says what the interval means for how much can be
  lost: a cafe at 30 s loses at most half a minute of orders, and the
  invoice numbers it had given are in the events anyway.

- [x] **Task 3 - The relay asks who is entitled.** *This repo.* Split on
  2026-10-12 while building it (ADR-316): the relay's half is one session
  and the venue's is another.

  - [x] **Task 3a - The relay's half.** `relay.Entitlements` and
    `StaticEntitlements` (`internal/relay/entitlement.go`),
    `RELAY_ENTITLEMENTS`. Asked at the dial (403 `not_entitled`, never the
    401 of a bad key) and again each hour and at the date (close 4003 with
    the reason). `Remote` turned false is 404 on the open socket. The
    venue is told where it stands in `X-Mez-Entitlement` on the answer to
    its dial, and in a `Notice` frame sent only to a venue that asked with
    `X-Mez-Tunnel-Notices: 1`, since an installed venue hangs up on a frame
    it does not know. `*` in the file is every venue not named; without it
    they are not carried. `RELAY_REVOKED` is left to the key check. An
    error from the store closes nobody. README and `compose.yml`.
  - [x] **Task 3b - The venue's half.** As built (ADR-317):
    `tunnel.Standing` on `Status`, `StateNotEntitled` with a wait of half a
    minute to a minute, `cloud` on `GET /api/v1/tunnel`, the chip **Not
    carried**, `cloudLine` on the ordering card, and the second card
    saying when the relay does not carry the back office. Tested from a
    real relay through to the API's status; the two cards were not walked
    in a browser. As it was set: In `internal/tunnel`: send
    `wire.HeaderNotices`, read `wire.HeaderEntitlement` off the dial's
    answer (a 403 too) and `wire.TypeNotice` in `session.read`, keep what
    was told on `Status`; a state for not entitled (403 `not_entitled` at
    the dial, close code `wire.CloseNotEntitled` after) that tries again
    within a minute and not in the five a refused key waits. In
    `internal/api`: `not_entitled` in the tunnel's vocabulary and `cloud:
    {state, paid_until, until, back_office, reason}` on `GET
    /api/v1/tunnel`, absent when the relay said nothing. The Devices
    screen: the ordering card's sentence for not entitled, and the cloud
    line ("Cloud: paid until 12 Nov", "Payment failed, 5 days left") from
    `paid_until` and `until` (read `docs/design/tokens.md` and the Devices
    screen's file first). Tests in `internal/tunnel` against a real
    `relay.Server` with entitlements a test changes, in `internal/api`,
    and in the card's logic. `deploy.md` where it lists the tunnel's
    states.

  The task as it was set:
  `relay.Entitlements` with one method: for a venue id, `Until time.Time`,
  `Remote bool`, `State string`, `PaidUntil time.Time`, `Reason string`
  (ADR-313: the relay enforces the first two and passes the rest on).
  `StaticEntitlements` reads
  `RELAY_ENTITLEMENTS` (a file of `venue until remote` lines) and keeps
  `RELAY_REVOKED` working as "until zero". The agent is checked when it
  connects and once an hour after; a venue past its date is closed with
  `not_entitled` and the reason, and a venue whose `Remote` turned false
  has `/backoffice/` and `/api/` answered 404 as a guests-only key is. The
  tunnel client surfaces the close reason and tries again at least once a
  minute after a `not_entitled` close; `GET /api/v1/tunnel` gains
  `cloud: {state, paid_until, until, reason}` so the Devices card can say
  "Cloud: paid until 12 Nov" or "Payment failed, 5 days left". Tests in `internal/relay`
  and `internal/tunnel`. `build/relay/README.md` documents the file for
  self-hosters.

- [ ] **Task 4 - Pairing the venue to an account.** *Both repos.* Split on
  2026-10-12 before building it (ADR-318). It does not write `config.toml`:
  a pairing is `cloud.json` in the data directory.

  - [x] **Task 4a - The pairing itself.** *This repo.* `internal/cloud`:
    `CleanCode`, `Client.Pair` (one POST to `{cloud_url}/v1/pair`, no
    redirect followed), `Pairing.Validate`, `Load`, `Save`, `Remove`, and
    the setting `cloud_url`. The request and the answer are written down
    in ADR-318 for `mez-cloud` to build to. Tests against a stand-in
    pairing server. Nothing uses a pairing yet.
  - [x] **Task 4b - The venue uses its pairing.** *This repo.* Split in
    two on 2026-10-12 while building it (ADR-319).
    - [x] **4b1 - At start.** `cmd/mez/cloud.go`: `loadPairing`,
      `withPairing` (the relay, its key, the back office, `public_url`; a
      relay in the settings file wins whole and the pairing is set aside
      with a warning), `cloudSyncFloor` through
      `backup.ManagerOptions.SyncFloor` (30 s for the pairing's bucket and
      for no other destination), and `paired_to: {owner, hostname,
      paired_at}` on `GET /api/v1/tunnel`, beside `cloud` and not in it. A
      pairing that cannot be read stops the start. Tested by starting the
      whole server from a `cloud.json` against a real relay.
    - [x] **4b2 - While running.** As built (ADR-320): no restart.
      `internal/service/cloudpair` refuses what it can before the code is
      sent, keeps the pairing first once it is spent, and reports a tunnel
      or a backup that failed beside a venue that is paired. `cmd/mez`'s
      `venueLink` holds the pairing, the tunnel and the address, and the
      API reads each through it (`api.WithPublicURL`). The venue's own
      backup is kept; none, or the pairing's own bucket, is set. `POST
      /api/v1/cloud/pair`, six tries a minute. As it was set: A service in
      `internal/service` that
      pairs: refuse, with the reason, a venue whose settings file names a
      relay; `cloud.Client.Pair` with the venue's `public_id`, name and
      version; `cloud.Save`; the backup's destination set to the
      pairing's bucket through `backup.Manager.Set`, unless the venue
      already backs up somewhere and has not said to move (decide which,
      and say it in the answer); and whether a restart is needed. Decide
      first whether the tunnel, the table codes' address and the
      gateway's return address are made to change while running or the
      answer is "restart mez": they are all read at start today. `POST
      /api/v1/cloud/pair` for whoever holds `settings.edit`, at the venue
      only (in `staysAtTheVenue`), rate limited, answering the owner's
      name, the hostname and `restart_needed`, never a key. Tests with a
      stand-in pairing server.

    As 4b was set: ADR first:
    which wins when the settings file names a relay and there is a pairing
    too. At start, `cmd/mez` loads `cloud.json` and from it takes the
    relay's address and key, `public_url`, and the back office across the
    relay; a file that cannot be read stops the start. A service in
    `internal/service` that pairs: `cloud.Client.Pair` with the venue's
    `public_id`, name and version, `cloud.Save`, the backup's destination
    set to the pairing's bucket through `backup.Manager.Set`, and the
    tunnel started or told a restart is needed, as the backup already does
    it (ADR-276). A paired venue's backup goes no faster than 30 s whatever
    `backup_sync_interval` says (ADR-313, ADR-315). `POST
    /api/v1/cloud/pair` for whoever holds `settings.edit`, at the venue
    only (put it in `staysAtTheVenue`), rate limited, answering the
    owner's name and the hostname and never a key; `paired`, the hostname
    and the owner's name in the `cloud` block of `GET /api/v1/tunnel`.
    Tests with the stand-in pairing server and a real relay.
  - [x] **Task 4c - The screens.** *This repo.* Split in two on 2026-10-12
    (ADR-321): the Devices card works for every venue and was the way to
    run pairing in one piece; the wizard's step is its own session.
    - [x] **4c1 - The Devices card.** At the foot of "Back office from
      anywhere": who the venue is paired to, and **Connect to mez Cloud**
      (or **Pair again**), which opens a modal for the code. Walked in a
      browser against `build/dev/pairstub` and a local relay: a code that
      could not be one, a code the cloud refuses, the right code, the
      tunnel connected with no restart, a guest's request through the
      relay answered by the venue.
    - [x] **4c2 - The wizard's step.** As built (ADR-322): the fifth of
      six steps, after the printer, when the venue exists and the owner is
      signed in, so it calls the same route with nothing new on the
      server. Skippable. Its words are `web/shared/cloud-pair.ts`, which
      the Devices card now uses too. Walked in a browser on a venue made
      in the wizard: a code that could not be one, then the right one,
      then on to More devices. As it was set: "Connect to mez Cloud" in
      the setup
      wizard, skippable. Read `docs/design/screens/setup-wizard.md` and
      `web/shared/screens/setup/` first: the wizard runs before anyone is
      signed in, and `POST /api/v1/cloud/pair` wants a signed-in owner, so
      decide where the step goes (after the owner exists and is signed
      in, or a route of the wizard's own behind its setup token). Reuse
      `cloud-logic.ts` and the modal's form.

    As 4c was set: The setup wizard's step
    "Connect to mez Cloud", skippable, and the same entry on the Devices
    screen's "Back office from anywhere" card for a venue set up earlier:
    eight boxes or one field for the code, what pairing does in two
    sentences, each refusal in the owner's words, and after it the address
    and who it is paired to. Read `docs/design/tokens.md` and the two
    screens' files first. Walk it in a browser against the stand-in.
  - [ ] **Task 4d - The cloud's side.** *`mez-cloud`.* Issue an
    eight-character code for a venue of an account, good for an hour and
    once; answer `POST /v1/pair` as ADR-318 sets out; mint the relay key
    and the bucket's credentials (8b); mark the venue paired and start its
    trial, once per `venue.id` (ADR-313); a second pairing replaces the key
    and revokes the old one. Tests on the code's life and single use.

  The task as it was set: The cloud
  issues an eight-character pairing code for a venue, good for an hour. The
  setup wizard gains a step, "Connect to mez Cloud", and the Devices
  screen's "Back office from anywhere" card gains the same entry for a venue
  set up earlier. The venue posts the code once to `api.mezpos.com/v1/pair`
  and receives the relay key (remote scope), the venue hostname, R2
  credentials scoped to that venue's prefix, the cloud's Ed25519 public key
  and the owner's display name. It writes a `[cloud]` section in
  `config.toml` (the key under the same protection as `relay_key`,
  ADR-307), restarts, and connects. The cloud marks the venue paired and
  starts the trial. A second pairing of the same venue replaces the key and
  revokes the old one. The venue's `cloud` block in `GET /api/v1/tunnel`
  says `paired`. Venue-side tests with a fake pairing server; cloud-side
  tests on the code's life and single use.

- [x] **Task 5 - The owner signs in from the cloud.** *This repo, after
  Phase 13 Task 5.* Split on 2026-10-12 before building it (ADR-323): a
  session belongs to a device and the owner's browser was never paired as
  one, so there is a device to make as well as a person, and the route is
  a session of its own.

  - [x] **Task 5a - The assertion, the person and the sign-in.** As built
    (ADR-323): `internal/cloud/assertion.go` (`v1.<payload>.<signature>`,
    `SignAssertion` for `mez-cloud`, `VerifyAssertion`; the subject is the
    word `owner`, the audience the venue's `public_id`, five minutes and a
    minute's grace each side). Migration 00055: `staff.cloud`,
    `devices.cloud`, `cloud_assertions`. `internal/auth/cloud.go`:
    `PairCloudOwner` (made or restored at pairing, through
    `cloudpair.Options.Owner`), `EnsureCloudOwner` (at start, for a venue
    paired by an older build, never restoring), `SignInCloud` (spends the
    nonce and opens ADR-314's session in one transaction). The person has
    no PIN: `verify` answers any as it does for someone deactivated, the
    People service and `mez reset-pin` refuse to give one, and they are
    off the sign-in tiles and out of the count of who can manage staff.
    **Nothing calls `SignInCloud` yet.**
  - [x] **Task 5b - The route, and what the screens say.** Split on
    2026-10-12 while building it (ADR-324): the server is one session and
    the two screens another.
    - [x] **5b1 - The route, and being the cloud's device after it.** As
      built (ADR-324): `POST /api/v1/sessions/cloud`, relayed only, with
      `Authorization: Mez-Cloud <assertion>`, thirty a minute, answering
      the session and its device. After it the session's token is
      presented as both `Authorization: Bearer` and `X-Staff-Token`:
      `requireDevice` asks `auth.StaffAuth.CloudDevice` first, for a
      relayed request only. `internal/service/cloudsignin` holds the
      order: the key of the pairing in use (`venueLink.cloudKey`), each
      venue's `public_id` tried as the audience, then `SignInCloud`.
      Refusals read alike (401 `cloud_sign_in_refused`) but the clock's
      (401 `cloud_clock`); not relayed is 403. `by=cloud` in the access
      log. `cloud` on a person in `GET /api/v1/people` and on a device in
      `GET /api/v1/devices`. `build/dev/pairstub` answers `GET
      /dev/assertion?venue=<public id>`. `deploy.md`, under "The back
      office from anywhere". **Walked with curl** against a real relay,
      the stand-in and `bin/mez`: paired by code, signed in through the
      relay, a replay and another venue's assertion refused, the token
      refused on the LAN, an order refused, and the session dead the
      moment "mez Cloud" was revoked on Devices.
    - [x] **5b2 - The People and Devices screens say what these are.** As
      built (ADR-325): on People, "Signs in from mez Cloud" under the
      name, no **New PIN**, and Edit and Deactivate each saying what this
      person is and what the change shuts; on Devices, a cloud for its
      mark and "Your mez Cloud account's way in. Nobody holds it" in
      place of its kind, no **Rename**, and Revoke as **Shut mez Cloud's
      sign-in**. `staffActions` and `deviceActions` in the two logic
      files, tested. **Walked in a browser** at 1366 by 768 on a venue
      paired against the stand-in: the sign-in tiles without the person,
      both rows, both menus, the three dialogs, and the device revoked.

  The task as it was set: At pairing the venue creates one staff member, kind
  `owner`, named from the account, with no PIN and the owner's permissions,
  marked as the cloud's. A relayed request may carry `Authorization: Mez-Cloud
  <assertion>`: Ed25519-signed by the cloud, audience the venue id, subject
  that staff member, five minutes of life, a nonce kept for those five
  minutes. The venue verifies it with the public key from pairing and opens
  the same twelve-hour remote session Phase 13 Task 5 defined. Nothing else
  changes: the remote allow list (ADR-308), the access log (`via=relay`,
  now also `by=cloud`), the read-only screens (ADR-309). A venue that is
  not paired has no such staff member and refuses the header. Tests: a
  valid assertion signs in, an expired, replayed, wrong-audience or
  wrong-key one does not, and the staff member cannot be given a PIN.

- [ ] **Task 6 - `mez mirror`: the back office over a backup.** *This
  repo.* Split on 2026-10-12 before building it (ADR-326).

  - [x] **Task 6a - A mirror over one restore.** As built (ADR-326):
    `api.WithMirror` makes the ordinary server a mirror. Every request is
    taken as one from away, so ADR-308's allow list decides what is
    reached; every request that is not a read is 409 `mirror_read_only`
    before its handler, but six named (`mirrorWrites` in
    `internal/api/mirror.go`); the guest app is not served; every answer
    carries `X-Mez-Mirror`, when the copy was last written. `mez mirror
    -from FOLDER` (or a bucket, or `-config`), `cmd/mez/mirror.go`:
    restores a generation and its photos into a directory under the
    system's temporary one, migrates the copy, serves on
    `127.0.0.1:8090`, prints a pairing code for one back office, and
    removes the directory when stopped. The print queue is built and
    never started. Sign-in is the venue's own PINs, on the copy.
    `deploy.md`, "Reading a backup without restoring it". **Walked in a
    browser** on a venue backed up to a folder: paired, signed in, the
    dashboard and People read, and a change refused in the mirror's
    words.
  - [x] **Task 6b - Following the backup.** Split on 2026-10-12 before
    building it (ADR-327): a copy that is followed cannot hold the
    mirror's own state, so that moves out first.
    - [x] **6b1 - The mirror's own door.** As built (ADR-327):
      `internal/mirror`. A `Keeper` remembers the browsers paired to the
      mirror and each person's wrong PINs and lock in
      `<state dir>/<venue public id>.json` (`-state-dir`, by default
      `mez/mirror` under the user's configuration directory), and
      sessions in memory. `mirror.Auth` is the API's `Devices` and
      `Staff` both, over one copy: people, roles and the venue's devices
      are read from the copy; a PIN is judged by the new read-only
      `auth.StaffAuth.CheckPIN` and counted by the keeper. Three wrong in
      a row lock that person on the mirror for a day. The venue's own
      device tokens open nothing. The copy is not written after it is
      migrated.
    - [x] **6b2 - Following.** As built (ADR-328): in `cmd/mez/mirror.go`,
      `mirrorSource.follow` looks at `backup.Generations` every `-follow`
      (a minute; `0` for never), and only if a request has arrived since
      its last look. A later last write, or another generation, is
      restored with its photos into a new directory, migrated and opened
      (`mirrorSource.open`), given a server with the same `Keeper`, and
      put in the old one's place by `mirrorHolder.swap`; the old copy is
      closed and removed once its requests are done. Whatever fails
      leaves the old copy serving. `X-Mez-Mirror` is now to the
      nanosecond. Tested against a venue that trades again after the
      mirror started: the newer name shows, the time moves, the session
      holds, the first copy's directory goes.
  - [ ] **Task 6c - mez Cloud's sign-in on a mirror.** Photos are kept
    up already: each copy is restored with its own (ADR-328).
    `POST /api/v1/sessions/cloud` is already let through the
    mirror's gate and answers 401, since no mirror is given a key. The
    key is not in the backup (`cloud.json` is beside the database, not in
    it): `mez mirror -cloud-key`, or `mez-cloud` handing it over
    in-process (8e). The person the cloud signs in as is in the copy; the
    device is too. `mirror.Auth.CloudDevice` answers nobody today, and
    the keeper has no cloud sessions: that is where it goes.
  - [ ] **Task 6d - What the screens say on a mirror.** Read
    `docs/design/tokens.md` and `backoffice-shell.md` first. The bar:
    "Mirror, as of 23:10", from `X-Mez-Mirror` (`web/shared/api.ts`
    already reads `X-Mez-Via` the same way). The connection pill: "Venue
    offline" when that is more than two minutes old. And every edit a
    mirror refuses taken off the screens, as ADR-309 took off what cannot
    be done from away: today they are offered and refused on save. Two
    reads answer wrongly on a mirror and need words of their own: the
    Floor plan's table codes (403, since a mirror serves no guests) and
    Settings, Backup (404 `backup_unavailable`).

  The task as it was set: `mez mirror -from <destination> [-as-of]` restores the latest
  replica into a temporary data directory, keeps following it, and serves
  only the back office bundle and the API routes in the remote allow list,
  with every write answered 409 `mirror_read_only` before the handler. The
  bar shows "Mirror, as of 23:10" and the connection pill reads
  "Venue offline" when the replica is older than two minutes. Photos come
  from the replicated `photos/`. The mirror accepts the Mez-Cloud header of
  Task 5 and also a PIN sign-in against the replicated staff table, so a
  self-hoster can use it with no cloud. Tests: writes refused, reads equal
  to the source, the "as of" time right after a sync.

- [ ] **Task 7 - Restore from the cloud, and pair again.** *Both repos.*
  The wizard's first step gains "I had mez before". It asks for the
  account's phone number, sends the one-time code, lists the account's
  venues with the time of their last backup, and restores the chosen one
  through the existing `mez restore` path from R2, with the credentials the
  cloud hands over for that one restore. Then it pairs as Task 4 does, with
  a fresh key, and revokes the old one. Devices in the restored database
  still hold their tokens, so tablets reconnect once they find the new
  machine through the discovery Phase 12 built. `deploy.md` gains "The PC
  died" as a numbered procedure, tested by doing it: a venue backed up,
  the data directory deleted, the venue back by code, a sale rung.

- [ ] **Task 8 - mez Cloud, the service.** *`mez-cloud`.* Split there into
  its own tasks; the shape is fixed here. One runtime, chosen on
  2026-10-12 after a Worker split was weighed and set aside (ADR-312, Cost):
  a Go service on the DigitalOcean Bangalore droplet the relay needs anyway,
  importing this module for keys, reports, exports and the mirror, with
  SQLite as its database, one writer, replicated to R2 by the same
  Litestream engine a venue uses. The account area is a third React bundle
  in this repo's design system. Cloudflare does what it does today: DNS,
  TLS, the proxy, R2, and Access in front of the admin area. Postgres comes
  only if a need appears; Workers do not.
  - [ ] **8a - Accounts and venues.** An account is a phone number proven
    by a one-time code sent as a WhatsApp authentication template (MSG91
    SMS for a number not on WhatsApp), with a name and an email for
    invoices. An account owns venues; a venue has a slug, a hostname, a key
    id, an R2 prefix, a state and the two dates of Task 1. Sessions are
    cookies, thirty days, revocable, built on the session, lockout and
    rate-limit patterns `internal/auth` already has. Meta's business
    verification starts on day one of this task; it can take weeks.
  - [ ] **8b - Keys and credentials.** Relay keys minted with the same
    `relay.Keys` this repo exports. R2 credentials per venue prefix through
    Cloudflare's API tokens, rotated on re-pairing. The Ed25519 key pair for
    Task 5 on disk under the same protection as the relay secret, with
    rotation that keeps the previous public key valid a week.
  - [ ] **8c - Razorpay.** Plans created once in the dashboard: Cloud
    monthly, Cloud yearly, and the founder versions (ADR-313 names the
    prices). "Subscribe" creates a Subscription with `start_at` the trial's
    end and returns the hosted checkout. One webhook endpoint, signature
    verified, idempotent on event id, mapping events to states by Task 1's
    table in one transaction. Payment Links for the manual lane. Test mode
    end to end before live keys.
  - [ ] **8d - The account area**, `app.mezpos.com`. Sign in, venues with
    each one's today and last seen, each venue's state, plan, next charge,
    invoices, "Subscribe", "Cancel at the end of the period", "Download my
    data" (the venue's latest replica as a zip), "Delete my account" (after
    cancel and a thirty-day wait). Phone first; it is where the owner is.
  - [ ] **8e - The venue page and the mirror**, `app.mezpos.com/v/<slug>/`.
    Behind the owner's session. While the venue is connected, the back
    office through the relay with the Task 5 assertion minted in-process;
    when it is not, a `mez mirror` instance over the replica, kept for
    venues opened in the last hour, with the switch made by the connection
    state and said on the bar.
  - [ ] **8f - The admin area**, `admin.mezpos.com`, for the operator:
    venues by state, revenue, failed charges, the hand-set date with a note,
    revoke, impersonate-read a venue's state. Behind the operator's account
    with a second factor, and Cloudflare Access in front.
  - [ ] **8g - Entitlement, in the same process.** The relay's
    `Entitlements` implementation reads the venue table directly, on
    connect and once an hour per connected venue. Phase 15's reader and
    the pending-bundle queue of Phase 16 read the same table.
  - [ ] **8h - Dunning.** WhatsApp from the Razorpay events: seven days
    before a charge, on failure, at day three, at suspension, on
    reactivation. The templates are approved alongside 8a's; until then,
    email through SES.
  - [ ] **8i - The box itself.** One droplet to start, Docker Compose as
    `build/relay` already does it: the service, Caddy, Litestream for the
    control plane's own database to R2. Nightly snapshots. A rebuild script
    tested by rebuilding onto a fresh droplet from the snapshot and the
    replica, timed, and written in the cloud repo's deploy doc. The droplet
    is the one point of failure for every paid feature, so this is where the
    uptime effort goes.

- [ ] **Task 9 - Words.** *This repo.* `deploy.md`'s owner section becomes
  "mez Cloud, or your own relay": the code, what pairing does, what stops
  when a subscription lapses and what never does, "The PC died". The
  landing brief's pricing column rewritten per
  `docs/marketing/cloud-unit-economics.md`. `00-product.md`'s licence
  section cites ADR-312.

## Done when
- A new venue is paired by code in under two minutes, in person and over a
  remote desktop, with nothing typed but the code.
- The owner opens `app.mezpos.com` on mobile data and sees live sales
  with the venue on. The venue PC is shut; the owner refreshes and sees
  the same screen marked "as of" the last minute it was on.
- A Razorpay test subscription is charged, fails, is retried and is
  halted, and each webhook moves the venue's state as ADR-313 says.
- A venue whose date has passed is refused by the relay within the hour
  and is back within a minute of a payment, with the Devices card saying
  why at every step.
- The friend's cafe's data directory is deleted on purpose and the venue is
  back on that machine by code, with a sale rung and an invoice printed.
- A self-hoster can run the relay with a static entitlements file and
  `mez mirror` against a folder, from the docs alone.

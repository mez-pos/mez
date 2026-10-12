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

- [ ] **Task 3 - The relay asks who is entitled.** *This repo.*
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

- [ ] **Task 4 - Pairing the venue to an account.** *Both repos.* The cloud
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

- [ ] **Task 5 - The owner signs in from the cloud.** *This repo, after
  Phase 13 Task 5.* At pairing the venue creates one staff member, kind
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
  repo.* `mez mirror -from <destination> [-as-of]` restores the latest
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

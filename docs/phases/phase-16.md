# Phase 16 - Two or three outlets in one login

## Goal
An owner with more than one venue opens one page and sees them together:
today across all outlets, each one against the others, the items that
sell everywhere and the ones that sell in one place. Their accountant gets
one mail a month with every outlet's exports. A menu change made once is
pushed to the other outlets, including the ones that are closed, which
take it when they open. Each venue stays its own single-writer database;
the cloud reads them all and writes to each through the door Phase 10 made
for spreadsheets.

## Depends on
Phase 14 (accounts owning several venues, the mirror), Phase 15 (the
reader over replicas). Phase 10's import (menu, prices, choices, stock)
is the transport for a push.

## Where the code lives
The rollup, the export mail and the push queue are `mez-cloud`. This repo
adds the venue's side of a push: pulling and applying a pending bundle on
connection, and saying in the access log that it came from the cloud and
who sent it.

## What stays true
- One venue, one database, one writer. The cloud never writes to a venue's
  database; it hands the venue a bundle, and the venue's own import applies
  it under its own rules and its own transaction.
- A venue is never changed silently. Every push is a line in that venue's
  access log with the owner's name and the source venue, and the back
  office at the venue shows "Menu updated from Main Street by Joel, 09:12".
- Nothing is deleted by a push unless the owner ticked "also remove items
  not in the source" and saw the list.
- `00-product.md`'s "not for chains" stands. This is for the owner who can
  name their outlets, not for a head office.

## Tasks

- [ ] **Task 1 - ADR-315: the rollup is read from replicas, the push is an
  import.** ADR before code. Amends `00-product.md`'s out-of-scope line for
  multi-venue reporting to "in mez Cloud, over replicas, for an account's
  own venues". Names the push's format (the import bundle of Phase 10, with
  a manifest naming source venue, author and time), the conflict rule
  (a push replaces items matched by name and category, adds the rest,
  removes only with the tick), and what is never pushed (prices in a
  different tax profile without a warning; printers; staff; devices;
  anything in settings).

- [ ] **Task 2 - The venues page and the switch.** *`mez-cloud`.* The
  account area's venues list becomes the owner's front door: each venue's
  today, state and last seen, and a combined row. In the mirrored and live
  back office, the bar gains a venue switcher for accounts with more than
  one venue; a single-venue account sees no change.

- [ ] **Task 3 - The rollup.** *`mez-cloud`.* Over the replicas Phase 15's
  reader already follows: today and this week across outlets; each outlet's
  sales, average ticket and voids against the others; items ranked across
  outlets with their per-outlet split; opening-hour heat across outlets.
  Read through this repo's reporting queries run against each replica, then
  summed in the cloud; no new query is written twice. Money stays `int64`
  minor units until the last render. Tests on two fixture venues with
  known totals.

- [ ] **Task 4 - The accountant's mail.** *`mez-cloud`.* On a day the owner
  picks, one email to an address they set with each venue's monthly
  exports, produced by this repo's export package over the replica, zipped
  by venue. The owner sees what was sent and can send it again.

- [ ] **Task 5 - Push: the cloud's side.** *`mez-cloud`.* From the source
  venue's replica the owner picks what to push (menu, prices, choices,
  stock items) and which venues get it. The cloud builds the bundle,
  shows the diff per target venue by running the import's own preview
  through the mirror, and on confirm: a connected venue receives the bundle
  through the relay to the import API as the owner's cloud session (Phase
  14 Task 5), and a disconnected venue has it queued with a time to live of
  seven days. The owner sees each venue's result: applied, waiting, failed
  with the import's reason.

- [ ] **Task 6 - Push: the venue's side.** *This repo.* On connecting to
  the relay, and every ten minutes while connected, the tunnel agent asks
  the cloud's entitlement implementation for pending bundles for this
  venue; the static implementation has none. A bundle is applied through
  the same path as a spreadsheet import, as the cloud owner staff member,
  in one transaction, with its manifest in the access log (`via=relay
  by=cloud push=<id> from=<venue>`). The back office shows a notice at the
  venue for a day. A bundle that fails the import's validation is reported
  back and not retried. Tests in `internal/tunnel` and the import package.

- [ ] **Task 7 - Pricing and words.** Cloud Plus, or a per-outlet price,
  per `docs/marketing/cloud-unit-economics.md`; the pricing page's third
  column rewritten; `deploy.md`'s cloud section gains "More than one
  venue".

## Done when
- An account with two venues opens one page and sees both and their sum,
  matching each venue's own dashboard to the rupee.
- A price changed at one venue is pushed to the other while it is closed,
  and is there when its PC is switched on in the morning, with the access
  log saying who and whence.
- A push that would remove items shows them and does nothing until the
  tick.
- The accountant's mail for a month arrives with both venues' exports and
  opens in Excel.

# Phase 15 - The owner's WhatsApp

## Goal
Every day at close the owner gets one WhatsApp message: sales, cash in the
drawer, UPI and card totals, voids, discounts, who closed. Between closes,
a message when something an owner would want to know happens: a void or
refund above a line they set, a discount by a cashier, a shift left open,
the venue dark for hours in opening time, a printer or the backup down.
Nothing is sent that the owner did not ask for, and nothing is marketing.

## Depends on
Phase 14 (the replica in the cloud, the venue's connection to it, the
account area). The venue's events are append-only (Phase 03), so a shift
close, a void and a refund are each one row the cloud can watch for.

## Where the code lives
The reader, the rules, the templates and the sending are `mez-cloud`. This
repo adds one thing: a health summary the agent sends over the relay
connection, so the cloud knows about a printer or a backup without reaching
into the venue.

## What stays true
- The venue computes nothing new. The cloud reads the replica it already
  has. A venue with no cloud has no messages and nothing missing.
- Messages are utility, never marketing. The owner chose each kind.
- Costs stay inside `docs/marketing/cloud-unit-economics.md`: about a
  hundred messages a venue a month. A rule that would send more is rate
  limited and says so in the account area.
- The owner's phone number is the account's. Recipients beyond it are
  added by the owner and can be removed by them.

## Tasks

- [x] **Task 1 - ADR-360: what is sent, when, to whom, and never.** ADR
  before code. (Written as ADR-360 on 2026-10-22; ADR-314 was already
  the back office from away's.) The close-of-day message's fields and their order, read from
  the Z-report the venue already produces. Each alert, its trigger, its
  default threshold, and whether it is on by default (close-of-day: on;
  void over ₹500: on; shift open past closing plus two hours: on; venue
  dark for three hours inside opening hours: on; printer or backup failing
  for an hour: on; discount by cashier: off). Quiet hours. The rule that a
  message is sent once per event, and at most twelve alerts a day a venue
  before a single "more at app.mezpos.com" message replaces the rest.

- [ ] **Task 2 - Meta, the number and the templates.** *`mez-cloud`.* A
  WhatsApp Business account on the Cloud API directly, with Gupshup
  self-serve as the fallback if verification stalls. Templates submitted in
  the utility category for: close-of-day, alert (one template, the kind
  and figure as parameters), renewal due, charge failed, suspended,
  reactivated, one-time code. A sending package with the per-template rate,
  idempotency on the event that caused it, and the delivery status written
  back from the webhook.

- [ ] **Task 3 - The reader.** *`mez-cloud`.* For each entitled venue the
  mirror process already follows the replica; this task adds a cursor over
  the events table and a handler per event kind. A `shift_closed` event
  produces the close-of-day message from the Z-report query this repo
  exports. Voids, refunds and discounts go through Task 1's thresholds.
  The cursor is stored per venue so a restart sends nothing twice. Tests
  on a replica built from a fixture venue: one message per close, none on
  a replay.

- [ ] **Task 4 - The venue's health, in one line, over the connection it
  has.** *This repo.* The tunnel agent sends a small JSON summary to the
  relay every five minutes and on change: printers (each one's last
  success), backup state and last sync, the open shift's id and when it
  opened, the venue's opening hours as set. `relay.Entitlements`'s cloud
  implementation receives it; the static one drops it. No new route and
  nothing new reachable from outside. Tests in `internal/tunnel` on the
  summary's contents and cadence. Split on 2026-10-22:
  - [x] **4a - The frame and the sink.** `wire.TypeHealth` and `wire.Health`;
    the relay says on the dial's answer that it reads them, as the venue
    says it reads notices; `relay.HealthSink`, which an `Entitlements`
    may also be, receives one; a relay whose entitlements are not a sink
    does not ask for them and drops any sent. cloudkit hands the sink and
    the summary out. Tests in `internal/relay`.
  - [x] **4b - Opening hours.** ADR-360's per-weekday setting at the venue:
    `internal/domain/hours`, the `opening_hours` key, the Rules API and
    the field on the Shifts card. Tests on the parsing and the check.
  - [ ] **4c - The summary, sent.** `internal/service/health` composes it
    from the printers' outcomes, the backup's status, the open shift and
    the hours; the tunnel sends it on connecting, every five minutes, and
    within a minute of a change, to a relay that asked. Wired in `main`.
    Tests in `internal/tunnel` on cadence and in `internal/service/health`
    on contents.

- [ ] **Task 5 - The clock alerts.** *`mez-cloud`.* From Task 4's
  summaries: shift still open past closing, venue not heard from inside
  opening hours, printer failing, backup failing. Each with its hour of
  patience and one message per episode, with a second when the episode
  ends.

- [ ] **Task 6 - The owner's choices.** *`mez-cloud`.* In the account area,
  per venue: each message kind on or off, its threshold, quiet hours, up to
  two more recipients each confirmed by code. The venue's Devices card in
  this repo shows "Last report: yesterday 23:04" from the `cloud` block,
  read-only, with the link to change it.

- [ ] **Task 7 - Words.** The pricing page's first bullet becomes the
  close-of-day message with a real one as the picture. `deploy.md`'s cloud
  section lists the messages and how to stop each.

## Done when
- A shift closed at the friend's cafe produces the close-of-day WhatsApp
  within two minutes, with figures equal to the printed Z-report.
- A void above the line produces one message; the same event replayed on a
  restart produces none.
- The venue PC switched off at noon produces "not heard from" at three and
  "back" when it is.
- A month of the friend's cafe costs under ₹30 in messages, read from the
  Meta invoice.

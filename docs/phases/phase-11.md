# Phase 11 — Relay and guest ordering

## Goal
A guest on cellular data scans a table QR and orders, and it becomes the same
ticket. Read ADR-008 in `docs/DECISIONS.md` before starting.

## Depends on
Phase 10.

## Tasks

- [x] **Task 1 — The relay service.**
  `cmd/relay`. Stateless. Venue agents connect outbound over WSS with a venue
  key; the relay maps `venue_id → socket`. Guest HTTP requests are framed and
  forwarded down the socket, responses streamed back. Auth, per-venue rate
  limiting, structured logs, health endpoint. Deployable as its own small
  binary or container. It stores nothing.

  - [x] **1b.** A hostname per venue: `RELAY_VENUE_DOMAIN` serves each venue at
    `<venue>.<domain>` and stops serving `/v/<venue>/` (ADR-203).
  - [x] **1c.** Deploy it: the owner's VPS behind Cloudflare and Caddy, at
    `relay.jskin.in` and `*.jskin.in`, with the files to do it again in `build/`.

- [x] **Task 2 — The tunnel client.**
  `internal/tunnel`. Outbound connection with exponential backoff, heartbeat,
  and clean reconnect. Tunnel status surfaced in `/api/v1/health` and visibly in
  back office. **The tunnel being down must not affect any local operation** —
  prove it with a test that kills the relay mid-service and asserts the terminal
  still sells.

  Split in two, since the back office half is a screen of its own:
  - [x] **2a.** The client, `relay_key`, the wiring in `cmd/mez`, the tunnel in
    `/api/v1/health`, and the test that kills the relay mid-service.
  - [x] **2b.** Tunnel status visibly in back office: an endpoint that says more
    than health's one word (state, since when, last error, next retry), and
    where the owner sees it.

- [x] **Task 3 — Guest menu and cart.**
  `web/guest`. Table code resolves the venue and table. Menu, modifiers, cart.
  Under the 60 KB gzipped budget — no shadcn, no heavy date library, no icon
  pack. Optimise for a cold 3G load: inline critical CSS, lazy images, no
  blocking fonts.

  Split in three (ADR-206):
  - [x] **3a.** The way in: a code for each table, the guest API under
    `/order/api/`, and the bundle loading through the relay.
  - [x] **3b.** The landing and the menu (M01, M02), with their specs.
  - [x] **3c.** An item with its choices, and the cart (M03, M04's cart, M11a).

- [ ] **Task 4 — Guest checkout.**
  Prepay-required and pay-at-table modes, per venue. UPI intent link and dynamic
  QR, plus a pluggable gateway adapter (Razorpay/Cashfree/Stripe) behind one
  interface. Payment confirmation emits the same `payment.tendered` event as the
  counter. Bill splitting for guests at the same table.

- [ ] **Task 5 — Order tracking and e-receipt.**
  The guest sees real kitchen events — accepted, firing, ready — not a fake
  timer. Pickup code for web pickup orders. E-receipt with the same invoice
  number as the counter would produce, and one-tap re-order.

- [ ] **Task 6 — QR codes and table codes.**
  Generate per-table QR codes from back office, printable as a sheet. Codes are
  non-guessable and revocable. Rate limit per table to stop a bored teenager
  opening forty tickets.

## Done when
A phone on mobile data, with no access to the venue LAN, scans a table QR,
orders, pays by UPI, and the ticket appears on the kitchen display and settles
into the same invoice series and the same Z-report as a counter sale.

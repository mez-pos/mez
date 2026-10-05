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

  Decided before it starts (ADR-209): the first order from a table in a sitting
  is a request staff accept or decline on the terminal, and nothing reaches the
  kitchen until they do; later rounds go straight through until the bill is
  closed; a table staff have seated, and a prepaid order, need no acceptance.
  The terminal must say a request is waiting, with a sound.

  Split in seven (ADR-210). 4c onward is a first cut, made before reading M08
  and M09: read them at 4c and move the seams if they are in the wrong place.
  - [x] **4a.** A guest's cart becomes an order, on the server.
    `POST /order/api/t/{code}/orders`, priced from the menu, with an
    `Idempotency-Key`. At a table with a ticket open the lines join it and go
    to the kitchen; at one with none the order is kept as a request.
  - [x] **4b.** The request on the terminal. Split in two (ADR-211):
    - [x] **4b1.** The server: `GET /api/v1/guest-requests`, and `accept` and
      `decline` on one. Accept opens the table's ticket if it has none, and
      appends the lines as priced and sends them. A request arriving or
      answered moves the floor version, so it reaches terminals as
      `floor_changed`.
    - [x] **4b2.** The terminal: what is waiting, on every screen, with a
      sound; Accept and Decline; and the table on the floor saying it wants
      to order.
  - [x] **4c.** Place Order on the phone: the cart's foot, the design's words
    back ("Start Ordering", "Add to order"), special instructions, and what
    the guest sees next: waiting, sent or declined, and each refusal.
  - [x] **4d.** The bill on the phone, with its tax as the server works it
    out: `GET /order/api/t/{code}/bill` and the screen (ADR-214). The setting
    for how the venue is paid moved to 4e, where it first means something.
  - [ ] **4e.** Paying from the phone by UPI. Staff confirm a payment, as at
    the till, until 4f's gateway can (ADR-215). Split in five:
    - [x] **4e1.** The venue's UPI address as a setting
      (`GET`/`PUT /api/v1/settings/payments`), and a pay link for what is
      left on the phone's bill.
    - [x] **4e2.** Where the owner sets it: a card under the venue's details
      on the back office's Settings (ADR-216).
    - [x] **4e3.** The QR on the bill, for paying from another phone: drawn
      on the server (`internal/domain/qr`, no dependency), so the bundle
      carries no encoder (ADR-217).
    - [x] **4e4.** "I have paid": the guest tells the terminal, which says so
      as it does a request, and staff record the payment from there. Split
      in two (ADR-218):
      - [x] **4e4a.** The server: `POST /order/api/t/{code}/bill/paid`, the
        notices on `GET /api/v1/guest-requests`, and
        `POST /api/v1/paid-notices/{id}/seen`. A notice is a message, never
        a payment.
      - [x] **4e4b.** The screens: "I have paid" on the phone's bill, and the
        notice on the terminal, leading to that table's payment.
    - [ ] **4e5.** The mode, prepay or pay at the table, and in a prepay
      venue a round held until staff have recorded its payment, then sent
      with no further acceptance (ADR-209). Decide who closes a bill nobody
      on staff touched (a sold stock movement needs a real staff id, see
      KNOWN.md).
  - [ ] **4f.** The gateway adapter: one interface, with Razorpay, Cashfree
    and Stripe behind it.
  - [ ] **4g.** Splitting the bill among the guests at a table.

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

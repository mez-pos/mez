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
    - [x] **4e5.** The mode, prepay or pay at the table. **After 4f**
      (ADR-220): prepay wants a payment that confirms itself, which is a
      gateway's. In a prepay venue a round is held until its charge is paid,
      then sent with no acceptance (ADR-209). Split in four (ADR-225):
      - [x] **4e5a.** The setting, `payment.mode`, and a round held unpaid:
        priced with its tax by the bill's own projection (`order.Adding`),
        no order and nothing for staff. Not yet in effect in `cmd/mez`.
      - [x] **4e5b.** A charge for an unpaid round, and the round becoming
        lines on a ticket, sent, and paid for, when the gateway says paid.
        Split in two (ADR-226):
        - [x] **4e5b1.** In the services: `charges.StartRound`, and a paid
          round's charge sending the round (`SendPaidRound`) and then
          tendering. `gateway_charges` rebuilt so a charge can be for a round.
        - [x] **4e5b2.** `POST /order/api/t/{code}/orders/{round}/charge`
          (ADR-227); its status is the bill charge's route. `CanCharge` wired
          in `cmd/mez`: the mode is in effect where a gateway is set.
      - [x] **4e5c.** The phone: Place Order, pay, and back to an order sent
        (ADR-228). Seen at 390 wide in every state but a real payment.
      - [x] **4e5d.** The owner's choice of mode in the back office, beside
        the UPI address, saying when it is in effect (ADR-229).
  - [ ] **4f.** The gateway adapter: one interface, with Razorpay, Cashfree
    and Stripe behind it. Split in six (ADR-220):
    - [x] **4f1.** The interface (`internal/gateway`), and the service that
      starts a charge and turns a gateway's "paid" into one
      `payment.tendered` (`internal/service/charges`), against a fake
      gateway. No provider, no API, nothing wired.
    - [x] **4f2.** The first real adapter, Razorpay
      (`internal/gateway/razorpay`); its keys in the configuration and the
      environment, not the database; and a sweeper that settles open charges
      every 20 seconds (ADR-221).
    - [x] **4f3.** The guest's API and the phone: pay online from the bill,
      the gateway's page, and coming back (ADR-222). `public_url` is where a
      gateway sends a guest back.
    - [x] **4f4.** Unapplied charges on the terminal: money a gateway took
      that no bill could, for a person to refund or record, and to say which
      (ADR-223).
    - [x] **4f5.** The gateway's webhook, through the relay, so a payment
      lands without waiting for the sweep: `POST /order/api/hook/{gateway}`,
      a hint to ask the gateway now and never proof (ADR-224).
    - [ ] **4f6.** Cashfree and Stripe, and the owner choosing a gateway in
      the back office. Split in three (ADR-230):
      - [x] **4f6a.** Stripe (`internal/gateway/stripe`): Checkout Sessions,
        its webhook, and `gateway = stripe` in the configuration.
      - [ ] **4f6b.** Cashfree. **Waits on the owner**: its payment links
        need the payer's phone number, and mez asks a guest for nothing.
        See STATE.md's open questions.
      - [x] **4f6c.** The back office saying which gateway is set and whether
        it has taken a payment (ADR-231). Not choosing one: the keys are
        configuration and not the database (ADR-221).
  - [x] **4g.** Splitting the bill among the guests at a table. A share is an
    amount of what is owed, not lines. Split in four (ADR-232):
    - [x] **4g1.** In the services: `charges.StartShare`, a charge for part
      of the bill for one phone; an open page is a claim, and no charge is
      given for more than is unclaimed (`Claims`, `EqualPart`). Migration
      00045. No route and no screen.
    - [x] **4g2.** The guest's API (ADR-233): `split` on the bill (the rest,
      what this phone can take, an equal part for 2 to 20 people), a share
      as the body of `POST /order/api/t/{code}/bill/charge`, the phone's
      name for itself, and `bill_claimed` and `bill_moved`.
    - [x] **4g2b.** Giving up a page (ADR-234): `charges.LetGo` and
      `POST /order/api/t/{code}/bill/charge/{id}/let-go`. The charge stays
      open and payable and is no claim on the bill. Migration 00046.
    - [x] **4g3.** The phone (ADR-235): Split the bill in the Pay online
      card, the rest, evenly among so many, or a figure typed; "I am not
      paying this" on a waiting payment; and the spec.

- [ ] **Task 5 — Order tracking and e-receipt.**
  The guest sees real kitchen events — accepted, firing, ready — not a fake
  timer. Pickup code for web pickup orders. E-receipt with the same invoice
  number as the counter would produce, and one-tap re-order.

  Split in six (ADR-236):
  - [x] **5a.** A round knows its lines on the ticket (`first_seq`, migration
    00047), `order.State.Progress` reads how far lines have come off their
    tickets, and `TrackRound` answers a round with it. No route, no screen.
  - [ ] **5b.** The guest's API: `GET /order/api/t/{code}/orders/{round}`,
    a round as it stands with its progress.
  - [ ] **5c.** The phone: the order's screen says accepted, with the
    kitchen, being made and ready, as they happen, and its spec.
  - [ ] **5d.** The e-receipt on the server: what a guest may read of a bill
    once it is settled, with its invoice number, and by what they ask.
  - [ ] **5e.** The receipt on the phone, and ordering the same again.
  - [ ] **5f.** Ordering for pickup, and its code (ADR-237, by the owner).
    After 5e, and beside Task 6, which makes and revokes the venue's code.
    Paid first, always, so only where a gateway is set. Split in four:
    - [ ] **5f1.** In the services: a venue's pickup code, a pickup round
      with no table, held unpaid, and paid, a ticket of its own on the
      `web` channel with a pickup code of three or four characters.
    - [ ] **5f2.** The guest's API: the menu and an order by the venue's
      code, paying for it, and tracking it with its code.
    - [ ] **5f3.** The phone: the way in with no table, pay, and the
      order's screen with the code in large type.
    - [ ] **5f4.** The counter: the pickup orders out and ready on the
      terminal, by code, to call out and hand over.

- [ ] **Task 6 — QR codes and table codes.**
  Generate per-table QR codes from back office, printable as a sheet. Codes are
  non-guessable and revocable. Rate limit per table to stop a bored teenager
  opening forty tickets.

## Done when
A phone on mobile data, with no access to the venue LAN, scans a table QR,
orders, pays by UPI, and the ticket appears on the kitchen display and settles
into the same invoice series and the same Z-report as a counter sale.

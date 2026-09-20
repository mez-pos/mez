# Phase 04 — API and realtime

## Goal
The HTTP boundary, the WebSocket hub, device and staff authentication, and
generated TypeScript types. After this phase a frontend can be built against a
real, typed API.

## Depends on
Phase 03.

## Tasks

- [x] **Task 1 — Order persistence.**
  `internal/db` queries for appending events and reading a stream. Appending is
  transactional with a `(order_id, seq)` unique constraint and a retry on
  conflict. Snapshotting every N events, with rebuild-from-events proven
  equivalent in a test. Invoice series allocation implemented against a counter
  row inside the issuing transaction — never `MAX(n)+1`. A concurrency test:
  many goroutines allocating invoice numbers produce a gapless, duplicate-free
  sequence.

- [x] **Task 2 — Order service.**
  `internal/service/order`. Load stream → project → validate the command →
  append event(s) → publish to the hub. One method per command. Handlers will
  call this; handlers contain no logic. Idempotency: a command carrying a key
  already applied returns the prior result instead of re-applying.

- [x] **Task 3 — Auth.**
  Device pairing: a back-office-generated code, redeemed once for a long-lived
  device token. Staff PIN sign-in on a paired device, Argon2id hashed, rate
  limited, lockout after repeated failures, all attempts logged. Roles and
  permissions with a middleware that checks them. Permission denial is a typed
  API error, not a 500.

  Split into three sessions. Each has its own schema, its own HTTP surface
  and its own adversarial tests, and it is security code: one session each.

  - [x] **Task 3a - Device pairing and device tokens.**
    `internal/auth`: a random, high-entropy device token (only its SHA-256
    stored in `devices.token_hash`; a token is long enough that a fast hash
    is right, unlike a PIN). Migration: `pairing_codes` (venue, device name
    and kind, code hash, expires_at, redeemed_at, created_by). Create a code
    (short, readable, Crockford alphabet from `internal/domain/id`, expires in
    minutes) and redeem it exactly once, in one transaction, for a device row
    plus token. Redeeming an expired, used or unknown code fails the same way.
    Revoking a device (`revoked_at`) kills its token. `internal/api`
    middleware: `Authorization: Bearer <device token>` on every `/api/v1`
    route except health and redeem; puts venue and device in the context;
    `last_seen_at` updated no more than once a minute. Endpoints:
    `POST /api/v1/devices/pair` (redeem). Creating a code is a back-office
    action and waits for 3c's permission check: until then it is a Go
    function with tests, not an endpoint. Typed errors `unauthenticated`
    (401) and `device_revoked` (401).

  - [x] **Task 3b - Staff PIN sign-in.**
    Add `golang.org/x/crypto` (allowed set; ADR required; v0.28.0 is already
    in the module cache). PINs as Argon2id PHC strings with parameters
    stored in the hash, constant-time compare, a dummy hash compared when the
    staff member is unknown so timing does not reveal who exists. Migration:
    `staff_sessions` (token hash, staff, device, expires_at, ended_at) and
    `sign_in_attempts` (venue, device, staff or null, at, outcome, reason),
    every attempt written, success or not. Lockout: N failures within a
    window locks the staff member for a period, and a per-device limit stops
    one tablet cycling through everyone. Both configurable with defaults in
    code. Sign-in only on a paired device (3a's middleware). Endpoints:
    `POST /api/v1/sessions` (sign in), `DELETE /api/v1/sessions/current`.
    Middleware puts the staff member in the context. Typed errors
    `invalid_pin` (401), `locked_out` (429 with Retry-After).
    Also, from 3a's gaps: a per-IP rate limit on `POST /api/v1/devices/pair`
    (429 `rate_limited` with Retry-After), and `venue_id`, `device_id` and
    `staff_id` on the access log line of every authenticated request.

  - [x] **Task 3c - Roles and permissions.**
    The permission keys as a Go enum, exhaustive, with the roles' JSON arrays
    validated against it. `requirePermission(p)` middleware returning a typed
    `forbidden` (403) error, never a 500. Manager override: commands that
    carry `AuthorisedBy` (void, comp, reopen, payment void, discount) check
    that staff member's permission and PIN, not the caller's. Wire the
    authenticated venue, staff and device into `service/order.Meta`. Expose
    3a's code creation as a back-office endpoint behind its permission.

  - [x] **Task 3d - First-run setup.**
    A fresh install cannot pair its first device through the API: making a
    code needs a permission, a permission needs a signed-in staff member,
    and signing in needs a paired device. `nofiltr setup`, run on the venue
    machine itself (local access is the trust root), creates the venue, its
    default roles, the owner with a PIN, and prints the first pairing code.
    Refuses to run twice on a database that already has a venue. Tested as
    a command against a temp data directory. This is what the phase's
    done-when script starts from.

- [x] **Task 4 - HTTP handlers.**
  `/api/v1` for orders, lines, tickets and payments: everything
  `internal/service/order` can already do. Decode, validate, call the
  service, encode, per `docs/03-conventions.md`. The stable error shape, with
  every domain error mapped to a code and a status, never a 500.
  `Idempotency-Key` honoured on every mutating endpoint. Orders are addressed
  by public ID. Manager overrides on void, comp, discount, payment void and
  reopen, through 3c's `Override`. `httptest` coverage of the happy path and
  every error shape.

  Tables, menu read and shifts were listed here, and have moved: none has a
  schema or a service yet. Tables go with phase 08, the menu with phase 10,
  shifts with phase 09, each alongside the domain they serve.

- [x] **Task 5 — WebSocket hub.**
  `internal/hub`. One connection per device, authenticated with the device
  token. Client sends its last-seen `seq` on connect; the server replays the gap
  from SQLite, then streams live. Subscriptions by topic (`station:bar`,
  `table:12`, `venue`). Heartbeat, backpressure handling, clean disconnect. A
  test that kills a connection mid-stream and verifies nothing is lost on
  reconnect.

- [ ] **Task 6 — TypeScript codegen.**
  Generate TS types from the API request/response structs into
  `web/shared/api-types.ts`. Wire into `make generate` and into CI so a drift
  between Go and TS fails the build. Document the loop in `docs/03-conventions.md`.

## Done when
A shell script can open an order, add lines, take a split payment, issue an
invoice and close it — entirely over HTTP — and a WebSocket client sees every
event. Types generate cleanly.

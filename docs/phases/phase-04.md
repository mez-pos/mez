# Phase 04 — API and realtime

## Goal
The HTTP boundary, the WebSocket hub, device and staff authentication, and
generated TypeScript types. After this phase a frontend can be built against a
real, typed API.

## Depends on
Phase 03.

## Tasks

- [ ] **Task 1 — Order persistence.**
  `internal/db` queries for appending events and reading a stream. Appending is
  transactional with a `(order_id, seq)` unique constraint and a retry on
  conflict. Snapshotting every N events, with rebuild-from-events proven
  equivalent in a test. Invoice series allocation implemented against a counter
  row inside the issuing transaction — never `MAX(n)+1`. A concurrency test:
  many goroutines allocating invoice numbers produce a gapless, duplicate-free
  sequence.

- [ ] **Task 2 — Order service.**
  `internal/service/order`. Load stream → project → validate the command →
  append event(s) → publish to the hub. One method per command. Handlers will
  call this; handlers contain no logic. Idempotency: a command carrying a key
  already applied returns the prior result instead of re-applying.

- [ ] **Task 3 — Auth.**
  Device pairing: a back-office-generated code, redeemed once for a long-lived
  device token. Staff PIN sign-in on a paired device, Argon2id hashed, rate
  limited, lockout after repeated failures, all attempts logged. Roles and
  permissions with a middleware that checks them. Permission denial is a typed
  API error, not a 500.

- [ ] **Task 4 — HTTP handlers.**
  `/api/v1` for orders, tickets, tables, menu read, payments, shifts. Decode →
  validate → service → encode, per `docs/03-conventions.md`. The stable error
  shape. `Idempotency-Key` honoured on every mutating endpoint. `httptest`
  coverage of the happy path and every error shape.

- [ ] **Task 5 — WebSocket hub.**
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

# Phase 08 — Floor and kitchen

## Goal
Dine-in works end to end: seat a party, route courses to the right stations, and
run the pass from a kitchen display on a second device.

## Depends on
Phase 07.

## Tasks

- [x] **Task 1 — Floor plan data and editor.**
  Migration for `areas`, `tables` (with position, seats, shape). A back-office
  editor to place tables. Keep it simple — a grid with drag, and a numeric
  fallback for position, because drag must never be the only path.

- [x] **Task 2 — Floor plan screen.**
  Spec first. Table tiles showing state, covers, elapsed time and balance.
  State colours follow the status rules — colour **and** a word on every tile.
  Seat a party, open a ticket for a table, transfer a ticket, merge and split
  tables.

  Too big for one session: split into 2a, 2b and 2c. Where a table is in its
  turn is derived, not stored, except for one fact orders cannot give:
  that it has been cleared since its last order closed.

  - [x] **2a: Table state, and the floor read for terminals.** Migration:
    `tables.cleared_at`. A pure function in `internal/domain/table` from a
    table's open and last closed order to its tile: `free`, `seated` (an
    order open with no active lines), `ordered`, `settling`, `dirty` (its
    last order closed after `cleared_at`), checked against the table
    machine. `GET /api/v1/floor/live` (`order.open`): the plan in service
    with each tile's state, covers, opened at, balance and order id. A
    stream notice when a tile changes. `POST .../tables/{id}/clear`
    (dirty to free). ADR. Tests: every state, and each transition's edge.
  - [x] **2b: The floor screen.** Spec first (`terminal-floor.md`). The
    terminal's Floor view: area tabs, tiles with colour **and** a word,
    covers, elapsed time, balance. Free: seat a party (covers, then an
    order opened at the table). Seated or later: opens the ticket on the
    register. Dirty: Clear. Seated with nothing ordered: Guests left
    (seated to free). Live across terminals from 2a's notice.
  - [x] **2c: Transfer, join and split tables.** Order events
    `order.table_changed` (a ticket moves to a free table) and
    `order.tables_joined` / `order.tables_parted` (a party spreads over
    more tables: one order, several tables, all showing it). Projection,
    service commands, API, and the floor screen's Move and Join. A table
    joined to an order is not free. Codegen lines for the new events.

- [x] **Task 3 — Station routing and courses.**
  Items carry a station. Lines carry a course. Firing a course sends only that
  course's lines, and only to the relevant stations. The bar and the kitchen
  each get only their own lines. Emits `ticket.sent`.

- [ ] **Task 4 — Kitchen display.**
  A separate route, designed for a wall-mounted tablet read at three metres.
  Large type. Ticket cards with elapsed timers that escalate through gold to
  vermilion. Bump to ready, recall, all-day counts per item. Touch targets
  bigger than the 44px minimum — this screen is used with wet hands.

  Too big for one session: split into 4a and 4b. A station ticket's life is
  told by order events, so a display that drops off catches up from the
  stream like any other device (Task 5).

  - [ ] **4a: Station tickets, bump and recall, and the kitchen's API.**
    Order events `ticket.bumped` and `ticket.recalled` `{ticket_seq,
    station}`: a sent ticket is in the kitchen until bumped to ready, and
    recalled back if it was bumped by mistake; checked against
    `internal/domain/kitchen`'s machine. Projection: each ticket's status
    and ready time. The hub files both under their station's topic.
    `GET /api/v1/kitchen/tickets?station=` (the station's tickets not yet
    collected, with lines, table and times) and `POST .../bump`, `.../recall`
    for a device paired as `kitchen`, with no staff sign-in: the actor is
    the device. Codegen, tests, ADR.
  - [ ] **4b: The kitchen display.** Spec first (`kitchen-display.md`).
    A `kitchen` app kind: pair as a kitchen device, choose the station, no
    PIN. Cards in large type, timers escalating through gold to vermilion,
    Bump (large), Recall the last bumped, all-day counts per item. Live
    from the station topic. The terminal's ticket shows a line Ready once
    its ticket is bumped.

- [ ] **Task 5 — Realtime across devices.**
  Verify the whole loop: fire on the terminal, appears on the KDS within a
  second; bump on the KDS, terminal reflects it. Unplug the KDS mid-service and
  reconnect — it must catch up from its last `seq` with nothing lost and nothing
  duplicated. Write this as an automated test, not a manual check.

## Done when
Two devices run a dine-in service together: seated, ordered, two courses fired
to two stations, bumped, settled, table flipped to dirty then free — with a
network interruption survived in the middle.

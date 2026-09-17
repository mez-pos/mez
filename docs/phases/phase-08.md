# Phase 08 — Floor and kitchen

## Goal
Dine-in works end to end: seat a party, route courses to the right stations, and
run the pass from a kitchen display on a second device.

## Depends on
Phase 07.

## Tasks

- [ ] **Task 1 — Floor plan data and editor.**
  Migration for `areas`, `tables` (with position, seats, shape). A back-office
  editor to place tables. Keep it simple — a grid with drag, and a numeric
  fallback for position, because drag must never be the only path.

- [ ] **Task 2 — Floor plan screen.**
  Spec first. Table tiles showing state, covers, elapsed time and balance.
  State colours follow the status rules — colour **and** a word on every tile.
  Seat a party, open a ticket for a table, transfer a ticket, merge and split
  tables.

- [ ] **Task 3 — Station routing and courses.**
  Items carry a station. Lines carry a course. Firing a course sends only that
  course's lines, and only to the relevant stations. The bar and the kitchen
  each get only their own lines. Emits `ticket.sent`.

- [ ] **Task 4 — Kitchen display.**
  A separate route, designed for a wall-mounted tablet read at three metres.
  Large type. Ticket cards with elapsed timers that escalate through gold to
  vermilion. Bump to ready, recall, all-day counts per item. Touch targets
  bigger than the 44px minimum — this screen is used with wet hands.

- [ ] **Task 5 — Realtime across devices.**
  Verify the whole loop: fire on the terminal, appears on the KDS within a
  second; bump on the KDS, terminal reflects it. Unplug the KDS mid-service and
  reconnect — it must catch up from its last `seq` with nothing lost and nothing
  duplicated. Write this as an automated test, not a manual check.

## Done when
Two devices run a dine-in service together: seated, ordered, two courses fired
to two stations, bumped, settled, table flipped to dirty then free — with a
network interruption survived in the middle.

-- Orders, their events and their snapshots. internal/db/orders.go is the only
-- caller: it keeps the orders row, the events and the snapshot in step inside
-- one transaction, which a handler calling these directly would not.

-- name: CreateOrder :one
INSERT INTO orders (
    public_id,
    venue_id,
    channel,
    table_id,
    status,
    last_seq,
    opened_at,
    closed_at,
    updated_at,
    shift_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: OrderByID :one
SELECT
    id, public_id, venue_id, channel, table_id, status, last_seq,
    opened_at, closed_at, updated_at
FROM orders
WHERE id = ?;

-- name: UpdateOrderIndex :execrows
-- Brings the index columns up to the projection just appended. The guard on
-- the previous last_seq makes a write from a stale read touch no row, which
-- the caller reports as a conflict rather than moving the index backwards.
UPDATE orders
SET table_id   = sqlc.arg(table_id),
    status     = sqlc.arg(status),
    last_seq   = sqlc.arg(last_seq),
    closed_at  = sqlc.arg(closed_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND last_seq = sqlc.arg(prev_seq);

-- name: AppendOrderEvent :exec
INSERT INTO order_events (order_id, seq, type, payload, actor_id, device_id, at, shift_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: OrderEventsAfter :many
SELECT id, order_id, seq, type, payload, actor_id, device_id, at
FROM order_events
WHERE order_id = ? AND seq > ?
ORDER BY seq;

-- name: OrderSnapshot :one
SELECT order_id, seq, basis, state
FROM order_snapshots
WHERE order_id = ?;

-- name: SaveOrderSnapshot :exec
INSERT INTO order_snapshots (order_id, seq, basis, state)
VALUES (?, ?, ?, ?)
ON CONFLICT (order_id) DO UPDATE
SET seq = excluded.seq, basis = excluded.basis, state = excluded.state;

-- name: OrderByPublicID :one
SELECT
    id, public_id, venue_id, channel, table_id, status, last_seq,
    opened_at, closed_at, updated_at
FROM orders
WHERE public_id = ?;

-- name: VenueEventsAfter :many
-- The venue's events after a point in its stream, oldest first. This is what
-- the hub reads, for a replay and for the live stream alike: order_events.id
-- only grows, so a client's last-seen id is a position in one sequence.
SELECT
    e.id, e.order_id, e.seq, e.type, e.payload, e.actor_id, e.device_id, e.at,
    o.public_id AS order_public_id, o.table_id, o.status AS order_status
FROM order_events e
JOIN orders o ON o.id = e.order_id
WHERE o.venue_id = ? AND e.id > ?
ORDER BY e.id
LIMIT ?;

-- name: VenueLastEventID :one
-- The cast is what gives sqlc an int64 rather than an any: coalesce over an
-- aggregate has no column to take a type from.
SELECT CAST(coalesce(max(e.id), 0) AS INTEGER) AS last_id
FROM order_events e
JOIN orders o ON o.id = e.order_id
WHERE o.venue_id = ?;

-- name: UnclosedOrdersForVenue :many
--
-- The tickets a till can switch between: everything not closed. It is a
-- working set, tens of orders, not history, so it is capped rather than
-- paginated (ADR-074). Newest first, so that what the cap drops is the
-- oldest tabs nobody closed and never the table being served; the service
-- turns them oldest first for the screens.
SELECT id, public_id
FROM orders
WHERE venue_id = ? AND status != 'closed'
ORDER BY opened_at DESC, id DESC
LIMIT ?;

-- name: ClosedOrdersForVenue :many
-- The till's past bills: the latest closed, newest first (Task 6b).
SELECT id, public_id
FROM orders
WHERE venue_id = ? AND status = 'closed'
ORDER BY closed_at DESC, id DESC
LIMIT ?;

-- name: OrdersByDocument :many
-- The orders an invoice or credit note number belongs to, as typed or
-- scanned from a receipt, or whose invoice has the sequence number seq in
-- any series or year ("42" for INV/26-27/000042). Newest first.
SELECT o.id, o.public_id
FROM orders o
WHERE o.venue_id = sqlc.arg(venue_id) AND o.id IN (
    SELECT i.order_id FROM invoices i
    WHERE i.venue_id = sqlc.arg(venue_id) AND (i.invoice_no = sqlc.arg(number) OR i.seq = sqlc.arg(seq))
    UNION
    SELECT c.order_id FROM credit_notes c
    WHERE c.venue_id = sqlc.arg(venue_id) AND c.credit_no = sqlc.arg(number)
)
ORDER BY o.id DESC
LIMIT 20;

-- name: DeleteOrderTables :exec
-- The order's rows in order_tables, before they are written again from its
-- projection (ADR-088).
DELETE FROM order_tables WHERE order_id = ?;

-- name: AddOrderTable :exec
INSERT INTO order_tables (order_id, table_id, closed_at) VALUES (?, ?, ?);

-- name: UpsertStationTicket :exec
-- A ticket as its order's projection now has it (ADR-090).
INSERT INTO station_tickets (order_id, ticket_seq, venue_id, station, status, sent_at, ready_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (order_id, ticket_seq) DO UPDATE SET status = excluded.status, ready_at = excluded.ready_at;

-- name: StationBoard :many
-- A station's tickets still in the kitchen, oldest first, and those bumped
-- since a time, for the display's recall.
SELECT st.order_id, st.ticket_seq, st.status, st.sent_at, st.ready_at, o.public_id
FROM station_tickets st
JOIN orders o ON o.id = st.order_id
WHERE st.venue_id = sqlc.arg(venue_id) AND st.station = sqlc.arg(station)
  AND (st.status = 'fired' OR (st.status = 'ready' AND st.ready_at >= sqlc.arg(ready_since)))
ORDER BY st.sent_at, st.order_id, st.ticket_seq
LIMIT sqlc.arg(max_tickets);

-- name: StationDay :many
-- Every ticket a station was sent since a time, in the order they were sent:
-- what its day's figures are added up from (ADR-178). A busy station is a
-- few hundred a day, so it is capped.
SELECT st.order_id, st.ticket_seq, st.status, st.sent_at, st.ready_at
FROM station_tickets st
WHERE st.venue_id = sqlc.arg(venue_id) AND st.station = sqlc.arg(station)
  AND st.sent_at >= sqlc.arg(since)
ORDER BY st.sent_at, st.order_id, st.ticket_seq
LIMIT sqlc.arg(max_tickets);

-- name: OrdersOfDay :many
-- The till's orders list (rebuild Task 10a): every order opened or closed
-- since the trading day began, and any still open from before it, newest
-- first. A day at a busy counter is hundreds of orders, so it is capped.
SELECT id, public_id
FROM orders
WHERE venue_id = sqlc.arg(venue_id)
  AND (status != 'closed' OR opened_at >= sqlc.arg(since) OR closed_at >= sqlc.arg(since))
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max);

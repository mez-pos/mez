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
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
INSERT INTO order_events (order_id, seq, type, payload, actor_id, device_id, at)
VALUES (?, ?, ?, ?, ?, ?, ?);

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

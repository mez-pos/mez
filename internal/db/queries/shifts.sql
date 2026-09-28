-- Shifts, their drawer counts and their events (phase 09, Task 1a). Written
-- only by internal/service/shift, each change in one transaction.

-- name: CreateShift :one
INSERT INTO shifts (public_id, venue_id, status, trading_day, opened_by, opened_at)
VALUES (?, ?, 'open', ?, ?, ?)
RETURNING id;

-- name: OpenShiftForVenue :one
SELECT id, public_id, venue_id, status, trading_day, opened_by, opened_at, closed_by, closed_at
FROM shifts
WHERE venue_id = ? AND status = 'open';

-- name: ShiftByID :one
SELECT id, public_id, venue_id, status, trading_day, opened_by, opened_at, closed_by, closed_at
FROM shifts
WHERE id = ?;

-- name: CloseShift :execrows
UPDATE shifts
SET status = 'closed', closed_by = sqlc.arg(closed_by), closed_at = sqlc.arg(closed_at)
WHERE id = sqlc.arg(id) AND status = 'open';

-- name: CreateDrawerCount :exec
INSERT INTO drawer_counts (shift_id, kind, denominations, total, counted_by, counted_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: DrawerCountsForShift :many
SELECT kind, denominations, total, counted_by, counted_at
FROM drawer_counts
WHERE shift_id = ?
ORDER BY id;

-- name: AppendShiftEvent :one
INSERT INTO shift_events (shift_id, type, amount, reason, actor_id, authorised_by, device_id, idempotency_key, at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ShiftEvents :many
SELECT id, type, amount, reason, actor_id, authorised_by, device_id, at
FROM shift_events
WHERE shift_id = ?
ORDER BY id;

-- name: ShiftEventByKey :one
SELECT id, type, amount, reason, actor_id, authorised_by, device_id, at
FROM shift_events
WHERE shift_id = ? AND idempotency_key = ? AND idempotency_key != '';

-- name: OpenShiftIDForVenue :one
-- The shift an order event is recorded in, read inside the appending
-- transaction so a close cannot slip between the read and the write.
SELECT id FROM shifts WHERE venue_id = ? AND status = 'open';

-- name: ShiftOfOrderEvents :many
-- Which shift each of an order's events was recorded in, by seq.
SELECT seq, shift_id FROM order_events WHERE order_id = ? ORDER BY seq;

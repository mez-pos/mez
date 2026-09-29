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
INSERT INTO drawer_counts (shift_id, kind, denominations, total, counted_by, counted_at, expected, variance, threshold)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DrawerCountsForShift :many
SELECT kind, denominations, total, counted_by, counted_at, expected, variance, threshold
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
-- transaction so a close cannot slip between the read and the write. A
-- shift whose drawer is counted takes nothing more (ADR-095).
SELECT id FROM shifts
WHERE venue_id = ? AND status = 'open'
  AND NOT EXISTS (SELECT 1 FROM drawer_counts WHERE drawer_counts.shift_id = shifts.id AND kind = 'closing');

-- name: ShiftOfOrderEvents :many
-- Which shift each of an order's events was recorded in, by seq.
SELECT seq, shift_id FROM order_events WHERE order_id = ? ORDER BY seq;

-- name: OrderEventsTouchingShift :many
-- The whole streams of the orders with any event in a shift: a refund in
-- it may be against a tender taken in an earlier one.
SELECT order_id, seq, type, payload, actor_id, device_id, at, shift_id
FROM order_events
WHERE order_id IN (SELECT DISTINCT e.order_id FROM order_events e WHERE e.shift_id = sqlc.arg(shift_id))
ORDER BY order_id, seq;

-- name: CreateVarianceReason :exec
INSERT INTO variance_reasons (shift_id, reason, explained_by, explained_at)
VALUES (?, ?, ?, ?);

-- name: VarianceReason :one
SELECT shift_id, reason, explained_by, explained_at
FROM variance_reasons
WHERE shift_id = ?;

-- name: CreateTipPool :exec
INSERT INTO tip_pools (shift_id, rule, total, body, pooled_by, pooled_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: TipPool :one
SELECT shift_id, rule, total, body, pooled_by, pooled_at
FROM tip_pools
WHERE shift_id = ?;

-- name: StaffRolesForVenue :many
-- Everyone who has worked at the venue with their role's name: a tip pool
-- weighs each by their role.
SELECT s.id, s.public_id, s.name, r.name AS role_name
FROM staff s
JOIN roles r ON r.id = s.role_id
WHERE s.venue_id = ?
ORDER BY s.id;

-- name: CreateExport :exec
INSERT INTO exports (shift_id, kind, filename, body, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ExportsForShift :many
SELECT id, kind, filename, length(body) AS size, created_at
FROM exports
WHERE shift_id = ?
ORDER BY id;

-- name: ExportByID :one
SELECT e.id, e.shift_id, e.kind, e.filename, e.body, e.created_at, s.venue_id
FROM exports e
JOIN shifts s ON s.id = e.shift_id
WHERE e.id = ?;

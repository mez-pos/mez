-- A guest's rounds (ADR-210). Written only inside the transaction that
-- decides where a round goes; see internal/service/order.

-- name: CreateGuestRound :one
INSERT INTO guest_rounds (public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
                          idempotency_key, fingerprint, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GuestRoundByKey :one
-- The round a key was first sent with, as it stands now.
SELECT id, public_id, table_id, status, order_id, lines, subtotal_minor, currency, fingerprint
FROM guest_rounds
WHERE venue_id = ? AND idempotency_key = ?;

-- name: CountWaitingGuestRounds :one
-- The requests waiting at a table since the shift opened. One left over
-- from a shift before has lapsed, and does not count.
SELECT count(*) FROM guest_rounds
WHERE table_id = ? AND status = 'waiting' AND created_at >= sqlc.arg(since);

-- name: WaitingGuestRounds :many
-- The requests staff have still to answer, oldest first, with each one's
-- table by name.
SELECT r.id, r.public_id, r.table_id, t.name AS table_name, r.lines, r.subtotal_minor, r.currency, r.created_at
FROM guest_rounds r
JOIN tables t ON t.id = r.table_id
WHERE r.venue_id = ? AND r.status = 'waiting' AND r.created_at >= sqlc.arg(since)
ORDER BY r.created_at, r.id;

-- name: GuestRoundByPublicID :one
SELECT r.id, r.public_id, r.venue_id, r.table_id, r.status, r.order_id, r.lines, r.subtotal_minor, r.currency, r.created_at,
       CAST(coalesce(o.public_id, '') AS TEXT) AS order_public_id
FROM guest_rounds r
LEFT JOIN orders o ON o.id = r.order_id
WHERE r.public_id = ?;

-- name: DecideGuestRound :execrows
-- Answers a request: sent, onto a ticket, or declined. Only a waiting one
-- can be answered, so of two tills answering at once one writes nothing.
UPDATE guest_rounds
SET status = sqlc.arg(status), order_id = sqlc.arg(order_id), decided_at = sqlc.arg(decided_at), decided_by = sqlc.arg(decided_by)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id) AND status = 'waiting';

-- name: UnclosedOrderAtTable :one
-- The ticket open at a table, its own or one it is joined to, the oldest if
-- there is more than one: where a guest's round goes.
SELECT o.id
FROM order_tables ot
JOIN orders o ON o.id = ot.order_id
WHERE ot.table_id = ? AND ot.closed_at IS NULL AND o.venue_id = ?
ORDER BY o.opened_at, o.id
LIMIT 1;

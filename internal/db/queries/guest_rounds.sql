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
SELECT count(*) FROM guest_rounds WHERE table_id = ? AND status = 'waiting';

-- name: UnclosedOrderAtTable :one
-- The ticket open at a table, its own or one it is joined to, the oldest if
-- there is more than one: where a guest's round goes.
SELECT o.id
FROM order_tables ot
JOIN orders o ON o.id = ot.order_id
WHERE ot.table_id = ? AND ot.closed_at IS NULL AND o.venue_id = ?
ORDER BY o.opened_at, o.id
LIMIT 1;

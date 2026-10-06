-- A guest's rounds (ADR-210). Written only inside the transaction that
-- decides where a round goes; see internal/service/order.

-- name: CreateGuestRound :one
-- table_id is 0 for an order to collect, which has no table (ADR-248): it is
-- stored as null, and read back as 0 by every query here.
INSERT INTO guest_rounds (public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
                          idempotency_key, fingerprint, created_at, prepay, due_minor, first_seq)
VALUES (sqlc.arg(public_id), sqlc.arg(venue_id), nullif(CAST(sqlc.arg(table_id) AS INTEGER), 0), sqlc.arg(status),
        sqlc.arg(order_id), sqlc.arg(lines), sqlc.arg(subtotal_minor), sqlc.arg(currency),
        sqlc.arg(idempotency_key), sqlc.arg(fingerprint), sqlc.arg(created_at), sqlc.arg(prepay), sqlc.arg(due_minor),
        sqlc.arg(first_seq))
RETURNING id;

-- name: GuestRoundByKey :one
-- The round a key was first sent with, as it stands now.
SELECT id, public_id, CAST(coalesce(table_id, 0) AS INTEGER) AS table_id, status, order_id, lines, subtotal_minor, currency,
       fingerprint, prepay, due_minor
FROM guest_rounds
WHERE venue_id = ? AND idempotency_key = ?;

-- name: CountWaitingGuestRounds :one
-- The requests waiting at a table since the shift opened. One left over
-- from a shift before has lapsed, and does not count. A round waiting to be
-- paid is not a request: staff have nothing to answer.
SELECT count(*) FROM guest_rounds
WHERE table_id = CAST(sqlc.arg(table_id) AS INTEGER) AND status = 'waiting' AND prepay = 0 AND created_at >= sqlc.arg(since);

-- name: CountUnpaidGuestRounds :one
-- The rounds at a table waiting to be paid, placed since a moment ago. One
-- nobody paid in that time has been walked away from, and does not count.
SELECT count(*) FROM guest_rounds
WHERE table_id = CAST(sqlc.arg(table_id) AS INTEGER) AND status = 'waiting' AND prepay = 1 AND created_at >= sqlc.arg(since);

-- name: WaitingGuestRounds :many
-- The requests staff have still to answer, oldest first, with each one's
-- table by name.
SELECT r.id, r.public_id, t.id AS table_id, t.name AS table_name, r.lines, r.subtotal_minor, r.currency, r.created_at
FROM guest_rounds r
JOIN tables t ON t.id = r.table_id
WHERE r.venue_id = ? AND r.status = 'waiting' AND r.prepay = 0 AND r.created_at >= sqlc.arg(since)
ORDER BY r.created_at, r.id;

-- name: GuestRoundByPublicID :one
SELECT r.id, r.public_id, r.venue_id, CAST(coalesce(r.table_id, 0) AS INTEGER) AS table_id, r.status, r.order_id, r.lines, r.subtotal_minor, r.currency, r.created_at,
       r.prepay, r.due_minor, r.first_seq,
       CAST(coalesce(o.public_id, '') AS TEXT) AS order_public_id
FROM guest_rounds r
LEFT JOIN orders o ON o.id = r.order_id
WHERE r.public_id = ?;

-- name: DecideGuestRound :execrows
-- Answers a request: sent, onto a ticket, or declined. Only a waiting one
-- can be answered, so of two tills answering at once one writes nothing.
-- first_seq is where a sent round's lines begin on its ticket (ADR-236).
UPDATE guest_rounds
SET status = sqlc.arg(status), order_id = sqlc.arg(order_id), decided_at = sqlc.arg(decided_at), decided_by = sqlc.arg(decided_by),
    first_seq = sqlc.arg(first_seq)
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

-- name: CountUnpaidPickupRounds :one
-- The orders to collect at a venue waiting to be paid, placed since a moment
-- ago: what bounds how many a stranger with the venue's code can leave lying.
SELECT count(*) FROM guest_rounds
WHERE venue_id = ? AND table_id IS NULL AND status = 'waiting' AND prepay = 1 AND created_at >= sqlc.arg(since);

-- Guests' "I have paid" notices (ADR-218). A notice is a message to staff,
-- never a payment; see internal/service/order.

-- name: CreatePaidNotice :one
INSERT INTO paid_notices (public_id, venue_id, table_id, order_id, amount_minor, currency, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, 'open', ?)
RETURNING id;

-- name: OpenPaidNoticeForOrder :one
-- The notice staff have still to deal with on a ticket, if there is one.
SELECT id, public_id, table_id, order_id, amount_minor, currency, created_at
FROM paid_notices
WHERE order_id = ? AND status = 'open';

-- name: OpenPaidNotices :many
-- Every notice staff have still to deal with, oldest first, with its table
-- by name and its ticket by public id.
SELECT n.id, n.public_id, n.table_id, t.name AS table_name, n.order_id, o.public_id AS order_public_id,
       n.amount_minor, n.currency, n.created_at
FROM paid_notices n
JOIN tables t ON t.id = n.table_id
JOIN orders o ON o.id = n.order_id
WHERE n.venue_id = ? AND n.status = 'open' AND o.status != 'closed'
ORDER BY n.created_at, n.id;

-- name: PaidNoticeByPublicID :one
SELECT id, public_id, venue_id, table_id, order_id, amount_minor, currency, status, created_at
FROM paid_notices
WHERE public_id = ?;

-- name: MarkPaidNoticeSeen :execrows
-- Only an open notice can be dealt with, so of two tills doing it at once
-- one writes nothing.
UPDATE paid_notices
SET status = 'seen', seen_at = sqlc.arg(seen_at), seen_by = sqlc.arg(seen_by)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id) AND status = 'open';

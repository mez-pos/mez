-- Idempotency keys. Read and written only inside the transaction that
-- appends a command's events; see internal/service/order.

-- name: IdempotencyKey :one
SELECT venue_id, key, fingerprint, order_id, first_seq, last_seq, created_at
FROM idempotency_keys
WHERE venue_id = ? AND key = ?;

-- name: SaveIdempotencyKey :exec
INSERT INTO idempotency_keys (venue_id, key, fingerprint, order_id, first_seq, last_seq, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

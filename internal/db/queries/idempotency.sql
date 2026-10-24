-- Idempotency keys. Read and written only inside the transaction that
-- appends a command's events; see internal/service/order.

-- name: IdempotencyKey :one
SELECT venue_id, key, fingerprint, order_id, first_seq, last_seq, created_at
FROM idempotency_keys
WHERE venue_id = ? AND key = ?;

-- name: PruneIdempotencyKeys :execrows
--
-- Keys older than a cut-off. A key is for a retry of the same command
-- minutes later, not months; kept for ever the table grows by every command
-- the venue ever took, and travels in every backup (internal/service/tidy).
DELETE FROM idempotency_keys
WHERE created_at < ?;

-- name: SaveIdempotencyKey :exec
INSERT INTO idempotency_keys (venue_id, key, fingerprint, order_id, first_seq, last_seq, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

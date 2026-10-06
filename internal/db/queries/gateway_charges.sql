-- Charges asked of guests through a payment gateway (ADR-220). See
-- internal/service/charges.

-- name: CreateGatewayCharge :one
INSERT INTO gateway_charges (public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
                             amount_minor, currency, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?)
RETURNING id;

-- name: GatewayChargeByPublicID :one
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at
FROM gateway_charges
WHERE public_id = ?;

-- name: OpenGatewayChargeForOrder :one
-- A charge still open for a ticket at a provider for an amount: asked for
-- again, it is the same charge.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at
FROM gateway_charges
WHERE order_id = ? AND gateway = ? AND amount_minor = ? AND currency = ? AND status = 'open'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GatewayChargesByStatus :many
-- A venue's charges in a status, oldest first: the open ones to ask the
-- provider about, the unapplied ones a person must deal with.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at
FROM gateway_charges
WHERE venue_id = ? AND status = ?
ORDER BY created_at, id
LIMIT ?;

-- name: SetGatewayChargePayment :execrows
-- Writes how an open charge was paid, once. Whoever is second finds it
-- written and writes nothing.
UPDATE gateway_charges
SET method = sqlc.arg(method), reference = sqlc.arg(reference)
WHERE id = sqlc.arg(id) AND status = 'open' AND reference IS NULL;

-- name: SettleGatewayCharge :execrows
-- Closes an open charge: paid, unapplied or failed. Only an open one can
-- be closed.
UPDATE gateway_charges
SET status = sqlc.arg(status), problem = sqlc.arg(problem), settled_at = sqlc.arg(settled_at)
WHERE id = sqlc.arg(id) AND status = 'open';

-- name: VenuesWithOpenGatewayCharges :many
-- The venues a sweep has anything to ask a provider about. Usually none.
SELECT DISTINCT venue_id FROM gateway_charges WHERE status = 'open' ORDER BY venue_id;

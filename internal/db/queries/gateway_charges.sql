-- Charges asked of guests through a payment gateway (ADR-220). See
-- internal/service/charges.

-- name: CreateGatewayCharge :one
INSERT INTO gateway_charges (public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
                             amount_minor, currency, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?)
RETURNING id;

-- name: GatewayChargeByPublicID :one
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by
FROM gateway_charges
WHERE public_id = ?;

-- name: OpenGatewayChargeForOrder :one
-- A charge still open for a ticket at a provider for an amount: asked for
-- again, it is the same charge.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by
FROM gateway_charges
WHERE order_id = ? AND gateway = ? AND amount_minor = ? AND currency = ? AND status = 'open'
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GatewayChargesByStatus :many
-- A venue's charges in a status, oldest first: the open ones to ask the
-- provider about, the unapplied ones a person must deal with.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by
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

-- name: UnresolvedGatewayCharges :many
-- Money a gateway took that no bill could, and nobody has dealt with yet:
-- oldest first, with its table by name and its ticket by public id.
SELECT c.public_id, c.table_id, t.name AS table_name, o.public_id AS order_public_id, c.gateway, c.gateway_id,
       c.amount_minor, c.currency, c.method, c.reference, c.problem, c.settled_at
FROM gateway_charges c
JOIN tables t ON t.id = c.table_id
JOIN orders o ON o.id = c.order_id
WHERE c.venue_id = ? AND c.status = 'unapplied' AND c.resolved_at IS NULL
ORDER BY c.settled_at, c.id
LIMIT ?;

-- name: ResolveGatewayCharge :execrows
-- Says how an unapplied charge was dealt with, once. Whoever is second
-- writes nothing.
UPDATE gateway_charges
SET resolution = sqlc.arg(resolution), resolved_at = sqlc.arg(resolved_at), resolved_by = sqlc.arg(resolved_by)
WHERE public_id = sqlc.arg(public_id) AND venue_id = sqlc.arg(venue_id) AND status = 'unapplied' AND resolved_at IS NULL;

-- name: GatewayChargeByGatewayID :one
-- The charge a provider's notice is about, by the provider's own id.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by
FROM gateway_charges
WHERE gateway = ? AND gateway_id = ?;

-- Charges asked of guests through a payment gateway (ADR-220). See
-- internal/service/charges.

-- name: CreateGatewayCharge :one
INSERT INTO gateway_charges (public_id, venue_id, table_id, order_id, round_public_id, payer, gateway, gateway_id,
                             checkout_url, amount_minor, currency, status, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'open', ?)
RETURNING id;

-- name: GatewayChargeByPublicID :one
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE public_id = ?;

-- name: OpenGatewayChargeForOrder :one
-- A charge still open for the whole of what a ticket owes at a provider, for
-- an amount: asked for again, it is the same charge, whichever phone asks. A
-- round's charge is not one, and neither is a guest's share, nor a charge
-- the table let go of.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE order_id = ? AND round_public_id IS NULL AND payer IS NULL AND gateway = ? AND amount_minor = ? AND currency = ?
  AND status = 'open' AND let_go_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GatewayChargesByStatus :many
-- A venue's charges in a status, oldest first: the open ones to ask the
-- provider about, the unapplied ones a person must deal with.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE venue_id = ? AND status = ?
ORDER BY created_at, id
LIMIT ?;

-- name: OpenGatewayChargesSince :many
-- A venue's open charges started since a moment, oldest first: the ones
-- the sweep asks the provider about. A charge older than that has lapsed
-- at the provider and can never be paid; left in the list it would hold a
-- place in every sweep for ever, and with enough of them the charges that
-- could still land would never be asked about.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE venue_id = ? AND status = 'open' AND created_at >= sqlc.arg(since)
ORDER BY created_at, id
LIMIT sqlc.arg('limit');

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
-- oldest first, with its table by name and its ticket by public id. A
-- round's charge that never reached a ticket has none, and the charge for an
-- order to collect has no table.
SELECT c.public_id, CAST(coalesce(c.table_id, 0) AS INTEGER) AS table_id, CAST(coalesce(t.name, '') AS TEXT) AS table_name,
       CAST(coalesce(o.public_id, '') AS TEXT) AS order_public_id,
       c.gateway, c.gateway_id, c.amount_minor, c.currency, c.method, c.reference, c.problem, c.settled_at
FROM gateway_charges c
LEFT JOIN tables t ON t.id = c.table_id
LEFT JOIN orders o ON o.id = c.order_id
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
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE gateway = ? AND gateway_id = ?;

-- name: OpenGatewayChargeForRound :one
-- The charge still open for a round at a provider: asked for again, it is
-- the same charge. A round asks for one amount, so there is at most one.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE round_public_id = ? AND gateway = ? AND status = 'open';

-- name: SetGatewayChargeOrder :execrows
-- Writes the ticket a round's charge goes on, once the paid round is on it.
UPDATE gateway_charges
SET order_id = sqlc.arg(order_id)
WHERE id = sqlc.arg(id) AND order_id IS NULL AND status = 'open';

-- name: LastPaidGatewayCharge :one
-- When a payment through a provider last landed on a bill at a venue.
SELECT settled_at FROM gateway_charges
WHERE venue_id = ? AND gateway = ? AND status = 'paid'
ORDER BY settled_at DESC, id DESC
LIMIT 1;

-- name: OpenGatewayChargeForShare :one
-- The share a phone already has open on a ticket for an amount: asked for
-- again, it is the same charge. Another phone's share of the same amount is
-- another charge.
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by, round_public_id, payer, let_go_at
FROM gateway_charges
WHERE order_id = ? AND payer = ? AND gateway = ? AND amount_minor = ? AND currency = ? AND status = 'open'
  AND let_go_at IS NULL;

-- name: OpenSharesOnOrder :one
-- What guests are paying of a ticket in shares, now: every share still open.
-- A charge for the whole of what is left asks for the balance less this.
SELECT CAST(coalesce(sum(amount_minor), 0) AS INTEGER) AS claimed
FROM gateway_charges
WHERE order_id = ? AND round_public_id IS NULL AND payer IS NOT NULL AND status = 'open' AND let_go_at IS NULL;

-- name: OpenClaimsOfOthers :one
-- What others are paying of a ticket, now, as one phone sees it: every open
-- charge on the bill but that phone's own shares. A share fits in the
-- balance less this.
SELECT CAST(coalesce(sum(amount_minor), 0) AS INTEGER) AS claimed
FROM gateway_charges
WHERE order_id = sqlc.arg(order_id) AND round_public_id IS NULL AND status = 'open' AND let_go_at IS NULL
  AND (payer IS NULL OR payer <> sqlc.arg(payer));

-- name: LetGoGatewayCharge :execrows
-- Marks an open charge on a bill as given up, once: it is no longer a claim
-- on the bill (ADR-234). It stays open, since its page can still be paid. A
-- round's charge is not a claim and cannot be let go.
UPDATE gateway_charges
SET let_go_at = sqlc.arg(let_go_at)
WHERE id = sqlc.arg(id) AND status = 'open' AND round_public_id IS NULL AND let_go_at IS NULL;

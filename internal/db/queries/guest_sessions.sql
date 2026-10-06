-- A guest's phone's session for the sitting (ADR-262). internal/service/order
-- issues, checks, binds and ends them; internal/service/tablecodes ends a
-- table's when its code changes.

-- name: CreateGuestSession :exec
-- order_id is 0 for a session not yet bound to a ticket, and round_id 0 for
-- one not made by accepting a round: both are stored as null.
INSERT INTO guest_sessions (public_id, venue_id, table_id, order_id, token_hash, round_id, handoff, created_at, expires_at)
VALUES (sqlc.arg(public_id), sqlc.arg(venue_id), sqlc.arg(table_id), nullif(CAST(sqlc.arg(order_id) AS INTEGER), 0),
        sqlc.arg(token_hash), nullif(CAST(sqlc.arg(round_id) AS INTEGER), 0), sqlc.narg(handoff),
        sqlc.arg(created_at), sqlc.arg(expires_at));

-- name: GuestSessionByTokenHash :one
-- A presented session. Whether it is still good is the service's to say.
SELECT id, public_id, venue_id, table_id, CAST(coalesce(order_id, 0) AS INTEGER) AS order_id,
       expires_at, ended_at, CAST(handoff IS NOT NULL AS INTEGER) AS unclaimed
FROM guest_sessions
WHERE token_hash = ?;

-- name: ClaimGuestSession :exec
-- The phone has presented a session it was handed: the clear copy goes.
UPDATE guest_sessions SET handoff = NULL WHERE id = ?;

-- name: BindGuestSession :execrows
-- Binds a session to the ticket its round went onto. One bound to it already
-- counts as bound; one bound to another, ended or expired binds nothing.
UPDATE guest_sessions
SET order_id = sqlc.arg(order_id)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id) AND table_id = sqlc.arg(table_id)
  AND (order_id IS NULL OR order_id = sqlc.arg(order_id)) AND ended_at IS NULL AND expires_at > sqlc.arg(now);

-- name: GuestSessionHandoff :one
-- The session made for a round when it was accepted, in clear, while its
-- phone has still to present it.
SELECT CAST(handoff AS TEXT) AS handoff
FROM guest_sessions
WHERE round_id = sqlc.arg(round_id) AND handoff IS NOT NULL AND ended_at IS NULL AND expires_at > sqlc.arg(now)
ORDER BY id DESC
LIMIT 1;

-- name: EndGuestSessionsOfOrder :exec
-- A ticket has closed: every session bound to it is over.
UPDATE guest_sessions
SET ended_at = sqlc.arg(ended_at), handoff = NULL
WHERE order_id = sqlc.arg(order_id) AND ended_at IS NULL;

-- name: EndUnboundGuestSessionsAtTable :exec
-- A ticket at the table has closed: the sessions handed out there before
-- it opened, and never bound, belonged to that sitting or to nobody.
UPDATE guest_sessions
SET ended_at = sqlc.arg(ended_at), handoff = NULL
WHERE table_id = sqlc.arg(table_id) AND order_id IS NULL AND ended_at IS NULL;

-- name: EndGuestSessionsAtTable :exec
-- The table's code has been renewed or turned off: whoever held it, every
-- session it gave is over.
UPDATE guest_sessions
SET ended_at = sqlc.arg(ended_at), handoff = NULL
WHERE venue_id = sqlc.arg(venue_id) AND table_id = sqlc.arg(table_id) AND ended_at IS NULL;

-- name: PruneGuestSessions :exec
-- Sessions at a table that expired before a moment ago: no use to anyone,
-- and a table hands out a few every sitting.
DELETE FROM guest_sessions WHERE table_id = sqlc.arg(table_id) AND expires_at < sqlc.arg(before);

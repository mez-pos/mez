-- A venue's pickup code (ADR-247). See internal/service/tablecodes.

-- name: CreatePickupCode :exec
INSERT INTO pickup_codes (venue_id, code, created_at)
VALUES (?, ?, ?);

-- name: RevokePickupCodes :exec
-- Turns off whatever pickup code a venue has. A venue has at most one live.
UPDATE pickup_codes
SET revoked_at = ?
WHERE venue_id = ? AND revoked_at IS NULL;

-- name: PickupCodeForVenue :one
-- The venue's live pickup code. No row when it has none.
SELECT code FROM pickup_codes
WHERE venue_id = ? AND revoked_at IS NULL;

-- name: VenueByPickupCode :one
-- What a pickup code leads to: the venue, and no table. A revoked code
-- leads nowhere.
SELECT c.venue_id, v.name AS venue_name, v.currency
FROM pickup_codes c
JOIN venues v ON v.id = c.venue_id
WHERE c.code = ? AND c.revoked_at IS NULL;

-- name: CodeIsATables :one
-- Whether a code was ever a table's, live or not: a pickup code is never
-- one, so the two cannot be mistaken for each other on a sticker.
SELECT count(*) FROM table_codes WHERE code = ?;

-- name: CodeIsAPickups :one
-- Whether a code was ever a venue's pickup code, live or not.
SELECT count(*) FROM pickup_codes WHERE code = ?;

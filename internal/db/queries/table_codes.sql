-- name: CreateTableCode :exec
INSERT INTO table_codes (venue_id, table_id, code, created_at)
VALUES (?, ?, ?, ?);

-- name: RevokeTableCodes :exec
-- Turns off whatever code a table has. A table has at most one live code.
UPDATE table_codes
SET revoked_at = ?
WHERE venue_id = ? AND table_id = ? AND revoked_at IS NULL;

-- name: TableByCode :one
-- What a guest's code leads to. A revoked code, and a code for a table that
-- has been taken off the floor, lead nowhere.
SELECT c.venue_id, v.name AS venue_name, v.currency, t.id AS table_id, t.name AS table_name
FROM table_codes c
JOIN tables t ON t.id = c.table_id
JOIN venues v ON v.id = c.venue_id
WHERE c.code = ? AND c.revoked_at IS NULL AND t.active = 1;

-- name: TableCodesForVenue :many
-- Every table on the floor with its live code, or an empty code when it has
-- none yet.
SELECT t.id AS table_id, t.name AS table_name, CAST(COALESCE(c.code, '') AS TEXT) AS code
FROM tables t
LEFT JOIN table_codes c ON c.table_id = t.id AND c.revoked_at IS NULL
WHERE t.venue_id = ? AND t.active = 1
ORDER BY t.id;

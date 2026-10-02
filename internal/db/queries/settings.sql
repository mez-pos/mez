-- Per-venue settings the schema has no opinion about. Anything the database
-- must constrain or compute with is a column on venues instead.

-- name: SettingsForVenue :many
SELECT venue_id, key, value, updated_at
FROM settings
WHERE venue_id = ?
ORDER BY key;

-- name: Setting :one
SELECT venue_id, key, value, updated_at
FROM settings
WHERE venue_id = ? AND key = ?;

-- name: SetSetting :exec
--
-- An upsert rather than INSERT OR REPLACE. Replace deletes the row and inserts
-- a new one, which fires delete triggers and cascades; this only touches the
-- two columns that change.
INSERT INTO settings (venue_id, key, value, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (venue_id, key) DO UPDATE
SET value      = excluded.value,
    updated_at = excluded.updated_at;

-- name: DeleteSetting :exec
DELETE FROM settings
WHERE venue_id = ? AND key = ?;

-- name: UnsettledOrderCount :one
-- Orders that are not closed: a ticket open, held, being paid or reopened.
SELECT COUNT(*)
FROM orders
WHERE venue_id = ? AND status != 'closed';

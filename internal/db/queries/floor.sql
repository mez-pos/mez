-- The floor plan: areas and the tables placed in them (ADR-085).

-- name: AreasForVenue :many
SELECT id, name, active
FROM areas
WHERE venue_id = ?
ORDER BY id;

-- name: AreaByID :one
SELECT id, venue_id, name, active
FROM areas
WHERE id = ?;

-- name: CreateArea :one
INSERT INTO areas (venue_id, name, active, created_at, updated_at)
VALUES (?, ?, 1, ?, ?)
RETURNING id;

-- name: RenameArea :execrows
UPDATE areas SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id);

-- name: SetAreaActive :execrows
UPDATE areas SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id);

-- name: CountActiveTablesInArea :one
SELECT count(*) FROM tables WHERE area_id = ? AND active = 1;

-- name: TablesForVenue :many
SELECT id, area_id, name, seats, shape, x, y, active
FROM tables
WHERE venue_id = ?
ORDER BY area_id, y, x, id;

-- name: TableByID :one
SELECT id, venue_id, area_id, name, seats, shape, x, y, active
FROM tables
WHERE id = ?;

-- name: TableInCell :one
-- The table in service in a cell, if any, other than the one being placed.
SELECT id, name
FROM tables
WHERE area_id = sqlc.arg(area_id) AND x = sqlc.arg(x) AND y = sqlc.arg(y) AND active = 1 AND id != sqlc.arg(except_id);

-- name: CreateTable :one
INSERT INTO tables (venue_id, area_id, name, seats, shape, x, y, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
RETURNING id;

-- name: UpdateTable :execrows
UPDATE tables
SET area_id = sqlc.arg(area_id), name = sqlc.arg(name), seats = sqlc.arg(seats), shape = sqlc.arg(shape),
    x = sqlc.arg(x), y = sqlc.arg(y), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id);

-- name: SetTableActive :execrows
UPDATE tables SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id);

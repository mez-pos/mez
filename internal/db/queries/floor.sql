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

-- name: LiveTables :many
-- The tables in service, with when each was last cleared and when the last
-- order at it closed, joined tables included (ADR-088): what a table's turn
-- is derived from, with its orders.
SELECT t.id, t.area_id, t.name, t.seats, t.shape, t.x, t.y, t.cleared_at,
       (SELECT ot.closed_at FROM order_tables ot
        WHERE ot.table_id = t.id AND ot.closed_at IS NOT NULL
        ORDER BY ot.closed_at DESC LIMIT 1) AS last_closed_at
FROM tables t
JOIN areas a ON a.id = t.area_id
WHERE t.venue_id = ? AND t.active = 1 AND a.active = 1
ORDER BY t.area_id, t.y, t.x, t.id;

-- name: TableTurn :one
-- One table's turn: the orders at it not yet closed, its own or joined, the
-- last one to close there, and when it was last cleared. For clearing it,
-- and for moving or joining a party to it.
SELECT t.id, t.venue_id, t.name, t.active, t.cleared_at,
       (SELECT count(*) FROM order_tables ot
        WHERE ot.table_id = t.id AND ot.closed_at IS NULL) AS unclosed,
       (SELECT ot.closed_at FROM order_tables ot
        WHERE ot.table_id = t.id AND ot.closed_at IS NOT NULL
        ORDER BY ot.closed_at DESC LIMIT 1) AS last_closed_at
FROM tables t
WHERE t.id = ?;

-- name: ClearTable :execrows
UPDATE tables SET cleared_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id);

-- name: BumpFloorVersion :exec
-- Moves the venue's floor version on, in the transaction that changed it.
INSERT INTO floor_versions (venue_id, version)
VALUES (?, 1)
ON CONFLICT (venue_id) DO UPDATE SET version = floor_versions.version + 1;

-- name: FloorVersion :one
SELECT CAST(coalesce((SELECT version FROM floor_versions WHERE venue_id = ?), 0) AS INTEGER) AS version;

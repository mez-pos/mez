-- The menu. Reads return active rows only, in the order a terminal shows
-- them: a retired item stays in the table for the events that name it, but
-- never reaches a till. db.DB.Menu runs the reads in one read transaction,
-- so the version and the rows it describes come from the same snapshot.

-- name: MenuVersion :one
--
-- A venue whose menu has never been written has no row, and is at 1.
SELECT CAST(COALESCE(
    (SELECT version FROM menu_versions WHERE venue_id = sqlc.arg(venue_id)), 1
) AS INTEGER) AS version;

-- name: BumpMenuVersion :one
--
-- Every write to a menu table calls this in the same transaction. A write
-- that forgot would leave terminals selling from a cached menu, which is
-- why the writes live behind db.DB rather than being called loose.
INSERT INTO menu_versions (venue_id, version)
VALUES (?, 2)
ON CONFLICT (venue_id) DO UPDATE
SET version = menu_versions.version + 1
RETURNING version;

-- name: ActiveStations :many
SELECT id, code, name, sort_order
FROM stations
WHERE venue_id = ? AND active = 1
ORDER BY sort_order, id;

-- name: CoursesForVenue :many
SELECT id, number, name
FROM courses
WHERE venue_id = ?
ORDER BY number;

-- name: TaxClassesForVenue :many
SELECT id, name, rate, exempt
FROM tax_classes
WHERE venue_id = ?
ORDER BY id;

-- name: ActiveCategories :many
SELECT id, name, sort_order
FROM categories
WHERE venue_id = ? AND active = 1
ORDER BY sort_order, id;

-- name: ActiveItems :many
--
-- An item in a retired category is not on sale either: retiring
-- "Breakfast" at noon takes every breakfast item off the till with it.
SELECT
    i.id, i.category_id, i.name, i.price, i.hsn,
    i.tax_class_id, i.station_id, i.course_id, i.sort_order
FROM items i
JOIN categories c ON c.venue_id = i.venue_id AND c.id = i.category_id
WHERE i.venue_id = ? AND i.active = 1 AND c.active = 1
ORDER BY i.category_id, i.sort_order, i.id;

-- name: ActiveModifierGroups :many
SELECT id, name, min_select, max_select
FROM modifier_groups
WHERE venue_id = ? AND active = 1
ORDER BY id;

-- name: ActiveModifiers :many
SELECT id, group_id, name, price, is_default, sort_order
FROM modifiers
WHERE venue_id = ? AND active = 1
ORDER BY group_id, sort_order, id;

-- name: ItemModifierGroupsForVenue :many
SELECT item_id, group_id, sort_order
FROM item_modifier_groups
WHERE venue_id = ?
ORDER BY item_id, sort_order, group_id;

-- name: CreateStation :one
INSERT INTO stations (venue_id, code, name, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateCourse :one
INSERT INTO courses (venue_id, number, name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateTaxClass :one
INSERT INTO tax_classes (venue_id, name, rate, exempt, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateCategory :one
INSERT INTO categories (venue_id, name, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateItem :one
INSERT INTO items (
    venue_id, category_id, name, price, hsn,
    tax_class_id, station_id, course_id, sort_order, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateModifierGroup :one
INSERT INTO modifier_groups (venue_id, name, min_select, max_select, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreateModifier :one
INSERT INTO modifiers (venue_id, group_id, name, price, is_default, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: AttachModifierGroup :exec
INSERT INTO item_modifier_groups (venue_id, item_id, group_id, sort_order)
VALUES (?, ?, ?, ?);

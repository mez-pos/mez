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
WHERE venue_id = ? AND active = 1
ORDER BY number;

-- name: TaxClassesForVenue :many
SELECT id, name, rate, exempt, active
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
    i.tax_class_id, i.station_id, i.course_id, i.sort_order, i.sold_out_until, i.photo,
    i.description, i.diet, i.gluten_free, i.popular
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
    tax_class_id, station_id, course_id, sort_order, created_at, updated_at,
    description, diet, gluten_free, popular
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

-- The back office's editor reads everything, retired rows included, so a
-- retired item can be brought back. db.DB.MenuEditor runs these in one read
-- transaction with MenuVersion.

-- name: EditorStations :many
SELECT id, code, name, active
FROM stations
WHERE venue_id = ?
ORDER BY sort_order, id;

-- name: EditorCourses :many
SELECT id, number, name, active
FROM courses
WHERE venue_id = ?
ORDER BY number;

-- name: EditorCategories :many
SELECT id, name, sort_order, active
FROM categories
WHERE venue_id = ?
ORDER BY sort_order, id;

-- name: EditorItems :many
SELECT
    id, category_id, name, price, hsn,
    tax_class_id, station_id, course_id, sort_order, active, sold_out_until, photo,
    description, diet, gluten_free, popular
FROM items
WHERE venue_id = ?
ORDER BY category_id, sort_order, id;

-- The editor's writes. Each goes through db.DB.EditMenu, which bumps the
-- menu version in the same transaction. Every one is scoped by venue_id:
-- an id from another venue matches nothing.

-- name: CategoryForEdit :one
SELECT id, active
FROM categories
WHERE venue_id = ? AND id = ?;

-- name: ItemForEdit :one
SELECT id, category_id, sort_order, active
FROM items
WHERE venue_id = ? AND id = ?;

-- name: StationForEdit :one
SELECT id, active
FROM stations
WHERE venue_id = ? AND id = ?;

-- name: TaxClassForEdit :one
SELECT id, active
FROM tax_classes
WHERE venue_id = ? AND id = ?;

-- name: CourseForEdit :one
SELECT id, active
FROM courses
WHERE venue_id = ? AND id = ?;

-- name: NextCategorySort :one
SELECT CAST(COALESCE(MAX(sort_order) + 1, 0) AS INTEGER) AS next
FROM categories
WHERE venue_id = ?;

-- name: NextItemSort :one
SELECT CAST(COALESCE(MAX(sort_order) + 1, 0) AS INTEGER) AS next
FROM items
WHERE venue_id = ? AND category_id = ?;

-- name: CategoryIDs :many
SELECT id
FROM categories
WHERE venue_id = ?
ORDER BY id;

-- name: ItemIDsInCategory :many
SELECT id
FROM items
WHERE venue_id = ? AND category_id = ?
ORDER BY id;

-- name: RenameCategory :execrows
UPDATE categories
SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetCategoryActive :execrows
UPDATE categories
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetCategorySort :execrows
UPDATE categories
SET sort_order = sqlc.arg(sort_order), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: UpdateItem :execrows
--
-- A line already rung up keeps the name, price and tax it copied
-- (docs/02-domain.md, "Prices and the snapshot rule"); this changes only
-- what the next line copies.
UPDATE items
SET category_id = sqlc.arg(category_id), name = sqlc.arg(name), price = sqlc.arg(price),
    hsn = sqlc.arg(hsn), tax_class_id = sqlc.arg(tax_class_id), station_id = sqlc.arg(station_id),
    course_id = sqlc.arg(course_id), sort_order = sqlc.arg(sort_order), updated_at = sqlc.arg(now),
    description = sqlc.arg(description), diet = sqlc.arg(diet), gluten_free = sqlc.arg(gluten_free),
    popular = sqlc.arg(popular)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetItemActive :execrows
UPDATE items
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetItemSort :execrows
UPDATE items
SET sort_order = sqlc.arg(sort_order), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- Modifier groups in the editor (phase 10, Task 2c). A group's choices are
-- saved with it: a choice left out is retired, not deleted, so a line that
-- chose it still names something.

-- name: EditorModifierGroups :many
SELECT id, name, min_select, max_select, active
FROM modifier_groups
WHERE venue_id = ?
ORDER BY name, id;

-- name: GroupForEdit :one
SELECT id, active
FROM modifier_groups
WHERE venue_id = ? AND id = ?;

-- name: UpdateModifierGroup :execrows
UPDATE modifier_groups
SET name = sqlc.arg(name), min_select = sqlc.arg(min_select), max_select = sqlc.arg(max_select),
    updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetModifierGroupActive :execrows
UPDATE modifier_groups
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: ModifierIDsOfGroup :many
SELECT id
FROM modifiers
WHERE venue_id = ? AND group_id = ?
ORDER BY id;

-- name: UpdateModifier :execrows
--
-- Saving a choice also brings it back: one the owner lists is on offer.
UPDATE modifiers
SET name = sqlc.arg(name), price = sqlc.arg(price), is_default = sqlc.arg(is_default),
    sort_order = sqlc.arg(sort_order), active = 1, updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND group_id = sqlc.arg(group_id) AND id = sqlc.arg(id);

-- name: RetireModifier :execrows
UPDATE modifiers
SET active = 0, is_default = 0, updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: DetachItemGroups :exec
DELETE FROM item_modifier_groups
WHERE venue_id = ? AND item_id = ?;

-- name: SetItemSoldOut :execrows
--
-- NULL puts it back on sale now; a time keeps it sold out until then.
UPDATE items
SET sold_out_until = sqlc.narg(until), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetItemPhoto :execrows
--
-- The file is written before this runs; '' takes the photo off. A file no
-- item names any more stays on disk (ADR-110).
UPDATE items
SET photo = sqlc.arg(photo), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: UpdateTaxClass :execrows
UPDATE tax_classes
SET name = sqlc.arg(name), rate = sqlc.arg(rate), exempt = sqlc.arg(exempt), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetTaxClassActive :execrows
UPDATE tax_classes
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: ActiveItemsOnTaxClass :many
-- The items on the menu now that charge this class: what stops it being
-- retired.
SELECT name
FROM items
WHERE venue_id = ? AND tax_class_id = ? AND active = 1
ORDER BY name, id;

-- name: ItemTaxClassActive :one
SELECT tax_classes.active
FROM items
JOIN tax_classes ON tax_classes.venue_id = items.venue_id AND tax_classes.id = items.tax_class_id
WHERE items.venue_id = ? AND items.id = ?;

-- name: ItemsForHSN :many
-- Every item on the menu now, with the code it carries: what a profile
-- that prints HSN needs every one to have.
SELECT id, name, hsn
FROM items
WHERE venue_id = ? AND active = 1
ORDER BY name, id;

-- Stations and courses (phase 10, Task 6e). Each write goes through
-- db.DB.EditMenu and is scoped by venue_id.

-- name: NextStationSort :one
SELECT CAST(COALESCE(MAX(sort_order) + 1, 0) AS INTEGER) AS next
FROM stations
WHERE venue_id = ?;

-- name: StationIDs :many
SELECT id
FROM stations
WHERE venue_id = ?
ORDER BY id;

-- name: StationRouting :one
-- A station as retiring it needs to see it: whether it is in service and the
-- printer its tickets go to, if any.
SELECT id, active, printer_id
FROM stations
WHERE venue_id = ? AND id = ?;

-- name: ActiveStationCount :one
SELECT COUNT(*)
FROM stations
WHERE venue_id = ? AND active = 1;

-- name: RenameStation :execrows
UPDATE stations
SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStationSort :execrows
UPDATE stations
SET sort_order = sqlc.arg(sort_order), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStationActive :execrows
UPDATE stations
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStationPrinterByID :execrows
UPDATE stations
SET printer_id = sqlc.narg(printer_id), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: MoveStationItems :execrows
-- Every item of a station, retired ones too: one brought back later must not
-- name a station that is gone.
UPDATE items
SET station_id = sqlc.arg(to_station), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND station_id = sqlc.arg(from_station);

-- name: ItemCountForStation :one
SELECT COUNT(*)
FROM items
WHERE venue_id = ? AND station_id = ?;

-- name: NextCourseNumber :one
SELECT CAST(COALESCE(MAX(number) + 1, 1) AS INTEGER) AS next
FROM courses
WHERE venue_id = ?;

-- name: CourseIDsByNumber :many
SELECT id
FROM courses
WHERE venue_id = ?
ORDER BY number;

-- name: ActiveCourseCount :one
SELECT COUNT(*)
FROM courses
WHERE venue_id = ? AND active = 1;

-- name: RenameCourse :execrows
UPDATE courses
SET name = sqlc.arg(name), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetCourseNumber :execrows
UPDATE courses
SET number = sqlc.arg(number), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetCourseActive :execrows
UPDATE courses
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: MoveCourseItems :execrows
UPDATE items
SET course_id = sqlc.arg(to_course), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND course_id = sqlc.arg(from_course);

-- name: ItemCountForCourse :one
SELECT COUNT(*)
FROM items
WHERE venue_id = ? AND course_id = ?;

-- name: ItemStationCourseActive :one
-- Whether an item's station and course are both in service.
SELECT stations.active AS station_active, courses.active AS course_active
FROM items
JOIN stations ON stations.venue_id = items.venue_id AND stations.id = items.station_id
JOIN courses ON courses.venue_id = items.venue_id AND courses.id = items.course_id
WHERE items.venue_id = ? AND items.id = ?;

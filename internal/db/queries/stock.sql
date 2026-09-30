-- Stock (phase 10, Task 3a, ADR-111). Every query is scoped by venue_id: an
-- id from another venue matches nothing. On hand is always the sum of the
-- ledger, never a stored number.

-- name: CreateStockItem :one
INSERT INTO stock_items (venue_id, name, unit, low_level, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateStockItem :execrows
UPDATE stock_items
SET name = sqlc.arg(name), unit = sqlc.arg(unit), low_level = sqlc.arg(low_level), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStockItemActive :execrows
UPDATE stock_items
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: StockItemForEdit :one
SELECT id, unit, active
FROM stock_items
WHERE venue_id = ? AND id = ?;

-- name: StockItemsWithOnHand :many
--
-- Every item, retired ones too, with what the ledger says is on hand.
SELECT
    i.id, i.name, i.unit, i.low_level, i.active,
    CAST(COALESCE(SUM(m.delta), 0) AS INTEGER) AS on_hand
FROM stock_items i
LEFT JOIN stock_movements m ON m.venue_id = i.venue_id AND m.stock_item_id = i.id
WHERE i.venue_id = ?
GROUP BY i.id
ORDER BY i.name COLLATE NOCASE, i.id;

-- name: StockOnHand :one
SELECT CAST(COALESCE(SUM(delta), 0) AS INTEGER) AS on_hand
FROM stock_movements
WHERE venue_id = ? AND stock_item_id = ?;

-- name: StockMovementCount :one
SELECT COUNT(*) AS n
FROM stock_movements
WHERE venue_id = ? AND stock_item_id = ?;

-- name: InsertStockMovement :one
INSERT INTO stock_movements (
    venue_id, stock_item_id, kind, delta, counted, reason, note, staff_id, shift_id, order_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: StockMovementsForItem :many
--
-- Newest first, a page at a time: before_id is the last id of the page
-- before, or 0 for the first page.
SELECT
    m.id, m.kind, m.delta, m.counted, m.reason, m.note, m.shift_id, m.order_id, m.created_at,
    s.public_id AS staff_id, s.name AS staff_name
FROM stock_movements m
JOIN staff s ON s.id = m.staff_id
WHERE m.venue_id = sqlc.arg(venue_id) AND m.stock_item_id = sqlc.arg(stock_item_id)
  AND (sqlc.arg(before_id) = 0 OR m.id < sqlc.arg(before_id))
ORDER BY m.id DESC
LIMIT sqlc.arg(max_rows);

-- Recipes (Task 3b, ADR-112).

-- name: ItemRecipesForVenue :many
SELECT item_id, stock_item_id, qty
FROM item_recipes
WHERE venue_id = ?
ORDER BY item_id, stock_item_id;

-- name: ModifierRecipesForVenue :many
SELECT modifier_id, stock_item_id, qty
FROM modifier_recipes
WHERE venue_id = ?
ORDER BY modifier_id, stock_item_id;

-- name: ClearItemRecipe :exec
DELETE FROM item_recipes WHERE venue_id = ? AND item_id = ?;

-- name: AddItemRecipePart :exec
INSERT INTO item_recipes (venue_id, item_id, stock_item_id, qty) VALUES (?, ?, ?, ?);

-- name: ClearModifierRecipe :exec
DELETE FROM modifier_recipes WHERE venue_id = ? AND modifier_id = ?;

-- name: AddModifierRecipePart :exec
INSERT INTO modifier_recipes (venue_id, modifier_id, stock_item_id, qty) VALUES (?, ?, ?, ?);

-- name: MenuItemForRecipe :one
SELECT id FROM items WHERE venue_id = ? AND id = ?;

-- name: ModifierForRecipe :one
SELECT id FROM modifiers WHERE venue_id = ? AND id = ?;

-- name: SoldForOrder :many
--
-- What an order's sold movements have taken so far, per stock item, as a
-- positive amount.
SELECT stock_item_id, CAST(-SUM(delta) AS INTEGER) AS taken
FROM stock_movements
WHERE venue_id = ? AND order_id = ? AND kind = 'sold'
GROUP BY stock_item_id;

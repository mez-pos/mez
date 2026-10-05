-- Stock (phase 10, Task 3a, ADR-111). Every query is scoped by venue_id: an
-- id from another venue matches nothing. On hand is always the sum of the
-- ledger, never a stored number.

-- name: CreateStockItem :one
INSERT INTO stock_items (venue_id, name, unit, low_level, count_at_close, cost, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateStockItem :execrows
UPDATE stock_items
SET name = sqlc.arg(name), unit = sqlc.arg(unit), low_level = sqlc.arg(low_level),
    count_at_close = sqlc.arg(count_at_close), cost = sqlc.arg(cost), updated_at = sqlc.arg(now)
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
    i.id, i.name, i.unit, i.low_level, i.active, i.count_at_close, i.cost,
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
--
-- unit_cost is the item's cost as it stands, read here so no caller can
-- forget it and none can pass another (ADR-124).
INSERT INTO stock_movements (
    venue_id, stock_item_id, kind, delta, counted, reason, note, staff_id, shift_id, order_id, closing, created_at,
    menu_item_id, unit_cost
) VALUES (
    sqlc.arg(venue_id), sqlc.arg(stock_item_id), sqlc.arg(kind), sqlc.arg(delta), sqlc.arg(counted), sqlc.arg(reason),
    sqlc.arg(note), sqlc.arg(staff_id), sqlc.arg(shift_id), sqlc.arg(order_id), sqlc.arg(closing), sqlc.arg(created_at),
    sqlc.arg(menu_item_id),
    (SELECT i.cost FROM stock_items i WHERE i.venue_id = sqlc.arg(venue_id) AND i.id = sqlc.arg(stock_item_id))
)
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
-- What an order's sold movements have taken so far, per stock item and
-- the dish it went into (0 for stock taken before the dish was kept,
-- ADR-189), as a positive amount.
SELECT stock_item_id, menu_item_id, CAST(-SUM(delta) AS INTEGER) AS taken
FROM stock_movements
WHERE venue_id = ? AND order_id = ? AND kind = 'sold'
GROUP BY stock_item_id, menu_item_id;

-- Closing stock (Task 3d, ADR-114).

-- name: ItemsToCountAtClose :many
SELECT id, name, unit
FROM stock_items
WHERE venue_id = ? AND active = 1 AND count_at_close = 1
ORDER BY name COLLATE NOCASE, id;

-- name: ClosingCountForShift :many
--
-- The close's count of a shift: what was counted and what it moved.
SELECT m.stock_item_id, i.name, i.unit, m.counted, m.delta
FROM stock_movements m
JOIN stock_items i ON i.venue_id = m.venue_id AND i.id = m.stock_item_id
WHERE m.venue_id = ? AND m.shift_id = ? AND m.closing = 1
ORDER BY i.name COLLATE NOCASE, i.id;

-- name: ClosingCountsInRange :many
--
-- The closing counts of the shifts whose trading day is in a range (Task
-- 5f2, ADR-125), by day, then shift, then item. unit_cost is the item's
-- cost when it was counted, 0 for not known (ADR-124).
SELECT
    m.shift_id, sh.public_id AS shift_public_id, sh.trading_day,
    m.stock_item_id, i.name, i.unit, m.counted, m.delta, m.unit_cost
FROM stock_movements m
JOIN shifts sh ON sh.id = m.shift_id AND sh.venue_id = m.venue_id
JOIN stock_items i ON i.venue_id = m.venue_id AND i.id = m.stock_item_id
WHERE m.venue_id = sqlc.arg(venue_id) AND m.closing = 1
  AND sh.trading_day >= sqlc.arg(from_day) AND sh.trading_day <= sqlc.arg(to_day)
ORDER BY sh.trading_day, sh.id, i.name COLLATE NOCASE, i.id;

-- name: WastedBetween :many
--
-- What was written off between two instants (rebuild Task 16d1, ADR-186),
-- oldest first. delta is below zero; unit_cost is the item's cost when it
-- was written off, 0 for not known (ADR-124).
SELECT m.stock_item_id, i.name, i.unit, m.delta, m.reason, m.unit_cost, m.created_at
FROM stock_movements m
JOIN stock_items i ON i.venue_id = m.venue_id AND i.id = m.stock_item_id
WHERE m.venue_id = sqlc.arg(venue_id) AND m.kind = 'wasted'
  AND m.created_at >= sqlc.arg(from_at) AND m.created_at < sqlc.arg(to_at)
ORDER BY m.created_at, m.id;

-- name: SoldBetween :many
--
-- The stock sales took between two instants (rebuild Task 16d3, ADR-189),
-- oldest first: the dish it went into (0 when the ledger did not keep it),
-- the delta (below zero for stock taken, above for stock put back) and the
-- cost the stock had then, 0 for not known (ADR-124).
SELECT m.menu_item_id, i.unit, m.delta, m.unit_cost, m.created_at
FROM stock_movements m
JOIN stock_items i ON i.venue_id = m.venue_id AND i.id = m.stock_item_id
WHERE m.venue_id = sqlc.arg(venue_id) AND m.kind = 'sold'
  AND m.created_at >= sqlc.arg(from_at) AND m.created_at < sqlc.arg(to_at)
ORDER BY m.created_at, m.id;

-- name: FirstSoldByDish :many
--
-- When the ledger first kept the dish on a sale: the oldest such movement,
-- or no row for a venue that has none yet.
SELECT created_at
FROM stock_movements
WHERE venue_id = ? AND kind = 'sold' AND menu_item_id != 0
ORDER BY created_at, id
LIMIT 1;

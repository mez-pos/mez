-- +goose Up
-- Which dish stock went into (rebuild Task 16d3, ADR-189). A sold movement
-- keeps the menu item whose recipe used the stock, a modifier's stock going
-- to its line's item, so what a dish cost to make can be added up from the
-- ledger at the cost each movement carried (ADR-124). 0 is "no dish": every
-- movement that is not a sale, and every sale from before this. Adding a
-- column is not an UPDATE, so the ledger's no-update trigger does not object.

ALTER TABLE stock_movements ADD COLUMN menu_item_id INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(menu_item_id) = 'integer' AND menu_item_id >= 0);

-- +goose Down
ALTER TABLE stock_movements DROP COLUMN menu_item_id;

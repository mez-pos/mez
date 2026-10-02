-- +goose Up
-- What stock costs (phase 10, Task 5f1, ADR-124). cost is minor units of
-- the venue's currency for a kilogram, a litre or one of the item, by its
-- unit; 0 is "not known". Every movement keeps the cost as it was when it
-- was written (unit_cost), so a variance from last month is never valued
-- at today's price. Rows from before this have 0: not known, not free.
-- Adding a column is not an UPDATE, so the ledger's no-update trigger does
-- not object.

ALTER TABLE stock_items ADD COLUMN cost INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(cost) = 'integer' AND cost >= 0);

ALTER TABLE stock_movements ADD COLUMN unit_cost INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(unit_cost) = 'integer' AND unit_cost >= 0);

-- +goose Down
ALTER TABLE stock_movements DROP COLUMN unit_cost;
ALTER TABLE stock_items DROP COLUMN cost;

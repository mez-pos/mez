-- +goose Up
-- Closing stock (phase 10, Task 3d, ADR-114). An item marked count_at_close
-- is counted, blind, as the shift closes; those counts are ordinary
-- `counted` movements of that shift, marked closing so the close's count
-- can be found and taken only once. Adding a column is not an UPDATE, so
-- the ledger's no-update trigger does not object.

ALTER TABLE stock_items ADD COLUMN count_at_close BOOLEAN NOT NULL DEFAULT 0
    CHECK (count_at_close IN (0, 1));

ALTER TABLE stock_movements ADD COLUMN closing BOOLEAN NOT NULL DEFAULT 0
    CHECK (closing IN (0, 1));

CREATE INDEX stock_movements_closing ON stock_movements (shift_id) WHERE closing = 1;

-- +goose Down
DROP INDEX stock_movements_closing;
ALTER TABLE stock_movements DROP COLUMN closing;
ALTER TABLE stock_items DROP COLUMN count_at_close;

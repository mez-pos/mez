-- +goose Up
-- How many of a dish the stock ledger has accounted for (rebuild Task 16d3,
-- ADR-189). A dish's cost is what its sold movements moved (00035); this is
-- how many units that cost covers, so a margin is given only when the
-- ledger costed every unit the dish sold. A dish sold before it had a
-- recipe moved no stock and has no row here: its cost is short, and says so.
--
-- Append-only, as the ledger is. A row is a change: above zero when an order
-- closes having used stock for the dish, below when a line voided after a
-- reopen puts its stock back.

CREATE TABLE stock_dish_sales (
    id           INTEGER PRIMARY KEY,
    venue_id     INTEGER NOT NULL REFERENCES venues (id),
    order_id     INTEGER NOT NULL,
    menu_item_id INTEGER NOT NULL,
    qty          INTEGER NOT NULL,
    created_at   TIMESTAMP NOT NULL,

    CHECK (typeof(order_id) = 'integer' AND order_id > 0),
    CHECK (typeof(menu_item_id) = 'integer' AND menu_item_id > 0),
    CHECK (typeof(qty) = 'integer' AND qty != 0)
);

CREATE INDEX stock_dish_sales_order ON stock_dish_sales (venue_id, order_id);
CREATE INDEX stock_dish_sales_at ON stock_dish_sales (venue_id, created_at);

-- +goose StatementBegin
CREATE TRIGGER stock_dish_sales_no_update BEFORE UPDATE ON stock_dish_sales
BEGIN
    SELECT RAISE(ABORT, 'a dish sale is written once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER stock_dish_sales_no_delete BEFORE DELETE ON stock_dish_sales
BEGIN
    SELECT RAISE(ABORT, 'a dish sale is written once and never changed');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER stock_dish_sales_no_delete;
DROP TRIGGER stock_dish_sales_no_update;
DROP INDEX stock_dish_sales_at;
DROP INDEX stock_dish_sales_order;
DROP TABLE stock_dish_sales;

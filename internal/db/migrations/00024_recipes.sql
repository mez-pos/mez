-- +goose Up
-- Recipes (phase 10, Task 3b, ADR-112): what one of a menu item, or one
-- choice of a modifier, takes out of stock. qty is thousandths of the stock
-- item's unit, per unit sold. A modifier's recipe adds to its item's, so a
-- substitution (oat milk for dairy) lives on the modifiers.

CREATE TABLE item_recipes (
    venue_id      INTEGER NOT NULL,
    item_id       INTEGER NOT NULL,
    stock_item_id INTEGER NOT NULL,
    qty           INTEGER NOT NULL,

    PRIMARY KEY (item_id, stock_item_id),
    FOREIGN KEY (venue_id, item_id)       REFERENCES items (venue_id, id) ON DELETE CASCADE,
    FOREIGN KEY (venue_id, stock_item_id) REFERENCES stock_items (venue_id, id),
    CHECK (typeof(qty) = 'integer' AND qty > 0)
) WITHOUT ROWID;

-- modifiers has no (venue_id, id) key to point at, so the venue is checked
-- by the service; the id alone is the foreign key.
CREATE TABLE modifier_recipes (
    venue_id      INTEGER NOT NULL,
    modifier_id   INTEGER NOT NULL REFERENCES modifiers (id) ON DELETE CASCADE,
    stock_item_id INTEGER NOT NULL,
    qty           INTEGER NOT NULL,

    PRIMARY KEY (modifier_id, stock_item_id),
    FOREIGN KEY (venue_id, stock_item_id) REFERENCES stock_items (venue_id, id),
    CHECK (typeof(qty) = 'integer' AND qty > 0)
) WITHOUT ROWID;

CREATE INDEX item_recipes_venue ON item_recipes (venue_id);
CREATE INDEX modifier_recipes_venue ON modifier_recipes (venue_id);

-- A close reads what an order has already taken from stock.
CREATE INDEX stock_movements_order ON stock_movements (venue_id, order_id) WHERE order_id != 0;

-- +goose Down
DROP INDEX stock_movements_order;
DROP INDEX modifier_recipes_venue;
DROP INDEX item_recipes_venue;
DROP TABLE modifier_recipes;
DROP TABLE item_recipes;

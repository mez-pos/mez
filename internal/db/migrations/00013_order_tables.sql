-- +goose Up
-- Every table an order sits at (phase 08, Task 2c, ADR-088): its own, and
-- any joined to it for one party. An index over the projection, kept in step
-- by the transaction that appends the events, like the orders row. It
-- carries the order's closed_at so the floor's two questions about a table,
-- "is an order still open here" and "when did the last one close", are each
-- one index lookup however many orders the table has seen.
CREATE TABLE order_tables (
    order_id  INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    table_id  INTEGER NOT NULL,
    closed_at TIMESTAMP,
    PRIMARY KEY (order_id, table_id),
    CHECK (typeof(table_id) = 'integer' AND table_id > 0),
    CHECK (closed_at IS NULL OR closed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
) WITHOUT ROWID;

CREATE INDEX order_tables_table ON order_tables (table_id, closed_at);

INSERT INTO order_tables (order_id, table_id, closed_at)
SELECT id, table_id, closed_at FROM orders WHERE table_id > 0;

-- The floor reads order_tables now; 00012's index on orders is not used.
DROP INDEX orders_table_closed;

-- +goose Down
CREATE INDEX orders_table_closed ON orders (venue_id, table_id, closed_at);
DROP INDEX order_tables_table;
DROP TABLE order_tables;

-- +goose Up
-- Where a table is in its turn (phase 08, Task 2a, ADR-086). Free, seated,
-- ordered and settling come from the orders at the table. Dirty is the one
-- thing orders cannot say on their own: that the table's last order closed
-- and nobody has cleared it since. cleared_at is when it last was.

ALTER TABLE tables ADD COLUMN cleared_at TIMESTAMP
    CHECK (cleared_at IS NULL OR cleared_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00');

-- The floor's read: the last order to close at each table.
CREATE INDEX orders_table_closed ON orders (venue_id, table_id, closed_at);

-- A counter per venue that moves on every change the orders' own events do
-- not carry: the plan edited, a table cleared. The stream compares it, as it
-- does the print version, to tell terminals to read the floor again.
CREATE TABLE floor_versions (
    venue_id INTEGER PRIMARY KEY REFERENCES venues (id) ON DELETE CASCADE,
    version  INTEGER NOT NULL,
    CHECK (version > 0)
);

-- +goose Down
DROP TABLE floor_versions;
DROP INDEX orders_table_closed;
ALTER TABLE tables DROP COLUMN cleared_at;

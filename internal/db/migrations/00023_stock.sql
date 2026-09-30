-- +goose Up
-- Stock (phase 10, Task 3a, ADR-111): what the venue keeps, and a ledger of
-- every movement of it. On hand is the sum of the ledger's deltas; it is
-- never stored, so it cannot drift from the history that explains it.
--
-- Quantities are whole thousandths of the item's unit: 18.5 g is 18500.

-- stock.manage is new. Venues set up before it have owner and manager
-- roles without it; grant it to those two where they still have their
-- default names, once, as 00007 did for invoice.reprint.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'stock.manage')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'stock.manage');

CREATE TABLE stock_items (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    unit       TEXT    NOT NULL,
    low_level  INTEGER NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, id),
    CHECK (length(name) > 0),
    CHECK (unit IN ('g', 'ml', 'each')),
    CHECK (typeof(low_level) = 'integer' AND low_level >= 0),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- Two items with one name would be counted as one and recorded as two.
CREATE UNIQUE INDEX stock_items_venue_name ON stock_items (venue_id, name COLLATE NOCASE);

-- One row per movement, never changed. delta is what it did to on hand;
-- counted is what a count found (null for anything else); reason is a
-- waste's reason code. shift_id is the shift open when it was recorded, 0
-- for none; order_id the order a sale came from (Task 3b), 0 otherwise.
CREATE TABLE stock_movements (
    id            INTEGER PRIMARY KEY,
    venue_id      INTEGER NOT NULL,
    stock_item_id INTEGER NOT NULL,
    kind          TEXT    NOT NULL,
    delta         INTEGER NOT NULL,
    counted       INTEGER,
    reason        TEXT    NOT NULL DEFAULT '',
    note          TEXT    NOT NULL DEFAULT '',
    staff_id      INTEGER NOT NULL REFERENCES staff (id),
    shift_id      INTEGER NOT NULL DEFAULT 0,
    order_id      INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL,

    FOREIGN KEY (venue_id, stock_item_id) REFERENCES stock_items (venue_id, id),
    CHECK (kind IN ('received', 'adjusted', 'wasted', 'counted', 'sold')),
    CHECK (typeof(delta) = 'integer'),
    CHECK (counted IS NULL OR (typeof(counted) = 'integer' AND counted >= 0)),
    CHECK ((kind = 'counted') = (counted IS NOT NULL)),
    CHECK ((kind = 'wasted') = (reason != '')),
    CHECK (reason IN ('', 'expired', 'spoiled', 'spilled', 'dropped', 'other')),
    CHECK (typeof(shift_id) = 'integer' AND shift_id >= 0),
    CHECK (typeof(order_id) = 'integer' AND order_id >= 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE INDEX stock_movements_item ON stock_movements (venue_id, stock_item_id, id);

-- +goose StatementBegin
CREATE TRIGGER stock_movements_no_update BEFORE UPDATE ON stock_movements
BEGIN
    SELECT RAISE(ABORT, 'a stock movement is written once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER stock_movements_no_delete BEFORE DELETE ON stock_movements
BEGIN
    SELECT RAISE(ABORT, 'a stock movement is written once and never changed');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER stock_movements_no_delete;
DROP TRIGGER stock_movements_no_update;
DROP INDEX stock_movements_item;
DROP TABLE stock_movements;
DROP INDEX stock_items_venue_name;
DROP TABLE stock_items;
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions) WHERE value != 'stock.manage'
)
WHERE name IN ('owner', 'manager');

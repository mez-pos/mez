-- +goose Up
-- Orders and their events belong to shifts (phase 09, Task 1b, ADR-094).
--
-- orders.shift_id is the shift the order was opened in. Every order event
-- carries the shift open when it was recorded, so a tab opened in one shift
-- and paid in the next reports its payment in the second. 0 is no shift:
-- the orders and events written before shifts existed, and the few events
-- allowed after close (shift.NeedsShift). Not foreign keys, as table_id is
-- not: ALTER TABLE cannot add one with a non-null default.

ALTER TABLE orders ADD COLUMN shift_id INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(shift_id) = 'integer' AND shift_id >= 0);

ALTER TABLE order_events ADD COLUMN shift_id INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(shift_id) = 'integer' AND shift_id >= 0);

-- The Z-report's read: everything that happened in a shift.
CREATE INDEX order_events_shift ON order_events (shift_id, id);
CREATE INDEX orders_shift ON orders (shift_id);

-- shift.manage (open and close the shift) and cash.move (authorise cash in
-- and out of the drawer) arrive here. Grant both to owner and manager
-- where they keep their default names, once, as 00007 did for reprints.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'shift.manage')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'shift.manage');

UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'cash.move')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'cash.move');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value NOT IN ('shift.manage', 'cash.move')
)
WHERE name IN ('owner', 'manager');
DROP INDEX orders_shift;
DROP INDEX order_events_shift;
ALTER TABLE order_events DROP COLUMN shift_id;
ALTER TABLE orders DROP COLUMN shift_id;

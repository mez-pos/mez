-- +goose Up
-- kitchen.work arrives (ADR-180): starting a ticket, marking it ready and
-- recalling it is its own permission. Until now a person on a terminal could
-- not move tickets at all; since ADR-179 kitchen.view let them, and every
-- seller holds kitchen.view. It goes back to meaning what it says: see the
-- board.
--
-- The owner and the manager, where they keep their default names, as 00016
-- did for the shift's permissions.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'kitchen.work')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'kitchen.work');

-- And every role that sees the kitchen and does not take orders, whatever
-- it is called: the kitchen role, and a "Head chef" the owner made. Those
-- are the people the board is the work of (they land on it, ADR-179). A
-- role that sells keeps kitchen.view alone.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'kitchen.work')
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'kitchen.view')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'order.open')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'kitchen.work');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value <> 'kitchen.work'
)
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'kitchen.work');

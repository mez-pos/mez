-- +goose Up
-- shift.open arrives (ADR-152): counting the float in and opening the shift
-- is its own permission, so a cashier can start the day without also being
-- able to close it. Until now shift.manage covered both.
--
-- Every role that could open keeps the ability: whoever holds shift.manage
-- gains shift.open, whatever the role is called. A role without it (a cashier
-- the owner trusted with the whole shift keeps both) is not otherwise touched.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'shift.open')
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'shift.manage')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'shift.open');

-- The cashier, where the role kept its default name, once, as 00016 did for
-- the owner and the manager.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'shift.open')
WHERE name = 'cashier'
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'shift.open');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value <> 'shift.open'
)
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'shift.open');

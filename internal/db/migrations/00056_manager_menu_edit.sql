-- +goose Up
-- A manager may edit the menu (ADR-342): mark what is sold out, correct a
-- price. The flow map always gave a manager Menu, and the role a venue
-- starts with did not hold menu.edit, so the pill was missing and the back
-- office refused them.
--
-- The manager where it keeps its default name, as 00053 did for
-- tip.approve. An owner who wants prices kept from managers takes it away
-- under People, Roles. Settings and devices stay the owner's.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'menu.edit')
WHERE name = 'manager'
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'menu.edit');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value <> 'menu.edit'
)
WHERE name = 'manager'
  AND EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'menu.edit');

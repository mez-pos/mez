-- +goose Up
-- invoice.reprint arrived in phase 07, Task 3 (ADR-078). Venues set up
-- before it have owner and manager roles without it, and an owner with no
-- reprint could never give one out. Grant it to those two roles where they
-- still have their default names, once: a role an owner renamed or trimmed
-- keeps what the owner chose.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'invoice.reprint')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'invoice.reprint');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions) WHERE value != 'invoice.reprint'
)
WHERE name IN ('owner', 'manager');

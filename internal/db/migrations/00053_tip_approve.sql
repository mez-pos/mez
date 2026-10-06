-- +goose Up
-- tip.approve arrives (ADR-261): a tip over the venue's ceiling, a percent
-- of its payment, needs someone who holds it. Without this a venue that
-- exists would have nobody to approve one, and its owner would no longer
-- hold every permission.
--
-- The owner and the manager, where they keep their default names, as 00034
-- did for kitchen.work.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'tip.approve')
WHERE name IN ('owner', 'manager')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'tip.approve');

-- And every role that may void a payment, whatever it is called: whoever
-- may take money back off a bill may let a large tip onto one.
UPDATE roles
SET permissions = json_insert(permissions, '$[#]', 'tip.approve')
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'payment.void')
  AND NOT EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'tip.approve');

-- +goose Down
UPDATE roles
SET permissions = (
    SELECT json_group_array(value) FROM json_each(roles.permissions)
    WHERE value <> 'tip.approve'
)
WHERE EXISTS (SELECT 1 FROM json_each(roles.permissions) WHERE value = 'tip.approve');

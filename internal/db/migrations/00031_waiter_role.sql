-- +goose Up
-- The waiter role arrived in the rebuild (ADR-145): a tablet signs in, takes
-- orders, sends them and settles the bill, and cannot refund, void or touch
-- the shift. New venues get it from auth.DefaultRoles; this gives it, once, to
-- venues that already exist. A venue that made a role called "waiter" itself,
-- in any case, keeps that one and gets no second. The permissions are the same
-- list as auth.DefaultRoles (TestDefaultRoles and the migration's own test
-- each hold a copy).
INSERT INTO roles (venue_id, name, permissions, created_at)
SELECT v.id, 'waiter', '["order.open","order.edit","payment.take","kitchen.view"]', strftime('%Y-%m-%d %H:%M:%S+00:00', 'now')
FROM venues v
WHERE NOT EXISTS (SELECT 1 FROM roles r WHERE r.venue_id = v.id AND lower(r.name) = 'waiter');

-- +goose Down
-- A waiter role in use cannot be removed (staff.role_id references it), so
-- only the ones nobody holds are dropped.
DELETE FROM roles
WHERE name = 'waiter'
  AND permissions = '["order.open","order.edit","payment.take","kitchen.view"]'
  AND NOT EXISTS (SELECT 1 FROM staff s WHERE s.role_id = roles.id);

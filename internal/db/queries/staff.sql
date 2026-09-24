-- Staff and the roles they hold. Roles are here rather than in a file of their
-- own because a staff member cannot be created without one.

-- name: CreateRole :one
INSERT INTO roles (venue_id, name, permissions, created_at)
VALUES (?, ?, ?, ?)
RETURNING id, venue_id, name, permissions, created_at;

-- name: RolesForVenue :many
SELECT id, venue_id, name, permissions, created_at
FROM roles
WHERE venue_id = ?
ORDER BY name;

-- name: CreateStaff :one
INSERT INTO staff (
    public_id,
    venue_id,
    role_id,
    name,
    pin_hash,
    active,
    created_at,
    updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
    id, public_id, venue_id, role_id, name, pin_hash, active,
    created_at, updated_at;

-- name: StaffByPublicID :one
SELECT
    id, public_id, venue_id, role_id, name, pin_hash, active,
    created_at, updated_at
FROM staff
WHERE public_id = ?;

-- name: ActiveStaffForVenue :many
--
-- The index on (venue_id, active) is what this reads down, which is why it
-- exists.
SELECT
    id, public_id, venue_id, role_id, name, pin_hash, active,
    created_at, updated_at
FROM staff
WHERE venue_id = ? AND active = 1
ORDER BY name;

-- name: DeactivateStaff :exec
--
-- Staff are deactivated, never deleted: their name is on every order they took
-- and on every void they authorised, and a tax audit expects to find them.
UPDATE staff
SET active     = 0,
    updated_at = ?
WHERE id = ?;

-- name: RoleByID :one
SELECT id, venue_id, name, permissions, created_at
FROM roles
WHERE id = ?;

-- name: StaffNamesForVenue :many
--
-- Everyone who has worked at the venue, inactive included: an order's
-- history names the people on it, whether or not they still work there.
SELECT id, name
FROM staff
WHERE venue_id = ?
ORDER BY id;

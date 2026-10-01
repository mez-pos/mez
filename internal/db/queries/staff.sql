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

-- The People screen (phase 10, Task 4a, ADR-115). Every query is scoped by
-- venue_id: an id from another venue matches nothing.

-- name: StaffForVenue :many
--
-- Everyone, inactive too, with their role's name.
SELECT s.id, s.public_id, s.name, s.role_id, r.name AS role_name, s.active, s.created_at
FROM staff s
JOIN roles r ON r.id = s.role_id
WHERE s.venue_id = ?
ORDER BY s.active DESC, s.name COLLATE NOCASE, s.id;

-- name: StaffForEdit :one
SELECT id, role_id, active
FROM staff
WHERE venue_id = ? AND public_id = ?;

-- name: StaffRoleOf :one
SELECT role_id FROM staff WHERE venue_id = ? AND id = ?;

-- name: UpdateStaff :execrows
UPDATE staff
SET name = sqlc.arg(name), role_id = sqlc.arg(role_id), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStaffActive :execrows
UPDATE staff
SET active = sqlc.arg(active), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: SetStaffPIN :execrows
UPDATE staff
SET pin_hash = sqlc.arg(pin_hash), updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: EndSessionsOfStaff :exec
--
-- A new PIN signs the person out everywhere: whoever knew the old one
-- should not stay signed in with it.
UPDATE staff_sessions
SET ended_at = sqlc.arg(now)
WHERE staff_id = sqlc.arg(staff_id) AND ended_at IS NULL;

-- name: RoleForEdit :one
SELECT id, name, permissions
FROM roles
WHERE venue_id = ? AND id = ?;

-- name: UpdateRole :execrows
UPDATE roles
SET name = sqlc.arg(name), permissions = sqlc.arg(permissions)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: DeleteRole :execrows
DELETE FROM roles
WHERE roles.venue_id = sqlc.arg(venue_id) AND roles.id = sqlc.arg(id)
  AND NOT EXISTS (SELECT 1 FROM staff WHERE staff.role_id = roles.id);

-- name: StaffCountByRole :many
SELECT role_id, COUNT(*) AS n
FROM staff
WHERE venue_id = ? AND active = 1
GROUP BY role_id;

-- name: StaffManagersLeft :one
--
-- How many active people can still manage staff. Every change to people
-- or roles checks this inside its own transaction and is refused at zero:
-- a venue nobody can manage cannot be repaired from the back office.
SELECT COUNT(*) AS n
FROM staff s
JOIN roles r ON r.id = s.role_id
WHERE s.venue_id = ? AND s.active = 1
  AND EXISTS (SELECT 1 FROM json_each(r.permissions) WHERE value = 'staff.manage');

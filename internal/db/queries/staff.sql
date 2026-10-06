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
    created_at, updated_at, locked_at, locked_reason;

-- name: StaffByPublicID :one
SELECT
    id, public_id, venue_id, role_id, name, pin_hash, active,
    created_at, updated_at, locked_at, locked_reason
FROM staff
WHERE public_id = ?;

-- name: ActiveStaffForVenue :many
--
-- The index on (venue_id, active) is what this reads down, which is why it
-- exists. The role's name rides along for the sign-in screen. Someone
-- hard-locked is left out, as someone deactivated is (ADR-260): their tile
-- would only take PINs it refuses.
SELECT
    s.id, s.public_id, s.venue_id, s.role_id, r.name AS role_name, s.name, s.pin_hash, s.active,
    s.created_at, s.updated_at
FROM staff s
JOIN roles r ON r.id = s.role_id
WHERE s.venue_id = ? AND s.active = 1 AND s.locked_at IS NULL
ORDER BY s.name;

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
SELECT s.id, s.public_id, s.name, s.role_id, r.name AS role_name, s.active, s.created_at, s.locked_at
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

-- The hard lock (ADR-260). A lockout is recorded at the wrong PIN that locks
-- a person for the window; the third in a day locks them until someone who
-- can manage staff clears it.

-- name: RecordStaffLockout :exec
INSERT INTO staff_lockouts (venue_id, staff_id, at)
VALUES (?, ?, ?);

-- name: CountStaffLockoutsSince :one
--
-- The lockouts of a person since a moment that no manager has cleared.
SELECT COUNT(*) AS n
FROM staff_lockouts
WHERE staff_id = sqlc.arg(staff_id) AND at > sqlc.arg(since) AND cleared_at IS NULL;

-- name: LastStaffLockClear :one
--
-- When a manager last cleared a person's lock. The wrong PINs before it no
-- longer count toward a lockout.
SELECT cleared_at
FROM staff_lockouts
WHERE staff_id = ? AND cleared_at IS NOT NULL
ORDER BY cleared_at DESC
LIMIT 1;

-- name: SetStaffHardLock :execrows
UPDATE staff
SET locked_at = sqlc.arg(locked_at), locked_reason = sqlc.arg(reason), updated_at = sqlc.arg(locked_at)
WHERE id = sqlc.arg(id) AND locked_at IS NULL;

-- name: ClearStaffHardLock :execrows
UPDATE staff
SET locked_at = NULL, locked_reason = '', updated_at = sqlc.arg(now)
WHERE venue_id = sqlc.arg(venue_id) AND id = sqlc.arg(id);

-- name: ClearStaffLockouts :exec
UPDATE staff_lockouts
SET cleared_at = sqlc.arg(now)
WHERE staff_id = sqlc.arg(staff_id) AND cleared_at IS NULL;

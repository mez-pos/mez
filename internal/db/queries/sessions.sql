-- Staff sessions and sign-in attempts. internal/auth is the only caller.

-- name: RecordSignInAttempt :exec
INSERT INTO sign_in_attempts (venue_id, device_id, staff_id, staff_ref, at, outcome)
VALUES (?, ?, ?, ?, ?, ?);

-- name: RecentStaffVerdicts :many
-- The staff member's latest attempts that verified a PIN, newest first:
-- successes and wrong PINs. Refusals during a lockout are left out, or a
-- hammering client would push the failures that caused it out of the window.
SELECT at, outcome
FROM sign_in_attempts
WHERE staff_id = ? AND outcome IN ('success', 'invalid_pin')
ORDER BY id DESC
LIMIT ?;

-- name: RecentDeviceFailures :many
-- The device's latest failed attempts, of any staff member, newest first.
SELECT at
FROM sign_in_attempts
WHERE device_id = ? AND outcome IN ('invalid_pin', 'unknown_staff', 'inactive_staff')
ORDER BY id DESC
LIMIT ?;

-- name: CreateStaffSession :one
INSERT INTO staff_sessions (token_hash, venue_id, staff_id, device_id, created_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: StaffSessionByTokenHash :one
SELECT
    s.id, s.venue_id, s.staff_id, s.device_id, s.expires_at, s.ended_at,
    st.public_id AS staff_public_id, st.name AS staff_name, st.role_id, st.active AS staff_active
FROM staff_sessions s
JOIN staff st ON st.id = s.staff_id
WHERE s.token_hash = ?;

-- name: EndStaffSession :execrows
UPDATE staff_sessions
SET ended_at = ?
WHERE id = ? AND ended_at IS NULL;

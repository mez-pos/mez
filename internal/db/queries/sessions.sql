-- Staff sessions and sign-in attempts. internal/auth is the only caller.

-- name: RecordSignInAttempt :exec
INSERT INTO sign_in_attempts (venue_id, device_id, staff_id, staff_ref, at, outcome, via)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: RecentStaffVerdicts :many
-- The staff member's latest attempts that verified a PIN, newest first:
-- successes and wrong PINs. Refusals during a lockout are left out, or a
-- hammering client would push the failures that caused it out of the window.
-- Only those that arrived one way: the venue's attempts and the ones from
-- away are counted apart (ADR-314).
SELECT at, outcome
FROM sign_in_attempts
WHERE staff_id = ? AND via = ? AND outcome IN ('success', 'invalid_pin')
ORDER BY id DESC
LIMIT ?;

-- name: RecentDeviceFailures :many
-- The device's latest failed attempts, of any staff member, newest first,
-- however they arrived.
SELECT at
FROM sign_in_attempts
WHERE device_id = ? AND outcome IN ('invalid_pin', 'unknown_staff', 'inactive_staff')
ORDER BY id DESC
LIMIT ?;

-- name: CreateStaffSession :one
INSERT INTO staff_sessions (token_hash, venue_id, staff_id, device_id, created_at, expires_at, via)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: StaffSessionByTokenHash :one
SELECT
    s.id, s.venue_id, s.staff_id, s.device_id, s.expires_at, s.ended_at, s.via,
    st.public_id AS staff_public_id, st.name AS staff_name, st.role_id, r.name AS role_name, st.active AS staff_active,
    st.cloud AS staff_cloud, d.revoked_at AS device_revoked_at
FROM staff_sessions s
JOIN staff st ON st.id = s.staff_id
JOIN roles r ON r.id = st.role_id
JOIN devices d ON d.id = s.device_id
WHERE s.token_hash = ?;

-- name: EndStaffSession :execrows
UPDATE staff_sessions
SET ended_at = ?
WHERE id = ? AND ended_at IS NULL;

-- name: PruneSignInAttempts :execrows
-- Drops attempts older than a cutoff. The lockouts read the latest few rows
-- only, so nothing a month old decides anything, and the table would
-- otherwise grow by every PIN ever typed.
DELETE FROM sign_in_attempts
WHERE at < ?;

-- Assertions mez Cloud signed, spent here (ADR-323).

-- name: SpendCloudAssertion :execrows
-- One row changed is an assertion seen for the first time; none is one that
-- has been spent already.
INSERT INTO cloud_assertions (nonce, venue_id, expires_at)
VALUES (?, ?, ?)
ON CONFLICT (nonce) DO NOTHING;

-- name: PruneCloudAssertions :exec
-- An assertion past this moment is refused for its age, so its nonce no
-- longer needs remembering.
DELETE FROM cloud_assertions
WHERE expires_at <= ?;

-- name: CloudSessionDevice :one
-- The cloud's device, found by a session opened on it (ADR-324): it has no
-- token of its own. Only a session of the cloud's owner on the cloud's
-- device that has not ended, with neither revoked nor deactivated. Whether
-- it has expired is the caller's to judge, by its own clock.
SELECT d.id, d.public_id, d.venue_id, d.name, d.kind, d.last_seen_at, s.expires_at
FROM staff_sessions s
JOIN staff st ON st.id = s.staff_id
JOIN devices d ON d.id = s.device_id
WHERE s.token_hash = ?
  AND s.ended_at IS NULL AND s.via = 'relay'
  AND st.cloud = 1 AND st.active = 1
  AND d.cloud = 1 AND d.revoked_at IS NULL;

-- name: EndCloudSessions :exec
-- Every session mez Cloud opened, on any venue here: for a machine that no
-- longer runs on a pairing (ADR-324).
UPDATE staff_sessions
SET ended_at = sqlc.arg(now)
WHERE ended_at IS NULL
  AND staff_id IN (SELECT id FROM staff WHERE cloud = 1);

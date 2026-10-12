-- Devices and the codes they pair with. internal/auth is the only caller.

-- name: CreatePairingCode :one
INSERT INTO pairing_codes (venue_id, code_hash, device_name, device_kind, created_by, created_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: PairingCodeByHash :one
SELECT id, venue_id, code_hash, device_name, device_kind, created_by,
       created_at, expires_at, redeemed_at, device_id
FROM pairing_codes
WHERE code_hash = ?;

-- name: RedeemPairingCode :execrows
-- Only an unredeemed code changes, so two redeemers cannot both succeed.
UPDATE pairing_codes
SET redeemed_at = ?, device_id = ?
WHERE id = ? AND redeemed_at IS NULL;

-- name: CreatePairedDevice :one
INSERT INTO devices (public_id, venue_id, name, kind, token_hash, paired_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: DeviceByTokenHash :one
SELECT id, public_id, venue_id, name, kind, token_hash, paired_at, last_seen_at, revoked_at, created_at, cloud
FROM devices
WHERE token_hash = ? AND token_hash != '';

-- name: DeviceByID :one
SELECT id, public_id, venue_id, name, kind, token_hash, paired_at, last_seen_at, revoked_at, created_at, cloud
FROM devices
WHERE id = ?;

-- name: TouchDevice :exec
UPDATE devices
SET last_seen_at = ?
WHERE id = ?;

-- name: RevokeDevice :execrows
-- Revoked rather than deleted, and the token hash kept, so a revoked token is
-- recognised and refused as revoked rather than as unknown.
UPDATE devices
SET revoked_at = ?
WHERE id = ? AND venue_id = ? AND revoked_at IS NULL;

-- name: DeviceNamesForVenue :many
--
-- Every device ever paired at the venue, revoked included, for the same
-- reason: history names the till an event came from.
SELECT id, name
FROM devices
WHERE venue_id = ?
ORDER BY id;

-- The back office's devices screen (phase 10, Task 6k). Every one is scoped by
-- venue_id: an id from another venue matches nothing.

-- name: DevicesForVenue :many
-- Every paired device, revoked ones last: the screen lists who is in service
-- and who was. A guest session is not a device, and a row with no pairing is
-- not one either.
SELECT id, public_id, name, kind, paired_at, last_seen_at, revoked_at
FROM devices
WHERE venue_id = ? AND kind != 'guest' AND paired_at IS NOT NULL
ORDER BY (revoked_at IS NOT NULL), id;

-- name: DeviceByPublicID :one
SELECT id, public_id, venue_id, name, kind, paired_at, last_seen_at, revoked_at
FROM devices
WHERE venue_id = ? AND public_id = ?;

-- name: RenameDevice :execrows
-- A revoked device keeps the name its history knows it by.
UPDATE devices
SET name = ?
WHERE venue_id = ? AND id = ? AND revoked_at IS NULL;

-- name: OpenPairingCodes :many
-- Codes made and not yet used or expired, without the codes themselves: only
-- their hashes are kept. The screen says a device is waiting to be paired.
SELECT device_name, device_kind, created_at, expires_at
FROM pairing_codes
WHERE venue_id = ? AND redeemed_at IS NULL AND expires_at > ?
ORDER BY id;

-- The cloud's device (ADR-323): what the sessions of the cloud's owner
-- belong to. It has no token, so DeviceByTokenHash never finds it.

-- name: CloudDeviceForVenue :one
SELECT id, public_id, venue_id, name, kind, token_hash, paired_at, last_seen_at, revoked_at, created_at, cloud
FROM devices
WHERE venue_id = ? AND cloud = 1 AND revoked_at IS NULL;

-- name: CreateCloudDevice :one
INSERT INTO devices (public_id, venue_id, name, kind, token_hash, paired_at, created_at, cloud)
VALUES (sqlc.arg(public_id), sqlc.arg(venue_id), sqlc.arg(name), 'back_office', '', sqlc.arg(now), sqlc.arg(now), 1)
RETURNING id;

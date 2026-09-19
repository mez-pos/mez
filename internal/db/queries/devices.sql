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
SELECT id, public_id, venue_id, name, kind, token_hash, paired_at, last_seen_at, revoked_at, created_at
FROM devices
WHERE token_hash = ? AND token_hash != '';

-- name: DeviceByID :one
SELECT id, public_id, venue_id, name, kind, token_hash, paired_at, last_seen_at, revoked_at, created_at
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

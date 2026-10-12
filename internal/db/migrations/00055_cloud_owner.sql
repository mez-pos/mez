-- +goose Up
-- The owner signs in from mez Cloud (ADR-323).
--
-- staff.cloud marks the one person a venue's pairing made: named from the
-- account, with no PIN (pin_hash is the word 'none', which is no hash), who
-- is signed in only by an assertion the cloud signs. One to a venue.
--
-- devices.cloud marks the device that person's sessions belong to: a back
-- office with no token, so nothing can present itself as it. Revoked, a
-- pairing made again gives the venue a new one.
--
-- cloud_assertions is every assertion spent and not yet dead, by its nonce:
-- one is good once. A row is dropped when its assertion could no longer be
-- accepted anyway.

ALTER TABLE staff ADD COLUMN cloud BOOLEAN NOT NULL DEFAULT 0
    CHECK (cloud IN (0, 1));

CREATE UNIQUE INDEX staff_cloud ON staff (venue_id) WHERE cloud = 1;

ALTER TABLE devices ADD COLUMN cloud BOOLEAN NOT NULL DEFAULT 0
    CHECK (cloud IN (0, 1));

CREATE UNIQUE INDEX devices_cloud ON devices (venue_id) WHERE cloud = 1 AND revoked_at IS NULL;

CREATE TABLE cloud_assertions (
    nonce      TEXT    NOT NULL PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,

    CHECK (length(nonce) BETWEEN 16 AND 64),
    CHECK (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
) WITHOUT ROWID;

-- +goose Down
DROP TABLE cloud_assertions;
DROP INDEX devices_cloud;
ALTER TABLE devices DROP COLUMN cloud;
DROP INDEX staff_cloud;
ALTER TABLE staff DROP COLUMN cloud;

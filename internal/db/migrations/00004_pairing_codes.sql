-- Device pairing. The back office makes a short code; a new terminal, kitchen
-- screen or back office browser redeems it once for a long-lived device
-- token, and every request after that carries the token.
--
-- Neither the code nor the token is stored, only its SHA-256. A token is 256
-- random bits, so a fast hash loses nothing; a code is 40 bits, lives ten
-- minutes and works once, so a stolen database is no use for one either.

-- +goose Up

CREATE TABLE pairing_codes (
    id          INTEGER PRIMARY KEY,
    venue_id    INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    code_hash   TEXT    NOT NULL,

    -- What the device will be called and what it is, decided by whoever
    -- made the code, so a terminal cannot pair itself as a back office.
    device_name TEXT    NOT NULL,
    device_kind TEXT    NOT NULL,

    -- The staff member who made the code. NULL for the first device of a
    -- venue, paired from the command line before any staff exist.
    created_by  INTEGER REFERENCES staff (id) ON DELETE SET NULL,

    created_at  TIMESTAMP NOT NULL,
    expires_at  TIMESTAMP NOT NULL,

    -- Set together, once, by the redeeming transaction.
    redeemed_at TIMESTAMP,
    device_id   INTEGER REFERENCES devices (id) ON DELETE SET NULL,

    CHECK (length(code_hash) = 64),
    CHECK (length(device_name) > 0),
    CHECK (device_kind IN ('terminal', 'kitchen', 'back_office', 'guest')),
    CHECK (created_at  GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (expires_at  GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (redeemed_at IS NULL OR redeemed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    -- A device only ever comes from a redeemed code. The device may later be
    -- deleted, which clears device_id and leaves the redemption on record.
    CHECK (device_id IS NULL OR redeemed_at IS NOT NULL)
);

CREATE UNIQUE INDEX pairing_codes_code_hash ON pairing_codes (code_hash);

-- A device is found by its token on every request.
CREATE UNIQUE INDEX devices_token_hash ON devices (token_hash) WHERE token_hash != '';

-- +goose Down

DROP INDEX devices_token_hash;
DROP TABLE pairing_codes;

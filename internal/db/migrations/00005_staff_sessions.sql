-- Staff sign-in: the sessions a PIN opens, and a record of every attempt.
--
-- A staff session belongs to one device. Its token is sent alongside the
-- device's own, and is refused from any other device, so a token read off
-- one terminal's screen is no use at another.

-- +goose Up

CREATE TABLE staff_sessions (
    id         INTEGER PRIMARY KEY,
    token_hash TEXT    NOT NULL,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    staff_id   INTEGER NOT NULL REFERENCES staff (id) ON DELETE CASCADE,
    device_id  INTEGER NOT NULL REFERENCES devices (id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,

    -- Set when the staff member signs out. An ended session is kept, so the
    -- record of who was signed in where, and when, survives.
    ended_at   TIMESTAMP,

    CHECK (length(token_hash) = 64),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (ended_at IS NULL OR ended_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX staff_sessions_token_hash ON staff_sessions (token_hash);

-- Every sign-in attempt, whatever came of it. This is what the lockout is
-- computed from, and what an owner reads when a PIN has been tried forty
-- times at 3am.
CREATE TABLE sign_in_attempts (
    id        INTEGER PRIMARY KEY,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    device_id INTEGER NOT NULL REFERENCES devices (id) ON DELETE CASCADE,

    -- The staff member, when the ID given was one. staff_ref is what was
    -- given, kept even when it matched nobody.
    staff_id  INTEGER REFERENCES staff (id) ON DELETE SET NULL,
    staff_ref TEXT    NOT NULL,

    at        TIMESTAMP NOT NULL,
    outcome   TEXT    NOT NULL,

    CHECK (length(staff_ref) <= 64),
    CHECK (outcome IN ('success', 'invalid_pin', 'unknown_staff', 'inactive_staff', 'locked_out')),
    CHECK (at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE INDEX sign_in_attempts_staff ON sign_in_attempts (staff_id, id);
CREATE INDEX sign_in_attempts_device ON sign_in_attempts (device_id, id);

-- +goose Down

DROP TABLE sign_in_attempts;
DROP TABLE staff_sessions;

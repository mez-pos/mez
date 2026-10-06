-- +goose Up
-- A PIN that keeps failing locks until a manager clears it (ADR-260).
--
-- locked_at is when the hard lock fell, NULL when there is none. A person
-- with one is refused at sign-in and at an override as a deactivated person
-- is, whatever PIN they type, until someone who can manage staff clears it.
-- locked_reason says why, for the People screen and the log.

ALTER TABLE staff ADD COLUMN locked_at TIMESTAMP
    CHECK (locked_at IS NULL OR locked_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00');

ALTER TABLE staff ADD COLUMN locked_reason TEXT NOT NULL DEFAULT ''
    CHECK (length(locked_reason) <= 200);

-- Every lockout: the wrong PIN after which a person was locked for the
-- window. The hard lock counts these. They are not derived from
-- sign_in_attempts because which failure completed a window is a fact of
-- the moment (it depends on the policy then in force and on the last
-- success), and recomputing it from a day of attempts would replay the
-- sliding window in SQL. cleared_at is set when a manager clears the hard
-- lock, so the lockouts before it count no longer, and the wrong PINs
-- before it no longer hold a lock either.
CREATE TABLE staff_lockouts (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    staff_id   INTEGER NOT NULL REFERENCES staff (id) ON DELETE CASCADE,
    at         TIMESTAMP NOT NULL,
    cleared_at TIMESTAMP,

    CHECK (at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (cleared_at IS NULL OR cleared_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE INDEX staff_lockouts_staff ON staff_lockouts (staff_id, at);

-- +goose Down
DROP TABLE staff_lockouts;
ALTER TABLE staff DROP COLUMN locked_reason;
ALTER TABLE staff DROP COLUMN locked_at;

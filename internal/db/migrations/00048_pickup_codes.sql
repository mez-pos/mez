-- +goose Up
-- A venue's pickup code (phase 11, Task 6e, ADR-237, ADR-247): a code that
-- leads to the venue and to no table, for a guest ordering to collect. It is
-- a table of its own and not a row of table_codes, whose every row is a
-- table's: a code with no table there would be a table's code with a hole in
-- it, and every reader of that table would have to remember the hole.
--
-- A venue has at most one live code. A revoked one stays, so that it is
-- never drawn again and a poster still on a door leads nowhere for good.
CREATE TABLE pickup_codes (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    code       TEXT    NOT NULL,

    created_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,

    UNIQUE (code),
    CHECK (length(code) >= 10),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (revoked_at IS NULL OR revoked_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX pickup_codes_live ON pickup_codes (venue_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP INDEX pickup_codes_live;
DROP TABLE pickup_codes;

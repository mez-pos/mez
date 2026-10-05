-- +goose Up
-- A table's code is what its QR sticker says (phase 11, ADR-206): the guest's
-- phone presents it, and it resolves to the venue and the table. It is the
-- guest's only credential, so it is drawn at random and never derived from
-- the table, and it can be turned off without touching the table: a sticker
-- that walked out of the door is revoked and the table gets another.
--
-- The code is kept as it is, not hashed. Whoever can read this database can
-- read the menu it leads to, and the back office has to print it again.
CREATE TABLE table_codes (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id   INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    code       TEXT    NOT NULL,

    created_at TIMESTAMP NOT NULL,
    -- Set when the code is turned off. A revoked code stays, so that it is
    -- never drawn again and a sticker still on a wall leads nowhere for good.
    revoked_at TIMESTAMP,

    UNIQUE (code),
    CHECK (length(code) >= 10),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (revoked_at IS NULL OR revoked_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- "This table's live code", which is at most one row: the service revokes
-- the old one in the transaction that makes the new.
CREATE INDEX table_codes_table ON table_codes (table_id, revoked_at);

-- +goose Down
DROP INDEX table_codes_table;
DROP TABLE table_codes;

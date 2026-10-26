-- +goose Up
-- The cash a shift's sales left in the drawer, kept as it happens (ADR-378):
-- taken by cash tenders (bill and tip), handed back by their voids, and
-- refunded against them. Reading a shift used to replay every order of the
-- day to find these three numbers; now each cash event adds to its shift's
-- row in the transaction that records it, and the close still replays the
-- day once and says so if the two disagree. A shift with no row yet (one
-- open when this arrived, or one with no cash) reads as before.
CREATE TABLE shift_cash (
    shift_id       INTEGER PRIMARY KEY REFERENCES shifts (id) ON DELETE CASCADE,
    currency       TEXT    NOT NULL,
    taken_minor    INTEGER NOT NULL DEFAULT 0,
    voided_minor   INTEGER NOT NULL DEFAULT 0,
    refunded_minor INTEGER NOT NULL DEFAULT 0,
    updated_at     TIMESTAMP NOT NULL,

    CHECK (length(currency) = 3),
    CHECK (typeof(taken_minor) = 'integer' AND taken_minor >= 0),
    CHECK (typeof(voided_minor) = 'integer' AND voided_minor >= 0),
    CHECK (typeof(refunded_minor) = 'integer' AND refunded_minor >= 0),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- +goose Down
DROP TABLE shift_cash;

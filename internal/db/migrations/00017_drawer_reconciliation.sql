-- +goose Up
-- The blind count's reconciliation (phase 09, Task 2a, ADR-095).
--
-- A closing count is written once, with what was expected when it was
-- counted, the variance, and the variance threshold then in force: a
-- threshold the owner changes afterwards cannot make a closed day's
-- shortfall need, or stop needing, a reason. An opening count carries
-- zeros in all three.

ALTER TABLE drawer_counts ADD COLUMN expected INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(expected) = 'integer');

ALTER TABLE drawer_counts ADD COLUMN variance INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(variance) = 'integer'
           AND (kind = 'opening' OR variance = total - expected)
           AND (kind = 'closing' OR (expected = 0 AND variance = 0)));

ALTER TABLE drawer_counts ADD COLUMN threshold INTEGER NOT NULL DEFAULT 0
    CHECK (typeof(threshold) = 'integer' AND threshold >= 0
           AND (kind = 'closing' OR threshold = 0));

-- Why the drawer was over or short, given once after the count revealed
-- it. One per shift, never changed: the count is the evidence and this is
-- the explanation, both on the record as first given.
CREATE TABLE variance_reasons (
    shift_id     INTEGER PRIMARY KEY REFERENCES shifts (id) ON DELETE RESTRICT,
    reason       TEXT    NOT NULL,
    explained_by INTEGER NOT NULL,
    explained_at TIMESTAMP NOT NULL,

    CHECK (length(trim(reason)) > 0 AND length(reason) <= 500),
    CHECK (typeof(explained_by) = 'integer' AND explained_by >= 1),
    CHECK (explained_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- +goose StatementBegin
CREATE TRIGGER variance_reasons_no_update BEFORE UPDATE ON variance_reasons
BEGIN
    SELECT RAISE(ABORT, 'a variance reason is recorded once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER variance_reasons_no_delete BEFORE DELETE ON variance_reasons
BEGIN
    SELECT RAISE(ABORT, 'a variance reason is recorded once and never changed');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER variance_reasons_no_delete;
DROP TRIGGER variance_reasons_no_update;
DROP TABLE variance_reasons;
ALTER TABLE drawer_counts DROP COLUMN threshold;
ALTER TABLE drawer_counts DROP COLUMN variance;
ALTER TABLE drawer_counts DROP COLUMN expected;

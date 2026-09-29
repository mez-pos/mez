-- +goose Up
-- A closed shift's tip pool (phase 09, Task 4a, ADR-101): who was in it,
-- by what rule, and each share, recorded once and never changed.
--
-- body is the pool as JSON (internal/service/shift.TipPool): each member
-- with their minutes, their role and its weight, the tips they took and
-- their share, and the names they had. total is the pool, kept as a
-- column so a check can hold it to the shift.

CREATE TABLE tip_pools (
    shift_id  INTEGER PRIMARY KEY REFERENCES shifts (id) ON DELETE RESTRICT,
    rule      TEXT    NOT NULL,
    total     INTEGER NOT NULL,
    body      TEXT    NOT NULL,
    pooled_by INTEGER NOT NULL,
    pooled_at TIMESTAMP NOT NULL,

    CHECK (rule IN ('even', 'hours', 'role_weight')),
    CHECK (typeof(total) = 'integer' AND total >= 0),
    CHECK (json_valid(body) AND json_type(body) = 'object'),
    CHECK (typeof(pooled_by) = 'integer' AND pooled_by >= 1),
    CHECK (pooled_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
) WITHOUT ROWID;

-- +goose StatementBegin
CREATE TRIGGER tip_pools_no_update BEFORE UPDATE ON tip_pools
BEGIN
    SELECT RAISE(ABORT, 'a tip pool is recorded once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER tip_pools_no_delete BEFORE DELETE ON tip_pools
BEGIN
    SELECT RAISE(ABORT, 'a tip pool is recorded once and never changed');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER tip_pools_no_delete;
DROP TRIGGER tip_pools_no_update;
DROP TABLE tip_pools;

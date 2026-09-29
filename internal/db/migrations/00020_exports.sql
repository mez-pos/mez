-- +goose Up
-- A closed shift's export files (phase 09, Task 5b, ADR-104): the tax
-- register, the Tally vouchers and the day's sales, written by the
-- transaction that closes the shift and kept, so a bookkeeper can take
-- them again at any time. One of each kind per shift, never changed.

CREATE TABLE exports (
    id         INTEGER PRIMARY KEY,
    shift_id   INTEGER NOT NULL REFERENCES shifts (id) ON DELETE RESTRICT,
    kind       TEXT    NOT NULL,
    filename   TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    created_at TIMESTAMP NOT NULL,

    CHECK (kind IN ('tax_register', 'tally', 'sales')),
    CHECK (length(filename) > 0 AND filename GLOB '*.csv'),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX exports_shift_kind ON exports (shift_id, kind);

-- +goose StatementBegin
CREATE TRIGGER exports_no_update BEFORE UPDATE ON exports
BEGIN
    SELECT RAISE(ABORT, 'an export is written once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER exports_no_delete BEFORE DELETE ON exports
BEGIN
    SELECT RAISE(ABORT, 'an export is written once and never changed');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER exports_no_delete;
DROP TRIGGER exports_no_update;
DROP INDEX exports_shift_kind;
DROP TABLE exports;

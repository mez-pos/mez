-- +goose Up
-- Z-reports (phase 09, Task 3b, ADR-098): a shift's figures, frozen as a
-- numbered document when the shift closes.
--
-- body is the whole document as JSON (internal/service/shift.ZDocument):
-- the figures, the drawer, the names of the staff on it. It is never
-- worked out again from the orders once written: a report reads the same
-- in ten years whatever the menu, the staff list or this code do.
--
-- A correction is a new report, with the next number, naming the one it
-- corrects and why. The original stays.

CREATE TABLE z_reports (
    id           INTEGER PRIMARY KEY,
    venue_id     INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    number       INTEGER NOT NULL,
    shift_id     INTEGER NOT NULL REFERENCES shifts (id) ON DELETE RESTRICT,
    corrects_id  INTEGER REFERENCES z_reports (id) ON DELETE RESTRICT,
    reason       TEXT    NOT NULL DEFAULT '',
    body         TEXT    NOT NULL,
    generated_by INTEGER NOT NULL,
    generated_at TIMESTAMP NOT NULL,

    CHECK (typeof(number) = 'integer' AND number >= 1),
    CHECK (json_valid(body) AND json_type(body) = 'object'),
    CHECK (typeof(generated_by) = 'integer' AND generated_by >= 1),
    CHECK (generated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    -- A correction says why; an original has nothing to explain.
    CHECK ((corrects_id IS NULL) = (reason = '')),
    CHECK (length(reason) <= 500)
);

CREATE UNIQUE INDEX z_reports_venue_number ON z_reports (venue_id, number);
CREATE INDEX z_reports_shift ON z_reports (shift_id, number);
-- One original per shift; corrections after it.
CREATE UNIQUE INDEX z_reports_one_original ON z_reports (shift_id) WHERE corrects_id IS NULL;

-- The counter every Z number comes from, one row per venue, moved in the
-- transaction that writes the report. Never reset, never MAX(number) + 1.
CREATE TABLE z_report_series (
    venue_id INTEGER PRIMARY KEY REFERENCES venues (id) ON DELETE RESTRICT,
    last     INTEGER NOT NULL,
    CHECK (typeof(last) = 'integer' AND last >= 1)
);

-- +goose StatementBegin
CREATE TRIGGER z_reports_no_update BEFORE UPDATE ON z_reports
BEGIN
    SELECT RAISE(ABORT, 'a Z-report is immutable: issue a correction instead');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER z_reports_no_delete BEFORE DELETE ON z_reports
BEGIN
    SELECT RAISE(ABORT, 'a Z-report is immutable: issue a correction instead');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER z_reports_no_delete;
DROP TRIGGER z_reports_no_update;
DROP TABLE z_report_series;
DROP INDEX z_reports_one_original;
DROP INDEX z_reports_shift;
DROP INDEX z_reports_venue_number;
DROP TABLE z_reports;

-- +goose Up
-- The credit note register (phase 07, Task 6a, ADR-083): every credit note
-- issued, beside the invoice it corrects. Its numbers come from the same
-- counters as invoices, under the credit note series, so they are gapless
-- per venue, series and financial year in the same way. The event,
-- credit_note.issued, is the record; this is the index over it.
CREATE TABLE credit_notes (
    id                 INTEGER PRIMARY KEY,
    venue_id           INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    order_id           INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    series             TEXT    NOT NULL,
    fy_start           INTEGER NOT NULL,
    seq                INTEGER NOT NULL,
    credit_no          TEXT    NOT NULL,
    against_invoice_no TEXT    NOT NULL,
    total_minor        INTEGER NOT NULL,
    issued_at          TIMESTAMP NOT NULL,

    CHECK (length(series) > 0),
    CHECK (typeof(fy_start) = 'integer' AND fy_start > 0),
    CHECK (typeof(seq) = 'integer' AND seq >= 1),
    CHECK (length(credit_no) > 0),
    CHECK (length(against_invoice_no) > 0),
    CHECK (typeof(total_minor) = 'integer' AND total_minor > 0),
    CHECK (issued_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX credit_notes_series_seq ON credit_notes (venue_id, series, fy_start, seq);
CREATE UNIQUE INDEX credit_notes_number ON credit_notes (venue_id, credit_no);
CREATE INDEX credit_notes_order ON credit_notes (order_id);

-- +goose StatementBegin
CREATE TRIGGER credit_notes_from_the_counter BEFORE INSERT ON credit_notes
WHEN NEW.seq > coalesce((
    SELECT last_seq FROM invoice_series
    WHERE venue_id = NEW.venue_id AND series = NEW.series AND fy_start = NEW.fy_start
), 0)
BEGIN
    SELECT RAISE(ABORT, 'credit note number was not allocated from invoice_series');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER credit_notes_no_update BEFORE UPDATE ON credit_notes
BEGIN
    SELECT RAISE(ABORT, 'credit notes are never edited');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER credit_notes_no_delete BEFORE DELETE ON credit_notes
BEGIN
    SELECT RAISE(ABORT, 'credit notes are never deleted');
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE credit_notes;

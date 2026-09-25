-- +goose Up
-- Where jobs print (phase 07, Task 5b, ADR-081). The back office sets these
-- in Task 5c; the till reads them from 5b.

-- A station's tickets print on its printer. NULL: the station has none, and
-- its tickets are kept dead with that reason until one is set and retried.
ALTER TABLE stations ADD COLUMN printer_id INTEGER REFERENCES printers (id);

-- The receipt printer: at most one per venue.
ALTER TABLE printers ADD COLUMN is_receipt BOOLEAN NOT NULL DEFAULT 0 CHECK (is_receipt IN (0, 1));
CREATE UNIQUE INDEX printers_one_receipt ON printers (venue_id) WHERE is_receipt = 1;

-- The station a kitchen ticket was for, so a retry of a ticket queued with
-- no printer can find the one set up since. Empty for anything else.
ALTER TABLE print_jobs ADD COLUMN station TEXT NOT NULL DEFAULT '';

-- A counter per venue that moves on every change to its print jobs. The
-- stream compares it, as it does the menu's version, to tell tills when
-- to read the queue again.
CREATE TABLE print_versions (
    venue_id INTEGER PRIMARY KEY REFERENCES venues (id) ON DELETE CASCADE,
    version  INTEGER NOT NULL,
    CHECK (version > 0)
);

-- +goose Down
DROP TABLE print_versions;
ALTER TABLE print_jobs DROP COLUMN station;
DROP INDEX printers_one_receipt;
ALTER TABLE printers DROP COLUMN is_receipt;
ALTER TABLE stations DROP COLUMN printer_id;

-- +goose Up
-- The print queue (phase 07, Task 5, ADR-080): the printers a venue has and
-- the jobs waiting for them. A job holds the finished ESC/POS bytes, so a
-- retry prints exactly what the first try would have, whatever changed on
-- the menu or the venue since.

CREATE TABLE printers (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,

    -- tcp: address is "host" or "host:port", port 9100 by default.
    -- windows: address is the printer's name in Windows, sent RAW through
    -- the spooler, for a USB printer the venue computer drives itself.
    transport  TEXT    NOT NULL,
    address    TEXT    NOT NULL,
    paper_mm   INTEGER NOT NULL DEFAULT 80,
    active     BOOLEAN NOT NULL DEFAULT 1,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    UNIQUE (venue_id, name),
    CHECK (length(name) > 0),
    CHECK (transport IN ('tcp', 'windows')),
    CHECK (length(address) > 0),
    CHECK (paper_mm IN (80, 58)),
    CHECK (active IN (0, 1)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE TABLE print_jobs (
    id              INTEGER PRIMARY KEY,
    venue_id        INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,

    -- NULL when the venue has no printer for this job: the job is kept, dead,
    -- with that as its error, never dropped (Task 5b).
    printer_id      INTEGER REFERENCES printers (id),

    kind            TEXT    NOT NULL,
    label           TEXT    NOT NULL,   -- what staff read on the queue: "Receipt INV/26-27/000042"
    payload         BLOB    NOT NULL,   -- the ESC/POS bytes
    state           TEXT    NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP NOT NULL,
    last_error      TEXT    NOT NULL DEFAULT '',

    -- What caused it, when an order did: the order and the seq of its event.
    order_id        INTEGER REFERENCES orders (id),
    event_seq       INTEGER,

    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL,
    done_at         TIMESTAMP,

    CHECK (kind IN ('receipt', 'kitchen', 'z_report', 'test')),
    CHECK (state IN ('queued', 'printing', 'done', 'failed', 'dead')),
    CHECK (length(payload) > 0),
    CHECK (attempts >= 0),
    CHECK ((order_id IS NULL) = (event_seq IS NULL)),
    CHECK ((state = 'done') = (done_at IS NOT NULL)),
    CHECK (next_attempt_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (done_at IS NULL OR done_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- The worker's read: the oldest unfinished job per printer.
CREATE INDEX print_jobs_pending ON print_jobs (printer_id, state, id);
-- The screen's read: a venue's jobs, newest first.
CREATE INDEX print_jobs_venue ON print_jobs (venue_id, id);

-- +goose Down
DROP TABLE print_jobs;
DROP TABLE printers;

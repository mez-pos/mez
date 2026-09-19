-- Orders as event streams, and the invoice series they are numbered from.
--
-- docs/02-domain.md is the source for all of this. An order is its events;
-- everything else here is either an index over them (orders, invoices) or a
-- cache of their projection (order_snapshots), and the database is set up so
-- that the events cannot be edited even by a query that tries.
--
-- Nothing here cascades from a venue. An order is a financial record, and a
-- venue with orders cannot be deleted by accident: the foreign keys restrict.

-- +goose Up

-- One row per order: the columns a list of open tickets or a day's orders
-- filters on, kept in step with the events by the same transaction that
-- appends them. It is an index, not the order. Status, last_seq and closed_at
-- are the projection's, written after it runs, never decided here.
CREATE TABLE orders (
    id         INTEGER PRIMARY KEY,
    public_id  TEXT    NOT NULL,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    channel    TEXT    NOT NULL,

    -- 0 for no table. Tables arrive in phase 08; this becomes a foreign key
    -- then, with 0 mapped to NULL.
    table_id   INTEGER NOT NULL DEFAULT 0,

    status     TEXT    NOT NULL,
    last_seq   INTEGER NOT NULL,
    opened_at  TIMESTAMP NOT NULL,
    closed_at  TIMESTAMP,
    updated_at TIMESTAMP NOT NULL,

    CHECK (length(public_id) > 0),
    CHECK (channel IN ('counter', 'qr_table', 'web', 'kiosk', 'aggregator')),
    CHECK (typeof(table_id) = 'integer' AND table_id >= 0),
    CHECK (status IN ('open', 'held', 'settling', 'closed', 'reopened')),
    CHECK (typeof(last_seq) = 'integer' AND last_seq >= 1),
    CHECK (opened_at  GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (closed_at IS NULL OR closed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX orders_public_id ON orders (public_id);
CREATE INDEX orders_venue_status ON orders (venue_id, status);

-- The events. The envelope is in columns so it can be filtered and joined;
-- the type-specific body is JSON, decoded by order.DecodePayload.
--
-- id is global and only grows, which is what the WebSocket hub will replay
-- from: a device that last saw event 9000 asks for everything after it.
CREATE TABLE order_events (
    id        INTEGER PRIMARY KEY,
    order_id  INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,

    -- Dense from 1 within the order. The unique index below is what turns a
    -- second writer at the same seq into an error instead of a fork.
    seq       INTEGER NOT NULL,

    type      TEXT    NOT NULL,
    payload   TEXT    NOT NULL,

    -- 0 when nobody did it: a guest ordering from a table QR has no staff
    -- ID. Not foreign keys for that reason; the event is the record.
    actor_id  INTEGER NOT NULL DEFAULT 0,
    device_id INTEGER NOT NULL DEFAULT 0,

    at        TIMESTAMP NOT NULL,

    CHECK (typeof(seq) = 'integer' AND seq >= 1),
    CHECK (length(type) > 0),
    CHECK (json_valid(payload) AND json_type(payload) = 'object'),
    CHECK (typeof(actor_id) = 'integer' AND actor_id >= 0),
    CHECK (typeof(device_id) = 'integer' AND device_id >= 0),
    CHECK (at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX order_events_order_seq ON order_events (order_id, seq);

-- Append-only, enforced where no code path can go around it. A correction is
-- a later event; there is no statement that edits or removes one.
-- +goose StatementBegin
CREATE TRIGGER order_events_no_update BEFORE UPDATE ON order_events
BEGIN
    SELECT RAISE(ABORT, 'order_events is append-only: append a corrective event instead');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER order_events_no_delete BEFORE DELETE ON order_events
BEGIN
    SELECT RAISE(ABORT, 'order_events is append-only: append a corrective event instead');
END;
-- +goose StatementEnd

-- The latest projection of an order, so loading one with a long history
-- replays only the events since. Optimisation only: delete every row and
-- nothing is lost, it is rebuilt from events on the next load.
--
-- basis records what the state was projected under, the projection version
-- and the venue's tax regime. A snapshot whose basis is not the one being
-- loaded with is ignored, because replaying the events would give a
-- different answer and the events are the truth.
CREATE TABLE order_snapshots (
    order_id INTEGER PRIMARY KEY REFERENCES orders (id) ON DELETE CASCADE,
    seq      INTEGER NOT NULL,
    basis    TEXT    NOT NULL,
    state    TEXT    NOT NULL,

    CHECK (typeof(seq) = 'integer' AND seq >= 1),
    CHECK (length(basis) > 0),
    CHECK (json_valid(state))
) WITHOUT ROWID;

-- The counter every invoice number comes from: one row per venue, series
-- and financial year, holding the last number handed out. Allocation is one
-- upsert inside the issuing transaction, and SQLite's single writer is the
-- lock. Never MAX(n)+1 over invoices.
CREATE TABLE invoice_series (
    venue_id INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    series   TEXT    NOT NULL,
    fy_start INTEGER NOT NULL,
    last_seq INTEGER NOT NULL,

    PRIMARY KEY (venue_id, series, fy_start),

    CHECK (length(series) > 0),
    CHECK (typeof(fy_start) = 'integer' AND fy_start > 0),
    CHECK (typeof(last_seq) = 'integer' AND last_seq >= 1)
) WITHOUT ROWID;

-- A counter moves forward by exactly one and is never removed. Any other
-- change is a skipped or reused number, which is the incident the whole
-- table exists to prevent.
-- +goose StatementBegin
CREATE TRIGGER invoice_series_step_by_one BEFORE UPDATE ON invoice_series
WHEN NEW.last_seq != OLD.last_seq + 1
  OR NEW.venue_id != OLD.venue_id
  OR NEW.series   != OLD.series
  OR NEW.fy_start != OLD.fy_start
BEGIN
    SELECT RAISE(ABORT, 'invoice_series only ever moves forward by one');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER invoice_series_no_delete BEFORE DELETE ON invoice_series
BEGIN
    SELECT RAISE(ABORT, 'invoice_series rows are never deleted');
END;
-- +goose StatementEnd

-- Every invoice number issued, and the order it was issued for. The
-- invoice.issued event is the record, with its tax snapshot; this is the
-- register a tax export and an audit read, and a second guard on the series:
-- the unique index refuses a number twice even if a counter were wrong.
CREATE TABLE invoices (
    id         INTEGER PRIMARY KEY,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    series     TEXT    NOT NULL,
    fy_start   INTEGER NOT NULL,
    seq        INTEGER NOT NULL,
    invoice_no TEXT    NOT NULL,
    issued_at  TIMESTAMP NOT NULL,

    CHECK (length(series) > 0),
    CHECK (typeof(fy_start) = 'integer' AND fy_start > 0),
    CHECK (typeof(seq) = 'integer' AND seq >= 1),
    CHECK (length(invoice_no) > 0),
    CHECK (issued_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX invoices_series_seq ON invoices (venue_id, series, fy_start, seq);
CREATE UNIQUE INDEX invoices_number ON invoices (venue_id, invoice_no);
CREATE INDEX invoices_order ON invoices (order_id);

-- An invoice can only carry a number its counter has handed out. A row
-- written with a number made up in Go, or computed from the table, fails.
-- +goose StatementBegin
CREATE TRIGGER invoices_from_the_counter BEFORE INSERT ON invoices
WHEN NEW.seq > coalesce((
    SELECT last_seq FROM invoice_series
    WHERE venue_id = NEW.venue_id AND series = NEW.series AND fy_start = NEW.fy_start
), 0)
BEGIN
    SELECT RAISE(ABORT, 'invoice number was not allocated from invoice_series');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER invoices_no_update BEFORE UPDATE ON invoices
BEGIN
    SELECT RAISE(ABORT, 'invoices are never edited');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER invoices_no_delete BEFORE DELETE ON invoices
BEGIN
    SELECT RAISE(ABORT, 'invoices are never deleted');
END;
-- +goose StatementEnd

-- +goose Down

DROP TABLE invoices;
DROP TABLE invoice_series;
DROP TABLE order_snapshots;
DROP TABLE order_events;
DROP TABLE orders;

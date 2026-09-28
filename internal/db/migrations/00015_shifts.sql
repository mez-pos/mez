-- +goose Up
-- Shifts (phase 09, Task 1a): the reporting unit of a trading day, from
-- docs/02-domain.md. One venue, one drawer, one open shift at a time.
--
-- A shift row is written twice: once when it opens, once when it closes.
-- Everything that happens in between is a shift_event, append-only as the
-- order events are, and every count of the drawer is a drawer_count that
-- can never be changed.

CREATE TABLE shifts (
    id          INTEGER PRIMARY KEY,
    public_id   TEXT    NOT NULL,
    venue_id    INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    status      TEXT    NOT NULL,

    -- The trading day, a civil date in the venue's zone, decided once when
    -- the shift opens (shift.TradingDay). A bar that closes at 2am reports
    -- the night under the day it opened.
    trading_day TEXT    NOT NULL,

    -- Who opened and closed it. Not foreign keys, as order events' actors
    -- are not: the shift is the record. closed_by is 0 while open.
    opened_by   INTEGER NOT NULL,
    opened_at   TIMESTAMP NOT NULL,
    closed_by   INTEGER NOT NULL DEFAULT 0,
    closed_at   TIMESTAMP,

    CHECK (length(public_id) > 0),
    CHECK (status IN ('open', 'closed')),
    CHECK (trading_day GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    CHECK (typeof(opened_by) = 'integer' AND opened_by >= 1),
    CHECK (typeof(closed_by) = 'integer' AND closed_by >= 0),
    CHECK (opened_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (closed_at IS NULL OR closed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    -- Closed exactly when it has a closer and a closing time.
    CHECK ((status = 'open') = (closed_at IS NULL AND closed_by = 0))
);

CREATE UNIQUE INDEX shifts_public_id ON shifts (public_id);
CREATE INDEX shifts_venue_day ON shifts (venue_id, trading_day);

-- The one open shift per venue. Two tills pressing Open at once get a
-- constraint error for the second, not a second drawer.
CREATE UNIQUE INDEX shifts_one_open ON shifts (venue_id) WHERE status = 'open';

-- A shift's only change is open to closed, once. A closed shift is what a
-- Z-report is drawn from, and no statement can move it after that.
-- +goose StatementBegin
CREATE TRIGGER shifts_closed_is_final BEFORE UPDATE ON shifts
WHEN OLD.status = 'closed'
BEGIN
    SELECT RAISE(ABORT, 'a closed shift cannot change');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER shifts_open_changes_only_to_close BEFORE UPDATE ON shifts
WHEN NEW.id != OLD.id OR NEW.public_id != OLD.public_id OR NEW.venue_id != OLD.venue_id
  OR NEW.trading_day != OLD.trading_day OR NEW.opened_by != OLD.opened_by
  OR NEW.opened_at != OLD.opened_at OR NEW.status != 'closed'
BEGIN
    SELECT RAISE(ABORT, 'an open shift changes only by closing');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER shifts_no_delete BEFORE DELETE ON shifts
BEGIN
    SELECT RAISE(ABORT, 'shifts are never deleted');
END;
-- +goose StatementEnd

-- Cash counted in the drawer: the opening float, and (Task 2) the blind
-- count at close. One of each per shift. denominations is a JSON array of
-- {"face": minor units, "count": pieces}, empty for a sum typed in whole;
-- total is what they add up to, checked in Go (shift.Count.Validate).
CREATE TABLE drawer_counts (
    id            INTEGER PRIMARY KEY,
    shift_id      INTEGER NOT NULL REFERENCES shifts (id) ON DELETE RESTRICT,
    kind          TEXT    NOT NULL,
    denominations TEXT    NOT NULL,
    total         INTEGER NOT NULL,
    counted_by    INTEGER NOT NULL,
    counted_at    TIMESTAMP NOT NULL,

    CHECK (kind IN ('opening', 'closing')),
    CHECK (json_valid(denominations) AND json_type(denominations) = 'array'),
    CHECK (typeof(total) = 'integer' AND total >= 0),
    CHECK (typeof(counted_by) = 'integer' AND counted_by >= 1),
    CHECK (counted_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

CREATE UNIQUE INDEX drawer_counts_shift_kind ON drawer_counts (shift_id, kind);

-- +goose StatementBegin
CREATE TRIGGER drawer_counts_no_update BEFORE UPDATE ON drawer_counts
BEGIN
    SELECT RAISE(ABORT, 'a drawer count is recorded once and never changed');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER drawer_counts_no_delete BEFORE DELETE ON drawer_counts
BEGIN
    SELECT RAISE(ABORT, 'a drawer count is recorded once and never changed');
END;
-- +goose StatementEnd

-- What happened in a shift, in order: opened, cash in, cash out, closed.
-- A cash movement carries its amount (always positive; the type says which
-- way), its reason, and the manager who authorised it.
--
-- idempotency_key is the client's key for a cash movement, so a till on
-- flaky wifi that retries a payout records it once. '' for none.
CREATE TABLE shift_events (
    id              INTEGER PRIMARY KEY,
    shift_id        INTEGER NOT NULL REFERENCES shifts (id) ON DELETE RESTRICT,
    type            TEXT    NOT NULL,
    amount          INTEGER NOT NULL DEFAULT 0,
    reason          TEXT    NOT NULL DEFAULT '',
    actor_id        INTEGER NOT NULL,
    authorised_by   INTEGER NOT NULL DEFAULT 0,
    device_id       INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT    NOT NULL DEFAULT '',
    at              TIMESTAMP NOT NULL,

    CHECK (type IN ('opened', 'cash_in', 'cash_out', 'closed')),
    CHECK (typeof(amount) = 'integer' AND amount >= 0),
    CHECK (typeof(actor_id) = 'integer' AND actor_id >= 1),
    CHECK (typeof(authorised_by) = 'integer' AND authorised_by >= 0),
    CHECK (typeof(device_id) = 'integer' AND device_id >= 0),
    CHECK (length(idempotency_key) <= 255),
    CHECK (at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    -- A cash movement is accountable: something moved, why, and who said so.
    CHECK (type NOT IN ('cash_in', 'cash_out')
           OR (amount > 0 AND length(trim(reason)) > 0 AND authorised_by >= 1))
);

CREATE INDEX shift_events_shift ON shift_events (shift_id, id);
CREATE UNIQUE INDEX shift_events_key ON shift_events (shift_id, idempotency_key)
    WHERE idempotency_key != '';

-- +goose StatementBegin
CREATE TRIGGER shift_events_no_update BEFORE UPDATE ON shift_events
BEGIN
    SELECT RAISE(ABORT, 'shift_events is append-only: record a corrective movement instead');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER shift_events_no_delete BEFORE DELETE ON shift_events
BEGIN
    SELECT RAISE(ABORT, 'shift_events is append-only: record a corrective movement instead');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER shift_events_no_delete;
DROP TRIGGER shift_events_no_update;
DROP INDEX shift_events_key;
DROP INDEX shift_events_shift;
DROP TABLE shift_events;
DROP TRIGGER drawer_counts_no_delete;
DROP TRIGGER drawer_counts_no_update;
DROP INDEX drawer_counts_shift_kind;
DROP TABLE drawer_counts;
DROP TRIGGER shifts_no_delete;
DROP TRIGGER shifts_open_changes_only_to_close;
DROP TRIGGER shifts_closed_is_final;
DROP INDEX shifts_one_open;
DROP INDEX shifts_venue_day;
DROP INDEX shifts_public_id;
DROP TABLE shifts;

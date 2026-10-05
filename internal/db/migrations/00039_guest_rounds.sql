-- +goose Up
-- A round is one time a guest pressed Place Order (phase 11, Task 4a,
-- ADR-210): the dishes they asked for at a table, priced by the server when
-- they asked.
--
-- A round at a table with a ticket open is written here in the transaction
-- that appends its lines to that ticket, and is 'sent' from the start. A
-- round at a table with none is a request (ADR-209): it waits here, and is
-- no order and no event until staff accept it. So a request nobody accepts,
-- or one made from a photograph of the sticker, never reaches the order
-- stream, the kitchen or the Z-report.
CREATE TABLE guest_rounds (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,

    status    TEXT    NOT NULL,
    -- The ticket the lines are on, once they are on one.
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,

    -- The lines as priced when the guest asked, a JSON list of line.added
    -- payloads. What an accepted request appends is this, so the price the
    -- guest was shown is the price on the bill.
    lines          TEXT    NOT NULL,
    subtotal_minor INTEGER NOT NULL,
    currency       TEXT    NOT NULL,

    -- A phone on a poor connection sends a round again: the key is how the
    -- second one is known to be the first.
    idempotency_key TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL,
    -- When a request was accepted or declined, and by which staff member.
    decided_at TIMESTAMP,
    decided_by INTEGER,

    UNIQUE (public_id),
    UNIQUE (venue_id, idempotency_key),
    CHECK (status IN ('waiting', 'sent', 'declined')),
    CHECK ((status = 'sent') = (order_id IS NOT NULL)),
    CHECK (typeof(subtotal_minor) = 'integer'),
    CHECK (length(idempotency_key) BETWEEN 1 AND 255),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (decided_at IS NULL OR decided_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- "What is waiting at this table", and "what is waiting at this venue".
CREATE INDEX guest_rounds_table ON guest_rounds (table_id, status);
CREATE INDEX guest_rounds_venue ON guest_rounds (venue_id, status, created_at);

-- +goose Down
DROP INDEX guest_rounds_venue;
DROP INDEX guest_rounds_table;
DROP TABLE guest_rounds;

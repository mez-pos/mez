-- +goose Up
-- A charge can be for a round held until it is paid (phase 11, Task 4e5b,
-- ADR-226), and such a round is on no ticket when the charge is asked: the
-- table may have none. So a charge names its round, and its ticket only
-- once the paid round is on one.
--
-- SQLite cannot take NOT NULL off a column, so the table is built again.
CREATE TABLE gateway_charges_new (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    -- The ticket the payment goes on. Null only for a round's charge, until
    -- the round is on a ticket.
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,

    gateway      TEXT NOT NULL,
    gateway_id   TEXT NOT NULL,
    checkout_url TEXT NOT NULL,

    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,

    status TEXT NOT NULL,

    method    TEXT,
    reference TEXT,
    problem TEXT,

    created_at TIMESTAMP NOT NULL,
    settled_at TIMESTAMP,

    resolution TEXT
        CHECK (resolution IS NULL OR resolution IN ('refunded', 'recorded')),
    resolved_at TIMESTAMP
        CHECK (resolved_at IS NULL OR resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    resolved_by INTEGER,

    -- The round this pays for, when it is a round's charge and not a bill's.
    round_public_id TEXT REFERENCES guest_rounds (public_id) ON DELETE CASCADE,

    UNIQUE (public_id),
    UNIQUE (gateway, gateway_id),
    CHECK (order_id IS NOT NULL OR round_public_id IS NOT NULL),
    CHECK (status IN ('open', 'paid', 'unapplied', 'failed')),
    CHECK ((status = 'open') = (settled_at IS NULL)),
    CHECK (status NOT IN ('paid', 'unapplied') OR (method IS NOT NULL AND reference IS NOT NULL)),
    CHECK ((status = 'unapplied') = (problem IS NOT NULL)),
    CHECK (typeof(amount_minor) = 'integer' AND amount_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (settled_at IS NULL OR settled_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO gateway_charges_new (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
                                 amount_minor, currency, status, method, reference, problem, created_at, settled_at,
                                 resolution, resolved_at, resolved_by)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
       amount_minor, currency, status, method, reference, problem, created_at, settled_at,
       resolution, resolved_at, resolved_by
FROM gateway_charges;

DROP INDEX gateway_charges_order;
DROP INDEX gateway_charges_venue;
DROP TABLE gateway_charges;
ALTER TABLE gateway_charges_new RENAME TO gateway_charges;

CREATE INDEX gateway_charges_venue ON gateway_charges (venue_id, status, created_at);
CREATE INDEX gateway_charges_order ON gateway_charges (order_id, status);
-- A round is asked for once at a time: two phones starting at once get the
-- one charge.
CREATE UNIQUE INDEX gateway_charges_round_open ON gateway_charges (round_public_id)
    WHERE round_public_id IS NOT NULL AND status = 'open';

-- +goose Down
-- A round's charge that never reached a ticket has no place in the table as
-- it was, and is left behind.
CREATE TABLE gateway_charges_old (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    order_id  INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    gateway      TEXT NOT NULL,
    gateway_id   TEXT NOT NULL,
    checkout_url TEXT NOT NULL,
    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,
    status TEXT NOT NULL,
    method    TEXT,
    reference TEXT,
    problem TEXT,
    created_at TIMESTAMP NOT NULL,
    settled_at TIMESTAMP,
    resolution TEXT
        CHECK (resolution IS NULL OR resolution IN ('refunded', 'recorded')),
    resolved_at TIMESTAMP
        CHECK (resolved_at IS NULL OR resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    resolved_by INTEGER,
    UNIQUE (public_id),
    UNIQUE (gateway, gateway_id),
    CHECK (status IN ('open', 'paid', 'unapplied', 'failed')),
    CHECK ((status = 'open') = (settled_at IS NULL)),
    CHECK (status NOT IN ('paid', 'unapplied') OR (method IS NOT NULL AND reference IS NOT NULL)),
    CHECK ((status = 'unapplied') = (problem IS NOT NULL)),
    CHECK (typeof(amount_minor) = 'integer' AND amount_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (settled_at IS NULL OR settled_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO gateway_charges_old (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
                                 amount_minor, currency, status, method, reference, problem, created_at, settled_at,
                                 resolution, resolved_at, resolved_by)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url,
       amount_minor, currency, status, method, reference, problem, created_at, settled_at,
       resolution, resolved_at, resolved_by
FROM gateway_charges
WHERE order_id IS NOT NULL;

DROP INDEX gateway_charges_round_open;
DROP INDEX gateway_charges_order;
DROP INDEX gateway_charges_venue;
DROP TABLE gateway_charges;
ALTER TABLE gateway_charges_old RENAME TO gateway_charges;

CREATE INDEX gateway_charges_venue ON gateway_charges (venue_id, status, created_at);
CREATE INDEX gateway_charges_order ON gateway_charges (order_id, status);

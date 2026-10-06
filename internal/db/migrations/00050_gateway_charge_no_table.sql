-- +goose Up
-- A charge may have no table (phase 11, Task 5f1b, ADR-249): the charge for
-- an order to collect, which has none. A bill's charge still has one: only a
-- round's may go without.
--
-- SQLite cannot take NOT NULL off a column, so the table is built again, as
-- migration 00044 built it. Nothing refers to this table, so building it
-- again deletes nothing elsewhere.
CREATE TABLE gateway_charges_new (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    -- The table it was started from. Null for an order to collect.
    table_id  INTEGER REFERENCES tables (id) ON DELETE CASCADE,
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,

    gateway      TEXT NOT NULL,
    gateway_id   TEXT NOT NULL,
    checkout_url TEXT NOT NULL,

    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,

    status TEXT NOT NULL,

    method    TEXT,
    reference TEXT,
    problem   TEXT,

    created_at TIMESTAMP NOT NULL,
    settled_at TIMESTAMP,

    resolution TEXT
        CHECK (resolution IS NULL OR resolution IN ('refunded', 'recorded')),
    resolved_at TIMESTAMP
        CHECK (resolved_at IS NULL OR resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    resolved_by INTEGER,

    round_public_id TEXT REFERENCES guest_rounds (public_id) ON DELETE CASCADE,

    payer TEXT
        CHECK (payer IS NULL OR (round_public_id IS NULL AND length(payer) BETWEEN 1 AND 64)),
    let_go_at TIMESTAMP
        CHECK (let_go_at IS NULL OR (round_public_id IS NULL
            AND let_go_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')),

    UNIQUE (public_id),
    UNIQUE (gateway, gateway_id),
    CHECK (order_id IS NOT NULL OR round_public_id IS NOT NULL),
    -- Only a round's charge may have no table: a bill is a table's.
    CHECK (table_id IS NOT NULL OR round_public_id IS NOT NULL),
    CHECK (status IN ('open', 'paid', 'unapplied', 'failed')),
    CHECK ((status = 'open') = (settled_at IS NULL)),
    CHECK (status NOT IN ('paid', 'unapplied') OR (method IS NOT NULL AND reference IS NOT NULL)),
    CHECK ((status = 'unapplied') = (problem IS NOT NULL)),
    CHECK (typeof(amount_minor) = 'integer' AND amount_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (settled_at IS NULL OR settled_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO gateway_charges_new (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
                                 status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
                                 round_public_id, payer, let_go_at)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges;

DROP INDEX gateway_charges_share_open;
DROP INDEX gateway_charges_round_open;
DROP INDEX gateway_charges_order;
DROP INDEX gateway_charges_venue;
DROP TABLE gateway_charges;
ALTER TABLE gateway_charges_new RENAME TO gateway_charges;

CREATE INDEX gateway_charges_venue ON gateway_charges (venue_id, status, created_at);
CREATE INDEX gateway_charges_order ON gateway_charges (order_id, status);
CREATE UNIQUE INDEX gateway_charges_round_open ON gateway_charges (round_public_id)
    WHERE round_public_id IS NOT NULL AND status = 'open';
CREATE UNIQUE INDEX gateway_charges_share_open ON gateway_charges (order_id, payer, amount_minor)
    WHERE payer IS NOT NULL AND status = 'open' AND let_go_at IS NULL;

-- A pickup code is unique among the orders still out, not for all time:
-- three characters are a few thousand codes, and a code is free again once
-- its order is closed. That is a fact about the order and not the round, so
-- it is held where the code is given (internal/service/order), in the one
-- write transaction, and not by an index here.
DROP INDEX guest_rounds_pickup_code;
CREATE INDEX guest_rounds_pickup_code ON guest_rounds (venue_id, pickup_code) WHERE pickup_code IS NOT NULL;

-- +goose Down
-- A charge with no table has no place in the table as it was, and it is
-- money: going back is refused while one is there.
CREATE TABLE gateway_charges_guard (no_table INTEGER CHECK (no_table = 0));
INSERT INTO gateway_charges_guard SELECT count(*) FROM gateway_charges WHERE table_id IS NULL;
DROP TABLE gateway_charges_guard;

DROP INDEX guest_rounds_pickup_code;
CREATE UNIQUE INDEX guest_rounds_pickup_code ON guest_rounds (venue_id, pickup_code) WHERE pickup_code IS NOT NULL;

CREATE TABLE gateway_charges_old (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,
    gateway      TEXT NOT NULL,
    gateway_id   TEXT NOT NULL,
    checkout_url TEXT NOT NULL,
    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,
    status TEXT NOT NULL,
    method    TEXT,
    reference TEXT,
    problem   TEXT,
    created_at TIMESTAMP NOT NULL,
    settled_at TIMESTAMP,
    resolution TEXT
        CHECK (resolution IS NULL OR resolution IN ('refunded', 'recorded')),
    resolved_at TIMESTAMP
        CHECK (resolved_at IS NULL OR resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    resolved_by INTEGER,
    round_public_id TEXT REFERENCES guest_rounds (public_id) ON DELETE CASCADE,
    payer TEXT
        CHECK (payer IS NULL OR (round_public_id IS NULL AND length(payer) BETWEEN 1 AND 64)),
    let_go_at TIMESTAMP
        CHECK (let_go_at IS NULL OR (round_public_id IS NULL
            AND let_go_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')),
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

INSERT INTO gateway_charges_old (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
                                 status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
                                 round_public_id, payer, let_go_at)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges;

DROP INDEX gateway_charges_share_open;
DROP INDEX gateway_charges_round_open;
DROP INDEX gateway_charges_order;
DROP INDEX gateway_charges_venue;
DROP TABLE gateway_charges;
ALTER TABLE gateway_charges_old RENAME TO gateway_charges;

CREATE INDEX gateway_charges_venue ON gateway_charges (venue_id, status, created_at);
CREATE INDEX gateway_charges_order ON gateway_charges (order_id, status);
CREATE UNIQUE INDEX gateway_charges_round_open ON gateway_charges (round_public_id)
    WHERE round_public_id IS NOT NULL AND status = 'open';
CREATE UNIQUE INDEX gateway_charges_share_open ON gateway_charges (order_id, payer, amount_minor)
    WHERE payer IS NOT NULL AND status = 'open' AND let_go_at IS NULL;

-- +goose Up
-- A guest's round may have no table (phase 11, Task 5f1a, ADR-237, ADR-248):
-- an order to collect, placed by the venue's pickup code. It is a round like
-- any other, held until it is paid and sent once it is, so it is a row here
-- and not a table of its own: a second kind of round would be a second copy
-- of everything a round already does about money.
--
-- SQLite cannot take NOT NULL off a column, so the table is built again.
-- gateway_charges refers to a round and is deleted with it, and dropping
-- this table deletes every row of it: the charges that name a round are
-- kept aside first and put back after. They are money, and none is lost.
--
-- pickup_code is what is called out when a pickup order is ready
-- (docs/02-domain.md): three or four characters, given when the paid round
-- is sent, and never to a round at a table.
CREATE TABLE guest_rounds_new (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    -- The table it was ordered from. Null for an order to collect.
    table_id  INTEGER REFERENCES tables (id) ON DELETE CASCADE,

    status    TEXT    NOT NULL,
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,

    lines          TEXT    NOT NULL,
    subtotal_minor INTEGER NOT NULL,
    currency       TEXT    NOT NULL,

    idempotency_key TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,

    created_at TIMESTAMP NOT NULL,
    decided_at TIMESTAMP,
    decided_by INTEGER,

    prepay    INTEGER NOT NULL DEFAULT 0,
    due_minor INTEGER,
    first_seq INTEGER,

    pickup_code TEXT,

    UNIQUE (public_id),
    UNIQUE (venue_id, idempotency_key),
    CHECK (status IN ('waiting', 'sent', 'declined')),
    CHECK ((status = 'sent') = (order_id IS NOT NULL)),
    CHECK (typeof(subtotal_minor) = 'integer'),
    CHECK (length(idempotency_key) BETWEEN 1 AND 255),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (decided_at IS NULL OR decided_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (prepay IN (0, 1)),
    CHECK (due_minor IS NULL OR (typeof(due_minor) = 'integer' AND due_minor > 0)),
    CHECK (first_seq IS NULL OR (typeof(first_seq) = 'integer' AND first_seq >= 2 AND status = 'sent')),
    -- An order to collect is always paid first: nobody at a table is there
    -- for staff to see, so the payment is what ties it to a guest (ADR-237).
    CHECK (table_id IS NOT NULL OR prepay = 1),
    CHECK (pickup_code IS NULL OR (table_id IS NULL AND status = 'sent' AND length(pickup_code) BETWEEN 3 AND 4))
);

INSERT INTO guest_rounds_new (id, public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
                              idempotency_key, fingerprint, created_at, decided_at, decided_by, prepay, due_minor, first_seq)
SELECT id, public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
       idempotency_key, fingerprint, created_at, decided_at, decided_by, prepay, due_minor, first_seq
FROM guest_rounds;

CREATE TABLE gateway_charges_kept AS
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges
WHERE round_public_id IS NOT NULL;

DROP INDEX guest_rounds_venue;
DROP INDEX guest_rounds_table;
DROP TABLE guest_rounds;
ALTER TABLE guest_rounds_new RENAME TO guest_rounds;

INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
                             status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
                             round_public_id, payer, let_go_at)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges_kept
WHERE id NOT IN (SELECT id FROM gateway_charges);
DROP TABLE gateway_charges_kept;

CREATE INDEX guest_rounds_table ON guest_rounds (table_id, status);
CREATE INDEX guest_rounds_venue ON guest_rounds (venue_id, status, created_at);
-- A pickup code is called out across a counter: no two orders still out at
-- one venue share one.
CREATE UNIQUE INDEX guest_rounds_pickup_code ON guest_rounds (venue_id, pickup_code) WHERE pickup_code IS NOT NULL;

-- +goose Down
-- A round with no table has no place in the table as it was. Going back
-- with one in it would mean deleting an order a guest paid for, so this
-- refuses to: the guard below fails while any is there.
CREATE TABLE guest_rounds_guard (pickup_rounds INTEGER CHECK (pickup_rounds = 0));
INSERT INTO guest_rounds_guard SELECT count(*) FROM guest_rounds WHERE table_id IS NULL;
DROP TABLE guest_rounds_guard;

CREATE TABLE guest_rounds_old (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    status    TEXT    NOT NULL,
    order_id  INTEGER REFERENCES orders (id) ON DELETE RESTRICT,
    lines          TEXT    NOT NULL,
    subtotal_minor INTEGER NOT NULL,
    currency       TEXT    NOT NULL,
    idempotency_key TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    decided_at TIMESTAMP,
    decided_by INTEGER,
    prepay    INTEGER NOT NULL DEFAULT 0 CHECK (prepay IN (0, 1)),
    due_minor INTEGER CHECK (due_minor IS NULL OR (typeof(due_minor) = 'integer' AND due_minor > 0)),
    first_seq INTEGER CHECK (first_seq IS NULL OR (typeof(first_seq) = 'integer' AND first_seq >= 2 AND status = 'sent')),
    UNIQUE (public_id),
    UNIQUE (venue_id, idempotency_key),
    CHECK (status IN ('waiting', 'sent', 'declined')),
    CHECK ((status = 'sent') = (order_id IS NOT NULL)),
    CHECK (typeof(subtotal_minor) = 'integer'),
    CHECK (length(idempotency_key) BETWEEN 1 AND 255),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (decided_at IS NULL OR decided_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

INSERT INTO guest_rounds_old (id, public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
                              idempotency_key, fingerprint, created_at, decided_at, decided_by, prepay, due_minor, first_seq)
SELECT id, public_id, venue_id, table_id, status, order_id, lines, subtotal_minor, currency,
       idempotency_key, fingerprint, created_at, decided_at, decided_by, prepay, due_minor, first_seq
FROM guest_rounds;

CREATE TABLE gateway_charges_kept AS
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges
WHERE round_public_id IS NOT NULL;

DROP INDEX guest_rounds_pickup_code;
DROP INDEX guest_rounds_venue;
DROP INDEX guest_rounds_table;
DROP TABLE guest_rounds;
ALTER TABLE guest_rounds_old RENAME TO guest_rounds;

INSERT INTO gateway_charges (id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
                             status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
                             round_public_id, payer, let_go_at)
SELECT id, public_id, venue_id, table_id, order_id, gateway, gateway_id, checkout_url, amount_minor, currency,
       status, method, reference, problem, created_at, settled_at, resolution, resolved_at, resolved_by,
       round_public_id, payer, let_go_at
FROM gateway_charges_kept
WHERE id NOT IN (SELECT id FROM gateway_charges);
DROP TABLE gateway_charges_kept;

CREATE INDEX guest_rounds_table ON guest_rounds (table_id, status);
CREATE INDEX guest_rounds_venue ON guest_rounds (venue_id, status, created_at);

-- +goose Up
-- A charge is money asked of a guest through a payment gateway (phase 11,
-- Task 4f, ADR-220): what was asked, of which ticket, at which provider,
-- and what became of it.
--
-- It is not a payment. The payment is the payment.tendered event the charge
-- puts on the ticket once the provider says it has the money. This row is
-- how that happens exactly once, and where a charge the provider took but
-- the bill could not take is kept until a person deals with it.
CREATE TABLE gateway_charges (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    order_id  INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,

    -- The provider, and its own id for the charge.
    gateway      TEXT NOT NULL,
    gateway_id   TEXT NOT NULL,
    checkout_url TEXT NOT NULL,

    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,

    -- open       asked, and the provider has not said paid or failed
    -- paid       the provider has the money and it is on the ticket
    -- unapplied  the provider has the money and the ticket could not take
    --            it: someone must refund it or record it by hand
    -- failed     the provider says it will not be paid
    status TEXT NOT NULL,

    -- How it was paid and the provider's id for the payment, written when
    -- the provider first says paid and never changed after: the tender is
    -- made from these, so asking twice tenders the same thing.
    method    TEXT,
    reference TEXT,
    -- Why the ticket could not take it, when unapplied.
    problem TEXT,

    created_at TIMESTAMP NOT NULL,
    settled_at TIMESTAMP,

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

CREATE INDEX gateway_charges_venue ON gateway_charges (venue_id, status, created_at);
CREATE INDEX gateway_charges_order ON gateway_charges (order_id, status);

-- +goose Down
DROP INDEX gateway_charges_order;
DROP INDEX gateway_charges_venue;
DROP TABLE gateway_charges;

-- +goose Up
-- A paid notice is a guest saying "I have paid" on their phone (phase 11,
-- Task 4e4, ADR-218). It is a message to staff and nothing else: it is not
-- a payment, it settles nothing, and no report counts it. A member of staff
-- looks for the money on the venue's own phone and records the payment on
-- the terminal, which is the only thing that is a payment.
--
-- It is kept against the ticket it was said of, with the balance as it
-- stood, so staff know what to look for.
CREATE TABLE paid_notices (
    id        INTEGER PRIMARY KEY,
    public_id TEXT    NOT NULL,
    venue_id  INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id  INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    order_id  INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,

    -- What was left to pay when the guest said so.
    amount_minor INTEGER NOT NULL,
    currency     TEXT    NOT NULL,

    -- 'open' until a member of staff has dealt with it, then 'seen'.
    status     TEXT      NOT NULL,
    created_at TIMESTAMP NOT NULL,
    seen_at    TIMESTAMP,
    seen_by    INTEGER,

    UNIQUE (public_id),
    CHECK (status IN ('open', 'seen')),
    CHECK ((status = 'seen') = (seen_at IS NOT NULL)),
    CHECK (typeof(amount_minor) = 'integer' AND amount_minor > 0),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (seen_at IS NULL OR seen_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- One open notice a ticket: a guest who taps forty times has said it once.
CREATE UNIQUE INDEX paid_notices_open ON paid_notices (order_id) WHERE status = 'open';
CREATE INDEX paid_notices_venue ON paid_notices (venue_id, status, created_at);

-- +goose Down
DROP INDEX paid_notices_venue;
DROP INDEX paid_notices_open;
DROP TABLE paid_notices;

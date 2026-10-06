-- +goose Up
-- A guest's phone holds a session for the sitting (ADR-262). The table's
-- code says which table; it never expires and anyone who once held it holds
-- it still, so it no longer opens the bill or the kitchen alone. A session
-- does: drawn by the server, bound to the ticket the phone's party is on,
-- and over when that ticket closes or in twelve hours, whichever is first.
--
-- order_id is null for a session handed out while the table had no ticket,
-- before its party's first order was accepted. Such a session is good only
-- while the table still has none: the ticket its round opens or joins is
-- what binds it.
--
-- The token is kept as its SHA-256, as a device's is. handoff is the one
-- exception: a session made when staff accept an order from a phone that
-- had none is kept in clear until that phone first presents it, since the
-- phone is told it by tracking the order it placed (round_id), and is then
-- forgotten.
CREATE TABLE guest_sessions (
    id         INTEGER PRIMARY KEY,
    public_id  TEXT    NOT NULL,
    venue_id   INTEGER NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    table_id   INTEGER NOT NULL REFERENCES tables (id) ON DELETE CASCADE,
    order_id   INTEGER REFERENCES orders (id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL,

    round_id   INTEGER REFERENCES guest_rounds (id) ON DELETE SET NULL,
    handoff    TEXT,

    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    ended_at   TIMESTAMP,

    UNIQUE (public_id),
    UNIQUE (token_hash),
    CHECK (length(token_hash) = 64),
    CHECK (handoff IS NULL OR (round_id IS NOT NULL AND order_id IS NOT NULL)),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (expires_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'),
    CHECK (ended_at IS NULL OR ended_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
);

-- "This table's sessions still going", ended when its code is renewed or
-- revoked, and "this ticket's", ended when it closes.
CREATE INDEX guest_sessions_table ON guest_sessions (table_id, ended_at);
CREATE INDEX guest_sessions_order ON guest_sessions (order_id);
CREATE INDEX guest_sessions_round ON guest_sessions (round_id) WHERE handoff IS NOT NULL;

-- The session a round was placed with, when it had one: accepting the
-- round, or its payment, binds that session to the ticket the round goes
-- onto. No foreign key, so going back can drop the column; a session
-- pruned since binds nothing, and the phone is made another.
ALTER TABLE guest_rounds ADD COLUMN session_id INTEGER;

-- +goose Down
ALTER TABLE guest_rounds DROP COLUMN session_id;
DROP INDEX guest_sessions_round;
DROP INDEX guest_sessions_order;
DROP INDEX guest_sessions_table;
DROP TABLE guest_sessions;

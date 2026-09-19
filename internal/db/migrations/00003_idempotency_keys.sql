-- Idempotency keys: a command carrying one that was already applied gets the
-- earlier result back instead of being applied again. A terminal on flaky
-- wifi retries a payment; this is what stops it charging twice.
--
-- A key is saved in the same transaction as the events it produced, so it
-- exists exactly when they do. A command that failed saved nothing, and the
-- same key may be tried again.

-- +goose Up

CREATE TABLE idempotency_keys (
    venue_id    INTEGER NOT NULL REFERENCES venues (id) ON DELETE RESTRICT,
    key         TEXT    NOT NULL,

    -- What the key was used for: the command and its body, hashed. The same
    -- key on a different request is a client bug, refused rather than
    -- answered with a result for something else.
    fingerprint TEXT    NOT NULL,

    -- The result, as the events it appended: seq first_seq to last_seq of
    -- the order. Replaying reads them back and projects to last_seq, which
    -- gives the state the command left the order in.
    order_id    INTEGER NOT NULL REFERENCES orders (id) ON DELETE RESTRICT,
    first_seq   INTEGER NOT NULL,
    last_seq    INTEGER NOT NULL,

    created_at  TIMESTAMP NOT NULL,

    PRIMARY KEY (venue_id, key),

    CHECK (length(key) BETWEEN 1 AND 255),
    CHECK (length(fingerprint) > 0),
    CHECK (typeof(first_seq) = 'integer' AND first_seq >= 1),
    CHECK (typeof(last_seq) = 'integer' AND last_seq >= first_seq),
    CHECK (created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00')
) WITHOUT ROWID;

-- +goose Down

DROP TABLE idempotency_keys;

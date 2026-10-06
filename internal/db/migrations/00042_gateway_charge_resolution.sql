-- +goose Up
-- An unapplied charge is money a gateway took that no bill could (ADR-220).
-- A person deals with it: they refund it at the gateway, or record it on a
-- bill by hand. This is where they say which, and who and when (phase 11,
-- Task 4f4, ADR-223). Until then it is in front of staff on every terminal.
--
-- The charge stays unapplied: that is what happened to it. Being dealt with
-- is a second fact, and a charge with neither column set is one nobody has
-- dealt with yet.
ALTER TABLE gateway_charges ADD COLUMN resolution TEXT
    CHECK (resolution IS NULL OR resolution IN ('refunded', 'recorded'));
ALTER TABLE gateway_charges ADD COLUMN resolved_at TIMESTAMP
    CHECK (resolved_at IS NULL OR resolved_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00');
ALTER TABLE gateway_charges ADD COLUMN resolved_by INTEGER;

-- +goose Down
ALTER TABLE gateway_charges DROP COLUMN resolved_by;
ALTER TABLE gateway_charges DROP COLUMN resolved_at;
ALTER TABLE gateway_charges DROP COLUMN resolution;

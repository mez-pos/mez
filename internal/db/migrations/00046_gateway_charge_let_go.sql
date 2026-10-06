-- +goose Up
-- A guest can give up a payment page they opened (phase 11, Task 4g2b,
-- ADR-234). The charge is not ended: its page can still be paid, and a
-- payment still settles as any does. It only stops being a claim on the
-- bill, so the rest of the table is not held until the gateway expires it.
ALTER TABLE gateway_charges ADD COLUMN let_go_at TIMESTAMP
    CHECK (let_go_at IS NULL OR (round_public_id IS NULL
        AND let_go_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00'));

-- A share let go does not stand in the way of the same phone asking for the
-- same amount again: that is a new charge.
DROP INDEX gateway_charges_share_open;
CREATE UNIQUE INDEX gateway_charges_share_open ON gateway_charges (order_id, payer, amount_minor)
    WHERE payer IS NOT NULL AND status = 'open' AND let_go_at IS NULL;

-- +goose Down
-- Shares let go and asked for again would be two open shares of one phone
-- for one amount, which the index as it was refuses: the ones let go are
-- closed as failed, which is how the claim on the bill reads without the mark.
UPDATE gateway_charges SET status = 'failed', settled_at = let_go_at
WHERE let_go_at IS NOT NULL AND status = 'open';
DROP INDEX gateway_charges_share_open;
ALTER TABLE gateway_charges DROP COLUMN let_go_at;
CREATE UNIQUE INDEX gateway_charges_share_open ON gateway_charges (order_id, payer, amount_minor)
    WHERE payer IS NOT NULL AND status = 'open';

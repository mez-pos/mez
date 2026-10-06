-- +goose Up
-- A guest can pay a share of the table's bill (phase 11, Task 4g1, ADR-232):
-- a charge for part of what is owed. Two guests paying the same amount are
-- two charges, where two phones asking for the whole bill are one, so a
-- share names the phone that asked for it. The name is one the phone made
-- up and keeps; it says nothing of who holds the phone.
ALTER TABLE gateway_charges ADD COLUMN payer TEXT
    CHECK (payer IS NULL OR (round_public_id IS NULL AND length(payer) BETWEEN 1 AND 64));

-- A phone asking twice for the same share is asking once: a double tap is
-- one charge.
CREATE UNIQUE INDEX gateway_charges_share_open ON gateway_charges (order_id, payer, amount_minor)
    WHERE payer IS NOT NULL AND status = 'open';

-- +goose Down
-- A share goes back to being a charge for part of a bill with nobody's name
-- on it. Its money, if it is paid, still lands as it would have.
DROP INDEX gateway_charges_share_open;
ALTER TABLE gateway_charges DROP COLUMN payer;

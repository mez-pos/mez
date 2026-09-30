-- +goose Up
-- Sold out for today (phase 10, Task 2d1, ADR-109). An item marked sold
-- out carries the moment the next trading day begins; until then the
-- register shows it sold out, and from then it is on sale again with no
-- edit to the menu. NULL is on sale. Retiring (active) is a different
-- thing: that is off the menu until someone brings it back.

ALTER TABLE items ADD COLUMN sold_out_until TIMESTAMP
    CHECK (sold_out_until IS NULL OR sold_out_until GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]*+00:00');

-- +goose Down
ALTER TABLE items DROP COLUMN sold_out_until;

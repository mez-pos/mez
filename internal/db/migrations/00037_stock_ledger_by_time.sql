-- +goose Up
-- The reports that read the stock ledger over a range of days (wastage,
-- ADR-186; a dish's cost, ADR-189) ask for one kind of movement between two
-- instants. Without this they read a venue's whole ledger each time.

CREATE INDEX stock_movements_kind_at ON stock_movements (venue_id, kind, created_at);

-- +goose Down
DROP INDEX stock_movements_kind_at;

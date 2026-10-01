-- +goose Up
-- The range reports (phase 10, Task 5a, ADR-118) read every order touched
-- in a range of days: opened before its end and last written after its
-- start. updated_at is the time of an order's last event.

CREATE INDEX orders_venue_updated ON orders (venue_id, updated_at);

-- +goose Down
DROP INDEX orders_venue_updated;

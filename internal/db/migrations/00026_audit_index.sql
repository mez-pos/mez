-- +goose Up
-- The audit trail (phase 10, Task 4c, ADR-117) reads order events by type
-- and time: every void, comp, discount, refund, payment void and reopen of
-- a venue in a range of days. The trail is these events, never a copy.

CREATE INDEX order_events_type_at ON order_events (type, at);

-- +goose Down
DROP INDEX order_events_type_at;

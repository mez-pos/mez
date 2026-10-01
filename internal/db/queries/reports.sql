-- The range reports (phase 10, Task 5a, ADR-118).

-- name: OrderEventsTouchingRange :many
--
-- The whole streams of the orders with any event in [from_at, to_at): a
-- refund in the range may be against a payment taken before it. An order's
-- events all fall between its opening and its last write (updated_at).
SELECT e.order_id, e.seq, e.type, e.payload, e.actor_id, e.device_id, e.at, e.shift_id
FROM order_events e
JOIN orders o ON o.id = e.order_id
WHERE o.venue_id = sqlc.arg(venue_id)
  AND o.updated_at >= sqlc.arg(from_at) AND o.opened_at < sqlc.arg(to_at)
ORDER BY e.order_id, e.seq;

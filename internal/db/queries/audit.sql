-- The audit trail (phase 10, Task 4c, ADR-117): corrections read straight
-- from the order events. staff_id 0 is anyone; otherwise it matches the
-- person who did it or the person who authorised it.

-- name: AuditEvents :many
--
-- Newest first, a page at a time: before_id is the last id of the page
-- before, or 0 for the first page.
SELECT e.id, e.order_id, o.public_id AS order_public_id, e.seq, e.type, e.payload, e.actor_id, e.at
FROM order_events e
JOIN orders o ON o.id = e.order_id
WHERE o.venue_id = sqlc.arg(venue_id)
  AND e.type IN ('line.voided', 'line.comped', 'discount.applied', 'refund.issued', 'payment.voided', 'order.reopened')
  AND e.at >= sqlc.arg(from_at) AND e.at < sqlc.arg(to_at)
  AND (sqlc.arg(kind) = '' OR e.type = sqlc.arg(kind))
  AND (sqlc.arg(staff_id) = 0 OR e.actor_id = sqlc.arg(staff_id)
       OR json_extract(e.payload, '$.authorised_by') = sqlc.arg(staff_id))
  AND (sqlc.arg(before_id) = 0 OR e.id < sqlc.arg(before_id))
ORDER BY e.id DESC
LIMIT sqlc.arg(max_rows);

-- name: AuditCounts :many
--
-- How many of each type the same filter finds, over every page.
SELECT e.type, COUNT(*) AS n
FROM order_events e
JOIN orders o ON o.id = e.order_id
WHERE o.venue_id = sqlc.arg(venue_id)
  AND e.type IN ('line.voided', 'line.comped', 'discount.applied', 'refund.issued', 'payment.voided', 'order.reopened')
  AND e.at >= sqlc.arg(from_at) AND e.at < sqlc.arg(to_at)
  AND (sqlc.arg(staff_id) = 0 OR e.actor_id = sqlc.arg(staff_id)
       OR json_extract(e.payload, '$.authorised_by') = sqlc.arg(staff_id))
GROUP BY e.type;

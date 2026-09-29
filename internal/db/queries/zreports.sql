-- Z-reports (ADR-098). Written only by internal/service/shift, in the
-- transaction that closes a shift or issues a correction.

-- name: NextZNumber :one
-- Moves the venue's counter on and returns the number taken.
INSERT INTO z_report_series (venue_id, last) VALUES (?, 1)
ON CONFLICT (venue_id) DO UPDATE SET last = last + 1
RETURNING last;

-- name: CreateZReport :one
INSERT INTO z_reports (venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: ZReportByNumber :one
SELECT id, venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at
FROM z_reports
WHERE venue_id = ? AND number = ?;

-- name: ZReportByID :one
SELECT id, venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at
FROM z_reports
WHERE id = ?;

-- name: LatestZReportForShift :one
-- The shift's report as it now stands: the last correction, or the original.
SELECT id, venue_id, number, shift_id, corrects_id, reason, body, generated_by, generated_at
FROM z_reports
WHERE shift_id = ?
ORDER BY number DESC
LIMIT 1;

-- name: ZReportsBefore :many
-- A page of the venue's reports, newest first, below a number: the
-- cursor is the last number of the page before.
SELECT z.number, z.shift_id, s.public_id AS shift_public_id, s.trading_day, z.corrects_id, z.reason, z.generated_at,
       CAST(json_extract(z.body, '$.report.sales.total.minor') AS INTEGER) AS sales_total_minor
FROM z_reports z
JOIN shifts s ON s.id = z.shift_id
WHERE z.venue_id = sqlc.arg(venue_id) AND z.number < sqlc.arg(before)
ORDER BY z.number DESC
LIMIT sqlc.arg(page_size);

-- name: ItemCategoriesForVenue :many
-- Every item's category, retired ones included: a report names what was
-- sold, even if it is off the menu now.
SELECT i.id AS item_id, c.id AS category_id, c.name AS category_name
FROM items i
JOIN categories c ON c.venue_id = i.venue_id AND c.id = i.category_id
WHERE i.venue_id = ?;

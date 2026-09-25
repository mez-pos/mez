-- The invoice series counters and the register of numbers issued.

-- name: NextInvoiceSeq :one
-- Allocates the next number in a series in one statement: the first call for
-- a key writes 1, every later one adds one. It runs inside the issuing
-- transaction, so a rollback takes the number back. Never MAX(n)+1.
INSERT INTO invoice_series (venue_id, series, fy_start, last_seq)
VALUES (?, ?, ?, 1)
ON CONFLICT (venue_id, series, fy_start) DO UPDATE
SET last_seq = invoice_series.last_seq + 1
RETURNING last_seq;

-- name: InvoiceSeriesLastSeq :one
SELECT last_seq
FROM invoice_series
WHERE venue_id = ? AND series = ? AND fy_start = ?;

-- name: CreateInvoice :one
INSERT INTO invoices (venue_id, order_id, series, fy_start, seq, invoice_no, issued_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: InvoicesForVenue :many
SELECT id, venue_id, order_id, series, fy_start, seq, invoice_no, issued_at
FROM invoices
WHERE venue_id = ?
ORDER BY series, fy_start, seq;

-- name: CreateCreditNote :one
INSERT INTO credit_notes (venue_id, order_id, series, fy_start, seq, credit_no, against_invoice_no, total_minor, issued_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: CreditNotesForVenue :many
SELECT id, venue_id, order_id, series, fy_start, seq, credit_no, against_invoice_no, total_minor, issued_at
FROM credit_notes
WHERE venue_id = ?
ORDER BY series, fy_start, seq;

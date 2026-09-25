-- The print queue: printers and the jobs waiting for them (ADR-080).

-- name: CreatePrinter :one
INSERT INTO printers (venue_id, name, transport, address, paper_mm, active, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?)
RETURNING id;

-- name: PrinterByID :one
SELECT id, venue_id, name, transport, address, paper_mm, active, created_at, updated_at
FROM printers
WHERE id = ?;

-- name: CreatePrintJob :one
INSERT INTO print_jobs (venue_id, printer_id, kind, label, payload, state, attempts, next_attempt_at,
                        last_error, order_id, event_seq, station, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: PrintJobByID :one
SELECT id, venue_id, printer_id, kind, label, payload, state, attempts, next_attempt_at, last_error,
       order_id, event_seq, station, created_at, updated_at, done_at
FROM print_jobs
WHERE id = ?;

-- name: DuePrintJobs :many
-- The next job for each printer, if it is due: the oldest job not yet done
-- or dead. A printer's jobs print one at a time and in order, because a job
-- still printing, or failed and waiting out its backoff, holds back every
-- later one for that printer. Jobs with no printer are never picked.
SELECT j.id, j.venue_id, j.printer_id, j.kind, j.label, j.payload, j.state, j.attempts,
       p.name AS printer_name, p.transport, p.address, p.active AS printer_active
FROM print_jobs j
JOIN printers p ON p.id = j.printer_id
WHERE j.state IN ('queued', 'failed')
  AND j.next_attempt_at <= sqlc.arg(now)
  AND j.id = (
      SELECT min(k.id) FROM print_jobs k
      WHERE k.printer_id = j.printer_id AND k.state IN ('queued', 'printing', 'failed')
  )
ORDER BY j.id;

-- name: ClaimPrintJob :execrows
-- Moves a job the worker picked to printing, and counts the attempt. It
-- matches only from the state the worker saw, so two claims of one job
-- cannot both succeed.
UPDATE print_jobs
SET state = 'printing', attempts = attempts + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = sqlc.arg(from_state);

-- name: FinishPrintJob :execrows
-- Records how a send went. Only a job still printing can finish.
UPDATE print_jobs
SET state = sqlc.arg(state), next_attempt_at = sqlc.arg(next_attempt_at), last_error = sqlc.arg(last_error),
    done_at = sqlc.narg(done_at), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'printing';

-- name: RequeueStalePrintJobs :execrows
-- On start, jobs a crash left printing go back to queued. Whether their
-- bytes reached the printer is unknown; printing twice beats never.
UPDATE print_jobs
SET state = 'queued', next_attempt_at = sqlc.arg(now), updated_at = sqlc.arg(now),
    last_error = 'the server stopped while this was printing'
WHERE state = 'printing';

-- name: ReceiptPrinter :one
-- The venue's receipt printer, if it has one in service.
SELECT id, paper_mm
FROM printers
WHERE venue_id = ? AND is_receipt = 1 AND active = 1;

-- name: StationPrinter :one
-- The printer a station's tickets go to, if it has one in service.
SELECT p.id, p.paper_mm
FROM stations s
JOIN printers p ON p.id = s.printer_id
WHERE s.venue_id = ? AND s.code = ? AND p.active = 1;

-- name: BumpPrintVersion :exec
-- Moves the venue's print version on, in the transaction that changed a job.
INSERT INTO print_versions (venue_id, version)
VALUES (?, 1)
ON CONFLICT (venue_id) DO UPDATE SET version = print_versions.version + 1;

-- name: BumpAllPrintVersions :exec
-- For a change that touches every venue at once: the requeue on start.
UPDATE print_versions SET version = version + 1;

-- name: PrintVersion :one
SELECT CAST(coalesce((SELECT version FROM print_versions WHERE venue_id = ?), 0) AS INTEGER) AS version;

-- name: PrintJobsForVenue :many
-- The queue as staff see it: every job not yet printed, and what printed
-- since the given time, newest first. Payloads stay behind.
SELECT j.id, j.printer_id, coalesce(p.name, '') AS printer_name, j.kind, j.label, j.state, j.attempts,
       j.next_attempt_at, j.last_error, j.order_id, j.station, j.created_at, j.updated_at, j.done_at
FROM print_jobs j
LEFT JOIN printers p ON p.id = j.printer_id
WHERE j.venue_id = sqlc.arg(venue_id) AND (j.state != 'done' OR j.updated_at >= sqlc.arg(since))
ORDER BY j.id DESC
LIMIT sqlc.arg(max_jobs);

-- name: RetryPrintJob :execrows
-- A person's retry: a failed or dead job back to the queue with its tries
-- restored, on the printer given (the one it had, or one set up since).
UPDATE print_jobs
SET state = 'queued', attempts = 0, printer_id = sqlc.arg(printer_id), next_attempt_at = sqlc.arg(now),
    last_error = '', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id) AND state IN ('failed', 'dead');

-- name: CancelPrintJob :execrows
-- A person's cancel: a job that has not printed stops being tried. It is
-- kept, dead, with who cancelled it, and can still be retried.
UPDATE print_jobs
SET state = 'dead', last_error = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND venue_id = sqlc.arg(venue_id) AND state IN ('queued', 'failed');

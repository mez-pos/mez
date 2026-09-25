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
                        last_error, order_id, event_seq, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: PrintJobByID :one
SELECT id, venue_id, printer_id, kind, label, payload, state, attempts, next_attempt_at, last_error,
       order_id, event_seq, created_at, updated_at, done_at
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

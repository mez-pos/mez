# Terminal: Print queue

What the printers are doing, and what did not print. Built in phase 07
Task 5b (ADR-080, ADR-081).

## Purpose

A receipt that never printed is an invoice the guest never got. The queue
makes every job that has not printed visible, says why in words, and lets
the person at the till retry it or stop it.

## Layout

- **In the header**, before the connection indicator, on every terminal
  screen: **Print queue** (ghost) while everything is fine, and **Not
  printed 2** (the danger button, the count in the numeral face) while any job is
  failing or has been given up on. Either opens the sheet.
- **The sheet** (`Sheet`: bottom on touch, side panel with a mouse), titled
  "Print queue": the jobs newest first. Each row:
  - the job's label: "Receipt INV/26-27/000042", "Duplicate ...",
    "Kitchen ticket, table 4"
  - its state as a `StatusChip`: Waiting, Printing, Printed (success),
    Retrying (warning), Not printed (danger)
  - under the label, what is happening: "Waiting for Counter.", "Sending
    to Counter.", "Printed on Counter.", "Counter: connection refused. Try
    3 of 10; next try in 8s." counting down, or the reason it gave up in
    danger ("No printer is set up for this; set one up and retry.",
    "Cancelled by Asha.")
  - **Retry** (a failing or given-up job) and **Cancel** (a job still
    waiting or failing), 48px. A refusal shows under them in words.

## Data

- `GET /api/v1/print-jobs`: every job not printed, and those printed in
  the last two hours. React Query key `["print-jobs"]`.
- The stream's `print_changed` (and a resync) invalidates it: a job
  printing on another till's receipt printer shows here as it moves.
- `POST /api/v1/print-jobs/{id}/retry`: a failed or dead job back to the
  queue with ten fresh tries. One that had no printer takes the one set up
  for it since; if there is still none, "There is still no printer set up
  for this."
- `POST /api/v1/print-jobs/{id}/cancel` (`order.edit`): a waiting job
  stops being tried, kept as Not printed with who cancelled it. It can
  still be retried.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap the header button | Opens the sheet | |
| Retry | Queues it again | Waiting, then Printed when the printer takes it |
| Cancel | Stops it | Not printed, "Cancelled by ..." |

## States

- **Nothing:** "Nothing to print, and nothing printed in the last two
  hours."
- **Loading / error:** the words, in the sheet.
- **Offline:** the list stays as last read; Retry fails in words.

## Constraints

- State is a chip with a word, never colour alone.
- Nothing here is green: the queue is not money. Not printed uses the
  danger button, brick on its tint.
- Every target 48px.

## Reference

The archived design has no print queue; this screen is new.

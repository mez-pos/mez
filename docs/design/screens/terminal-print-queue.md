# Terminal: Print queue

What the printers are doing, and what did not print. Built in phase 07
(ADR-080, ADR-081); where it opens from moved with the rebuild (ADR-148,
ADR-161). `web/terminal/src/print/`: `PrintQueueSheet.tsx`, `print-queue.ts`,
`use-print-jobs.ts`.

## Purpose

A receipt that never printed is an invoice the guest never got. The queue
makes every job that has not printed visible, says why in words, and lets the
person at the terminal retry it or stop it.

## Layout

- **Where it opens from:**
  - **Not printed 2**, a danger pill in the top bar on every screen, there
    only while a job is failing or has been given up on.
  - **Print queue** under the profile menu; for a waiter, under **More**.
  - **Print queue** beside the title on Orders, in danger with "(2 not
    printed)" while anything has failed.
- **The sheet** (`Sheet`: bottom on touch, a side panel with a mouse), titled
  "Print queue": the jobs, newest first. Each row:
  - the job's label, as the server names it: "Receipt INV/26-27/000042",
    "Duplicate ...", "Kitchen ticket, table 4"
  - its state as a chip
  - under the label, what is happening: "Waiting for Counter.", "Sending to
    Counter.", "Printed on Counter.", "Counter: connection refused. Try 3 of
    10; next try in 8s." counting down, or why it was given up on ("No
    printer is set up for this; set one up and retry.", "Cancelled by
    Asha.")
  - **Retry**, for a job failing or given up on, and **Cancel**, for one
    still waiting or failing; 48px. A refusal shows under them in words.

| State | Tone | Word |
|---|---|---|
| queued | neutral | Waiting |
| printing | info | Printing |
| done | success | Printed |
| failed | warning | Retrying |
| dead | danger | Not printed |

## Data

- `GET /api/v1/print-jobs`, key `["print-jobs"]`: every job not printed, and
  those printed in the last two hours.
- The stream's print notice, and a resync, read it again: a job printing on
  another device's receipt printer shows here as it moves.
- `POST /api/v1/print-jobs/{id}/retry`: a failed or given-up job back to the
  queue with ten fresh tries. One that had no printer takes the one set up
  for it since; if there is still none, "There is still no printer set up
  for this."
- `POST /api/v1/print-jobs/{id}/cancel` (`order.edit`): a waiting job stops
  being tried, kept as Not printed with who cancelled it. It can still be
  retried.

## Interactions

| Action | Does | Then |
|---|---|---|
| Not printed, or Print queue | Opens the sheet | |
| Retry | Queues it again | Waiting, then Printed when the printer takes it |
| Cancel | Stops it | Not printed, "Cancelled by ..." |

## States

- **Nothing:** "Nothing to print, and nothing printed in the last two
  hours."
- **Error:** the words, in the sheet.
- **Offline:** the list stays as last read; Retry fails in words.

## Constraints

- State is a chip with a word, never colour alone.
- Nothing here is green: the queue is not money. Not printed is danger.
- Every target 48px.
- The pill is drawn only when something is wrong: a quiet top bar means every
  receipt came out.

## Reference

The design draws no print queue; this screen is ours, kept under rule 4. It
is where the design has no place for it: under the profile, as the flow map
says, and on Orders, beside the bills it prints.

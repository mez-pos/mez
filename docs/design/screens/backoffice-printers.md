# Back office: Printers

The venue's printers, and where receipts and each station's tickets print.
Built in phase 07 (ADR-080, ADR-081) and laid out again on Task 16's patterns
in the rebuild's Task 16k (ADR-195). `web/backoffice/src/printers/`:
`PrintersScreen.tsx`, `PrinterTable.tsx`, `PrinterRouting.tsx`,
`PrinterFormModal.tsx`, `printers-logic.ts`, `use-printers.ts`. Section
**Printers**, path `/printers`, `settings.edit`, keys `g t`. Settings links
here from its rail.

## Purpose

Tell the venue which printer is where, send a test page, and know from the
last job whether each one is working.

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Printers" in the serif, a line of counts under it ("3
  printers, 1 retired", or "None yet"), and **Add a printer** at the right.
- A line over the table for what an action was refused for, named for the
  printer.
- **The table**, in a card, in the order added:

  | Column | Shows |
  |---|---|
  | Printer | its name, and how it is reached in the numeral face: `10.0.0.5:9100`, or `USB: EPSON TM-T82` for one this computer drives through Windows |
  | Paper | "80mm" or "58mm" |
  | Prints | "Receipts", and each station's tickets ("Kitchen, Bar"); "Retired" muted for a retired one |
  | Last print | the newest job's state as a chip, with when ("12 min ago"); or "Nothing printed yet" |
  | | **...**: Test page, Edit, and Retire or Bring back |

- **Where things print**, a card beside it from 1280px, at the back
  office's side width (`w-aside`, 307 to 380px; TECH-17), and under the
  table below that: **Receipts**, then a row per
  station ("Kitchen tickets"), each a select of the printers in service and
  **None**. A change saves at once and says "Saved". With no stations: "No
  stations yet. They are added in the menu, with the items made at each."
- **Add, Edit** (a modal): **Name**; **How it connects**, "Network (IP
  address)" or "USB, on this computer (Windows)"; the **IP address** ("Like
  192.168.1.50. Port 9100 unless you add another.") or **The printer's name
  in Windows**; **Paper**, 80mm or 58mm; and, when adding, **What it
  prints**: Receipts, each station's tickets, or Decide later, starting on
  what nothing prints yet (receipts, then the first station with no
  printer) (BO-15); **Save**. Errors under their field.

## Data

- `GET /api/v1/printers`, key `["printers"]`: `printers[]` (id, name,
  transport, address, paper_mm, active, is_receipt, stations[], last_job)
  and `stations[]`. Read again after every change and on the stream's print
  notice, so a test page or a failing receipt moves Last print live.
- `POST /api/v1/printers`, `PUT /api/v1/printers/{id}`, `POST
  .../{id}/retire`, `.../restore`.
- `PUT /api/v1/printers/receipt` `{printer_id}` and `PUT
  /api/v1/stations/{code}/printer` `{printer_id}`; null is none.
- `POST /api/v1/printers/{id}/test`: a test page through the print queue, so
  it retries and shows on the terminals' queue like any other job.

## Interactions

| Action | Does | Then |
|---|---|---|
| Add a printer, Save | Creates it, then sets it to what it prints | Its row appears, and Where things print shows it. If setting it fails, the printer is kept and the screen says so: pick it under Where things print |
| Edit, Save | Changes its name, connection or paper | Jobs already queued keep their bytes (ADR-080) |
| Test page | Queues one | A toast, "Test page queued. Check the printer."; Last print follows it |
| Retire | Stops sending to it | "Counter retired." Its queued jobs are given up on; it stops printing receipts; its stations print nowhere until given another |
| Bring back | Puts it in service | "Counter is back in service." It does not take back the receipts or stations |
| A select under Where things print | Sets that printer | "Saved" |

## States

- **Loading:** "Loading the printers".
- **None:** "No printers yet. Add the receipt printer first, then one for
  each station that needs tickets." and **Add a printer** in the card, at
  the venue (BO-15).
- **Error:** the server's words, with **Try again**.
- **Refused:** a line over the table, or under the field in the form.
- **Not allowed:** the frame's "That is not yours to open".

## Constraints

- No green primary: nothing here moves money. The form's Save is its own.
- A printer has no chip of its own. The server cannot tell whether a printer
  is reachable; its last job is the evidence, so the row shows that job's
  chip.
- What an action answered is a toast; what it was refused for is a line over
  the table.
- Retire has no confirm: it is one click back.
- Printing is ASCII only (ADR-079); a non-Latin name prints as "?".

## Reference

The design draws no printers screen: D22 lists "Printers & devices" as a
section of Settings and draws nothing under it. This one is ours, kept under
rule 4, on the table pattern of D19 and D20 so it reads as the same app
(ADR-195).

# Back office: Printers

Where an owner sets up the venue's receipt and kitchen printers, says which
prints receipts and which prints each station's tickets, and checks each
one works. Built in phase 07 Task 5c (ADR-082). The back office's first
real screen.

## Purpose

Get every ticket and receipt onto the right paper before service, and see
at a glance which printer is failing during it.

## Layout

Paper surface, inside `SignedInShell`. The back office home links here
("Printers") until the menu editor and reports arrive in phase 10.

- **Header row:** the title "Printers" in Instrument Serif, and **Add a
  printer** on the right.
- **Printers**, one `Card` each, in the order added:
  - the name (lg, semibold); a "Receipts" chip when it prints receipts, and
    a "Retired" chip (struck) when retired
  - how it is reached, in mono: `10.0.0.5:9100`, or `USB: EPSON TM-T82`
    for a printer this computer drives through Windows; the paper, "80mm"
    or "58mm"
  - the stations whose tickets it prints: "Kitchen, Bar", or "No stations"
  - its **last result**: the newest job's state as a `StatusChip`, its
    label and when; or "Nothing printed yet"
  - **Test page**, **Edit**, and **Retire** (or **Bring back** when retired),
    44px
- **Where things print** (a `Card`): the receipt printer, then one row per
  active station, each a native select of the active printers plus "None".
  A change saves at once and says "Saved".
- **Add / Edit** (`Modal`): name; **How it connects**: "Network (IP
  address)" or "USB, on this computer (Windows)"; the address or the
  Windows printer's name; paper 80mm or 58mm; Save. Errors under the field
  they belong to.

## Data

- `GET /api/v1/printers`: `printers[]` (id, name, transport, address,
  paper_mm, active, is_receipt, stations[], last_job?) and `stations[]`
  (code, name, printer_id?). React Query key `["printers"]`, invalidated
  after every change and on the stream's `print_changed`, so a test page
  or a failing receipt moves the last result live.
- `POST /api/v1/printers` `{ name, transport, address, paper_mm }`,
  `PUT /api/v1/printers/{id}` with the same, `POST .../{id}/retire`,
  `POST .../{id}/restore`.
- `PUT /api/v1/printers/receipt` `{ printer_id }` (null: none) and
  `PUT /api/v1/stations/{code}/printer` `{ printer_id }`.
- `POST /api/v1/printers/{id}/test`: a test page (`escpos.TestPage` at the
  printer's paper width) through the print queue, so it retries and shows
  on the terminals' queue like any other job.
- Every call needs `settings.edit`.

## Interactions

| Action | Does | Then |
|---|---|---|
| Add a printer, Save | Creates it | The card appears; set where it prints below |
| Edit, Save | Changes name, connection or paper | Jobs already queued keep their bytes (ADR-080) |
| Test page | Queues a test page | Last result: Waiting, then Printed or Retrying |
| Retire | Stops sending to it | Its queued jobs go dead with "retired"; it stops printing receipts; its stations print nowhere until given another |
| Bring back | Makes it active again | It does not take back the receipts or stations |
| Receipt printer select | Sets the one receipt printer | "Saved" |
| A station's select | Sets that station's printer | "Saved" |

## States

- **No printers:** "No printers yet. Add the receipt printer first, then
  one for each station that needs tickets."
- **A retired printer** stays listed, struck chip, out of the selects.
- **Refused:** the server's words under the field or the select: "A
  printer with that name already exists", "Give the IP address, like
  192.168.1.50", "Choose 80mm or 58mm".
- **Loading / offline:** the words in place of the list.

## Constraints

- One accent: **Add a printer** is the primary button; nothing else is
  vermilion (the paper accent).
- Every control 44px on this screen too: it is used on a tablet as often as
  a laptop.
- Addresses and paper widths in mono.

## Reference

The archived back office design has no printer settings; this screen is
new, built from the primitives in `docs/design/components.md`.

# Terminal: Close the day

The end of a shift: the drawer is counted blind, checked against what was
expected, explained when it is out, and closed. The Z-report follows. Built in
phase 09 (ADR-093 to ADR-095) and laid out again from D23 in the rebuild's
Task 16j (ADR-194). `web/terminal/src/shift/CloseDayScreen.tsx`,
`DrawerCounter.tsx`, `ClosingStock.tsx`, `shift-logic.ts`, `use-shift.ts`;
carried at `/shift/close` by `shell/pages.tsx`.

Opening the shift is `terminal-open-shift.md`.

## Purpose

Nobody at the drawer learns what it should hold until they have said what it
does hold. A closing takes a manager under three minutes with a full drawer.

## Layout

Reached from **Close the shift** under the profile, for someone with
`shift.manage` while a shift is open. Inside the frame, in two steps. A rail
on the right from 1024px; under that the rail's facts are one line under the
title.

**The head, both steps:** "Step 1 of 2" or "Step 2 of 2" in small caps,
**Close the day** in the serif, and a line saying what this step is for.

**Step 1, count the drawer:**

- **The counter** (`DrawerCounter`), titled "Count the drawer": a row for each
  note and coin of the venue's currency, largest first: the face, the count,
  and the row's value. The picked row has the green border and tint. Beside
  the rows the pad (`NumPad`, amount layout, large) and **Next**. Under them,
  "Counted" and the total.
- Nothing on this step says, hints or rounds towards what is expected.
- **The foot, kept in sight:** the error line, **Back to the order screen**,
  and **Submit count** (the primary, 56px).

**Step 2, reconcile and close**, which is also what a counted shift opens on
after a reload or on another device:

- **Cash reconciliation**, a card: Opening float; **Expected in drawer**;
  Counted; then how it came out as a word in its own tint, never a colour
  alone: "Balanced" (green), "Short ₹9.50" (danger), "Over ₹282.00"
  (mustard), with a line under it saying whether that is within what the
  venue allows.
- **The note**, in the same card. Past the venue's limit it is owed: "₹282.00
  is more than the ₹100.00 allowed. Say why before closing.", a field
  labelled "Why?" (up to 500 characters) and **Save note**. Within the limit
  the field is "A note on the count, if there is one". A saved note shows as
  a quote: "Saved with the count. It can't be changed."
- **Closing stock**, when the back office has items marked Count at close
  (below).
- **The foot:** the error line and **Close the day** with a padlock (the
  primary), waiting while a note or a stock count is owed.

**The rail, 22rem:** "The shift" in small caps; Trading day, Opened, Shift
(its public id) and Opening float; on step 1, "Counted" with the counter's
total as the one total on the screen; a script line at the foot.

## Closing stock (ADR-114)

- Listed once the drawer is counted: **Count the stock**, "How much of each
  is there now? The record is shown once the count is in.", and a field for
  each item in its unit, tabular, right-aligned. **Blind**, as the drawer is.
- **Save the stock count** sends every item together. An item left empty, or
  typed in words, is named under its own field before anything is sent. It is
  taken once per shift.
- Once it is in: **Closing stock**, each item's count and what it moved
  against the record, a loss in the danger colour, and "The change is the
  count against what the stock record said."
- With nothing marked, the section is not shown.

## Data

- `GET /api/v1/shift`, key `["shift"]`: the open shift, with `counted`,
  `reconciliation` and `reason` once they exist. Read on start, every 30
  seconds, and from every command's answer.
- `POST /api/v1/shift/{id}/count` `{total, denominations}`: its answer is the
  first place `reconciliation` appears.
- `POST /api/v1/shift/{id}/reason` `{reason}`.
- `POST /api/v1/shift/{id}/close`: needs `shift.manage`. Its answer carries
  `z_report`, the number of the report it issued.
- `GET` and `POST /api/v1/shift/{id}/stock-count`: needs `shift.manage`.
- One `Idempotency-Key` per attempt, reused on a retry of the same body.
- Denominations are sent as counted, rows with nothing in them left out; the
  server adds them up again and refuses a total that does not match.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a row | Picks it | The pad types its count |
| Pad keys | Set the picked row's count | The row's value and the total redraw |
| Next | Picks the row below | At the last row, stays |
| Back to the order screen | Leaves, nothing recorded | Selling goes on |
| Submit count | Asks first: "The count can't be changed once it's in. Submit ₹9,520.00?" | On yes, `shift.counted`; step 2. The gate now holds every path here |
| Save note | Records the reason | The quote; Close the day no longer waits for it |
| Save the stock count | Records the counts as stock movements | What each moved |
| Close the day | Closes the shift, issues the Z-report and its exports | A toast, "Shift closed."; Open the shift, with "Z-report 4 issued" and View the report |

## States

- **Loading:** "Checking the shift", from the gate.
- **Offline:** nothing is counted or closed offline. The refusal is a line in
  the foot and what was keyed stays.
- **Counted elsewhere:** a device that submits after another did gets "The
  drawer has already been counted." and, on the next read, step 2.
- **A currency with no table of notes:** "Type the total counted on the
  pad.", and the pad types one total.
- **A report that cannot be built:** the close is refused and says why; the
  shift stays open (ADR-097).
- **Permission denied:** Close the shift is not in the profile menu without
  `shift.manage`; the server refuses the close as well.

## Constraints

- **Blind:** the expected figure exists on this screen only in step 2, and
  only from the server's answer to the count.
- **The count is final.** A counted drawer cannot be recounted, and a saved
  note cannot be changed.
- The only arithmetic in the UI is adding up the counter for display; the
  server's total is the one recorded.
- One primary per step: Submit count, then Close the day. Both stay at the
  foot under a long closing stock list.
- Every number is tabular and right-aligned; every target 48px, the pad's
  keys 64px.
- Open orders are not closed with the shift; they carry into the next one.

## Reference

`mez Desktop Management.dc.html`, D23 Z Report (line 707). The design is the
close and the report side by side on one screen; they are two here (ADR-194):
there is no report until the shift is closed, so its slip is not drawn beside
the steps, and the Z-report with its slip is the screen after
(`zreport.md`). The design shows the expected figure above the count; the
count here is blind, so step 1 has no figure to copy.

Not in the design and kept: closing stock, and the note's limit in words.

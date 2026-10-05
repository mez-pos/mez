# Z-report: on the terminal and in the back office

A closed shift's Z-report on screen: one view of the one stored document,
shown on the terminal right after a close and in the back office's list of
reports. Built in phase 09 (ADR-097 to ADR-100, ADR-104) and laid out again
from D23 in the rebuild's Task 16j (ADR-194). `web/shared/zreport/`:
`ZReportScreen.tsx`, `ZReportView.tsx`, `ZPaper.tsx`, `ZTables.tsx`; in the
back office `reports/ZReportsScreen.tsx` and `ExportsPanel.tsx`.

| Where | Path | Needs |
|---|---|---|
| Terminal, after a close | `/shift/z/:n` | the close itself |
| Back office, the list | `/backoffice/reports`, keys `g r` | `reports.view` |
| Back office, one report | `/backoffice/reports/:n` | `reports.view` |

Counting the drawer and closing is `terminal-close.md`.

## Purpose

The owner reads last night's figures without the paper, finds any past day's
report in seconds, prints a copy, and sees what a correction changed and why.
Every figure is the stored report's: the screen adds nothing up.

## Layout

**The report** (`ZReportView`, shared by both apps): the report in full on the
left, and from 1280px a slip of paper beside it, 380px, that stays in sight as
the report scrolls. Under 1280px the slip sits under the report.

- **The head:** "Z-report 12" in the serif; under it the trading day, "Opened
  18:02, closed 01:47" in the venue's time, and "Closed by Priya". At the
  right, the way back, **Print a copy**, and in the back office **Tips**.
- A correction carries a warning band: "Corrects Z-report 11." and its
  reason.
- **Sales:** Bills, Gross, Discounts (with their count), Net, Taxable value,
  Tax, then **Total**; with credit notes, "Less credit notes" and **Net
  sales**. Invoices issued, and "First to last" with the two numbers.
- **Tax:** a row per component and rate ("CGST 2.5%"): taxable value and
  tax. These rows are the tax invoiced less the tax credited, so with a
  credit note in the shift they come to less than the Sales card's Tax, and
  a line under them says so: "After 1 credit note: the tax invoiced, less
  the tax credited."
- **By category**, **By channel**, **By how it was served**, **By staff**: a
  table each: Name, Units or Bills, Gross, Discounts, Total. **By item** is
  the same, folded after ten rows.
- **Payments:** each method with its count and net, and what was voided and
  refunded on it; **Tips, apart from sales**, and under it "Tips on payments
  taken by Kabir" for each person; when payments and sales differ, "Paid for
  bills of other shifts".
- **Exceptions:** Comps, Voids, "Removed in error by Kabir" for each person,
  Credit notes, Refunds, each a count and an amount; Reprints.
- **Cash drawer:** Float, Cash in, Cash out, Cash taken, Cash voided and
  refunded, **Expected**, "Counted by Priya", and the difference as a word
  and a colour ("Short ₹9.50", "Over ₹282.00", "Balanced"), with the reason
  quoted under it.

**The slip** (`ZPaper`, D23), "Z-report 12, in brief": the venue and the
time, then four blocks between dashed rules, in the numeral face:

- Gross sales, Discounts, Net, each tax, **Total**, Tips collected.
- Each payment method and what it took.
- Orders, Covers (where the report has them), Avg. order, Refunds and Voids
  as "2 · ₹556.00".
- Expected drawer, Counted, and **Variance** in words.

**On the terminal:** after **Close the day**, Open the shift shows a card:
"Z-report 12 issued. It is printing." with **View the report**. The report
takes the screen with **Back** and **Print a copy**. Only the device that
closed shows the card; the back office has every report.

**In the back office:**

- **Z-reports**, the list: the title, "One is issued each time a shift
  closes. Newest first.", then a row per report: "Z 12" in the numeral face,
  the trading day, the shift's id, the sales total, and **Correction** as a
  warning label on one. **Older reports** loads the next fifty. A row opens
  the report, with **All reports** back to the list.
- **Exports**, under a report: a card per file the close wrote: **Tax
  register** ("Every invoice and credit note by the columns your tax regime
  reports."), **Tally vouchers**, **Day's sales**, each with its file name
  and size and **Download**.

## Data

- `GET /api/v1/z-reports?before=`: a page of 50, key `["z-reports",
  before]`; `next_before` for the next page.
- `GET /api/v1/z-reports/{number}`, key `["z-report", number]`, kept for
  good: a report never changes.
- `POST /api/v1/z-reports/{number}/print`: the copy goes on the print queue.
- The close's answer carries `z_report`, the number the terminal shows.
- The shift's export files, listed and downloaded as CSV (ADR-104).
- `GET /api/v1/venue`, for the name on the slip.

## Interactions

| Action | Does | Then |
|---|---|---|
| View the report (terminal) | Opens the report | Back returns to Open the shift |
| Print a copy | Queues a COPY | A toast, "Printing a copy of Z-report 12." |
| A row (back office) | Opens that report | |
| Older reports | Loads the next page | More rows under |
| Tips | Opens the shift's tips | `tips.md` |
| Download | Saves that file | A failure is a toast; the screen stays |

## States

- **Loading:** "Loading the report", "Loading the reports", "Loading the
  files".
- **No reports yet:** "No Z-reports yet. One is issued each time a shift
  closes."
- **No payments:** "No payments in this shift."
- **No exports:** "This shift has no export files. Shifts closed before
  exports existed have none."
- **Not allowed:** "You don't have permission to see reports. Ask a
  manager."
- **Not found:** "There is no Z-report 99."
- **Offline:** "Can't reach the mez server to load the report." with **Try
  again**.
- **A report from before the newer figures** (service types, tips by person,
  covers): those rows are left out, not shown as zero.

## Constraints

- No arithmetic: every figure, total and count is the document's. The one
  sum on screen is the slip's average order, its total over its bills.
- Every number tabular and right-aligned; every target 48px.
- No primary: nothing on a report moves money.
- A difference is a word and a colour, never colour alone.
- A report is issued once and never edited; a mistake is a correction, a new
  report that names the one it corrects. No screen issues a correction yet
  (the API does).
- Categories are the menu's when the report is built, not when the item was
  sold. The printout has no by-item lines; the screen does.

## Reference

`mez Desktop Management.dc.html`, D23 Z Report (line 707). The design is the
close and the slip side by side on one screen. They are two screens here
(ADR-194): there is no report until the shift is closed, so the count and
the reconciliation are `terminal-close.md`, and the slip is beside the
report it summarises. Left out, and why:

- **Wallet** among the payments: there is none.
- **"Counter 1" under the venue on the slip:** a report is the venue's
  shift, not one device's.
- **"Close day & lock terminal":** the close is the step before; Lock is
  under the profile.
- **The slip in a monospace face:** the sheet has none. Figures are the
  numeral face, which lines them up (tokens.md).

Changed: the slip's "Net sales" row is **Net**, then the taxes and a
**Total**, since the design's slip has no total.

Kept, though not drawn (rule 4): the report in full beside the slip, with
its tables by category, channel, service, staff and item; the Z-reports
list; Exports; corrections; Print a copy.

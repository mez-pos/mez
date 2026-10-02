# Z-report: on the till and in the back office

A closed shift's Z-report on screen (phase 09, Task 3d; ADR-097 to
ADR-100). One view of the one stored document, shown on the till right
after a close and in the back office's list of reports.

## Purpose

The owner reads last night's figures without the paper, finds any past
day's report in seconds, prints a copy, and sees what a correction
changed and why. Every figure is the stored report's: the screen adds
nothing up.

## Layout

- **The report** (`ZReportView`, shared by both apps), one column, 48rem:
  - **Header:** "Z-report 12" in Instrument Serif; under it Meta: the
    trading day, "Opened 18:02, closed 01:47" in the venue's time, "Closed
    by Priya". A correction carries a warning band: "Corrects Z-report 11"
    and its reason. **Print a copy** (secondary) at the right.
  - **Sales** card: Bills, Gross, Discounts (with their count), Net,
    Taxable value, Tax, then **Total** as the one total; with credit notes,
    "Less credit notes" and **Net sales**. Invoices issued, first and last
    number, in mono.
  - **Tax** card: one row per component and rate ("CGST 2.5%"): taxable
    value and tax, in columns.
  - **By category**, **By channel**, **By staff**: a table each (name,
    units or bills, gross, discounts, total). **By item** is the same,
    collapsed behind "Show all 38 items".
  - **Payments** card: each method with its count, taken, voided,
    refunded and net; **Tips, apart from sales**; when payments and sales
    differ, "Paid for bills of other shifts" with the difference.
  - **Exceptions** card: comps, voids, removed in error by who removed
    them, credit notes, refunds, reprints: each a count and an amount.
  - **Cash drawer** card: float, cash in, cash out, cash sales, expected,
    counted, and the difference as a word and a colour ("Short Rs. 9.50",
    "Over", "Balanced"), with the reason quoted under it.
- **On the till:** after **Close the shift**, the till-closed screen shows
  a card above the float: "Z-report 12 issued. It is printing." with **View
  the report**. The report takes the register's place with **Back** and
  **Print a copy**. Only the till that closed shows the card; the back
  office has every report.
- **Exports** (back office only), under the report: a card per file the
  close wrote (Task 5b, ADR-104): **Tax register**, **Tally vouchers**,
  **Day's sales**, each with a line on what it holds, its file name and
  size in mono, and **Download**, which saves the CSV. A shift closed
  before exports existed says it has none.
- **In the back office:** **Z-reports** on the home page opens the list:
  newest first, each row the number in mono, the trading day, the shift id,
  the sales total, and "Correction" as a chip on a correction. **Older
  reports** loads the next 50. A row opens `/reports/12`: the report, with
  **All reports** back to the list.

## Data

- `GET /api/v1/z-reports?before=`: a page of 50, key
  `["z-reports", before]`; `next_before` for the next page.
- `GET /api/v1/z-reports/{number}`, key `["z-report", number]`, kept
  forever: a report never changes.
- `POST /api/v1/z-reports/{number}/print`: 204; the copy is on the queue.
- The close's answer carries `z_report`, the number the till shows.
- Reading needs reports.view.

## Interactions

| Action | Does | Then |
|---|---|---|
| View the report (till) | Opens the report | Back returns to the closed till |
| Print a copy | Queues a COPY | "Printing a copy of Z-report 12." |
| A row (back office) | Opens that report | |
| Older reports | Loads the next page | More rows under |
| Show all items | Opens the by-item table | |

## States

- **Loading:** "Loading the report" / "Loading the reports".
- **No reports yet:** "No Z-reports yet. One is issued each time a shift
  closes."
- **Not allowed:** "You don't have permission to see reports. Ask a
  manager."
- **Not found:** "There is no Z-report 99."
- **Offline:** "Can't reach the mez server to load the report." with
  Try again.

## Constraints

- No arithmetic: every figure, total and count is the document's.
- Every number mono and right-aligned; every target 44px.
- One accent per screen at most, and here none is needed: nothing on a
  report moves money.
- A difference is a word and a colour, never colour alone.

## Reference

None in the archived design; this screen is new.

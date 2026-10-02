# Back office: Sales reports

A range of trading days added up: what was sold, how it was paid, where it
came from. Built in phase 10 Task 5c over the server of Tasks 5a and 5b
(ADR-118, ADR-119). Task 5d added what sold, who sold it and when; Task
5e2 the corrections (ADR-122); Task 5f adds closing stock. They are
sections of this one screen, under the same range.

## Purpose

Answer "how did last week go?" in one look, and hand the accountant a file
whose numbers are the ones on the screen.

## Layout

Paper, inside the back office shell, section **Sales reports**, path
`/sales`, under Reports in the navigation, `reports.view`, keys `g s`.

- **Header:** "Sales reports" in Instrument Serif; under it, one line in
  `fg-3`: "A sale counts on the trading day its bill was invoiced. A
  trading day runs 04:00 to 04:00."
- **Range,** one row that wraps:
  - Quick ranges, a row of toggles (`aria-pressed`): **Today**, **7
    days**, **30 days**, **This month**. The last seven days, today
    included, by default. The pressed one is the one the dates match;
    none is pressed for a range typed by hand.
  - **From** and **To**: trading days (`<input type="date">`), mono.
  - **By**: Day, Week or Month, how sales over time is cut. It follows
    the range until it is chosen by hand: a day up to 31 days, a week up
    to 26 weeks, a month beyond.
  - The range applies as it changes. While a new range is read the old
    figures stay, dimmed.
- **The range,** the headline figures in a card:
  - The one total on the screen: **Sales**, the total of the bills
    invoiced, tax included, in Instrument Serif; under it "12 bills ·
    average 285.00" in `fg-3`, mono figures.
  - Beside it a short table of figures, label left, amount mono right:
    Gross, Discounts, Net sales, Tax, Credit notes (with their count),
    Sales after credit notes, Refunds (with their count), Tips.
  - **Download CSV** saves the `summary` sheet.
- **Sales over time,** a card:
  - Bars, one per period, 128px tall, drawn with plain elements: each bar's height
    is its share of the tallest period's total. Slate, the neutral data
    series (tokens.md); no accent. A period with no sales is a hairline.
    The first and last period are labelled under the bars. The bars are
    a picture of the table below and hidden from screen readers.
  - Under the bars, the table: Period ("1 Oct", or "28 Sept to 4 Oct"),
    Bills, Net, Tax, Total. It scrolls inside the card past twelve rows.
  - **Download CSV** saves the `over_time` sheet.
- **How it was paid** and **Where it came from,** two cards side by side
  on a wide screen:
  - Payments: Method, Tenders, Net (taken less voided and refunded), and
    its share of every method's net as a percentage with a slate bar.
  - Channels: Channel, Bills, Total, and its share of the total.
  - Each has **Download CSV** (`payments`, `channels`).
  - A share is a whole percentage of the listed rows; shares are not
    shown when the figures they would divide are zero or negative.

- **What sold,** product mix: Item, Qty, Gross, Discounts, Net, Tax,
  Total, and its share of the total. Sorted by Total, largest first;
  the Item, Qty and Total headers sort it (a header pressed again turns
  it round). It scrolls inside its frame past twelve rows. Items are
  named as the menu stands now. **Download CSV** saves `items`.
- **By category,** category performance: the same columns under
  Category, the same sorting. **Download CSV** saves `categories`.
- **Who sold it,** staff performance, by who opened the bill: Opened by,
  Bills, Gross, Discounts, Total, Average bill, and their share of the
  total. Sorted by Total; Opened by, Bills and Total sort it.
  **Download CSV** saves `staff`.
- **When it sold,** the hour by weekday heatmap, a table: a row per
  weekday, Monday first, all seven; a column per hour of the clock from
  the first hour with a bill to the last, in the order the trading day
  runs them (04:00 first, 03:00 last), so 01:00 is at the right, after
  the evening it belongs to. A weekday is the trading day's.
  - A cell with bills shows their count, mono, over a slate tint in four
    steps by its total against the busiest cell's. The count is the
    figure; the tint only repeats it. An empty cell is a sunken blank
    the same size, so the grid holds its shape.
  - Each cell says the whole of it to a screen reader and on hover:
    "Friday 13:00, 3 bills, 630.00".
  - Under it, one line: the busiest hour in words, "Busiest: Friday
    13:00, 3 bills, 630.00".
  - **Download CSV** saves `hours`.

- **Corrections,** the discount and void analysis: every void, comp,
  discount, refund, payment void and reopen in the range, added up. It
  is the audit trail read at a distance, and every row opens the trail
  on what it counted.
  - One line first: "8 corrections worth 330.00", figures mono, and under
    it in `fg-3`: "What each was worth to its bill, not money out of the
    drawer. A reopen counts and is worth nothing."
  - **By kind:** Kind, Count, Amount; every kind, none included. A kind
    with any is a link to the audit trail for the range and that kind.
  - **By reason:** Kind, Reason (a code in words, "Wrong item"; "No
    reason given" muted when empty), Count, Amount; largest first.
  - **Who did them** and **Who let them through:** the person, Count,
    Amount, and what they were in words under the name ("2 voids · 1
    comp"); largest first. A person on the staff is a link to the trail
    for the range and that person. Nobody on record reads "A guest" as
    who did it and "Not recorded" as who let it through, muted, and
    links nowhere, as does someone no longer on record.
  - The trail finds a person by what they did or what they authorised
    (ADR-117), so a person's link may show a few more rows than the
    count beside it. A note under the two tables says so.
  - **Download CSV** on By reason (`reason`), Who did them (`by`) and
    Who let them through (`authorised_by`).
  - It reads on its own, so the sales above show while it adds up.

## States

- **Nothing in the range:** the headline says zero in every figure; in
  place of the bars, "No sales between 24 Sept and 30 Sept."; the mix
  cards say "Nothing was paid" and "No bills"; the tables of what sold
  say "Nothing sold"; in place of the heatmap, "No bills to place".
- **A day cleared:** "Pick a first and a last day." in place of the cards.
- **Loading:** "Adding up the range" in place of the cards. While the
  last range's figures are shown dimmed, Download CSV is off: a file must
  match its screen.
- **Offline / refused:** the words in place of the cards, with Try again;
  a refused range (over a year, To before From) says the server's words.
- **No corrections in the range:** "No corrections between 24 Sept and
  30 Sept." in place of its tables.
- **Corrections cannot be read:** the words in place of its tables, with
  Try again; the sales above are unaffected.
- **A download that fails** says so in a toast; the screen stays.

## Data

- `GET /api/v1/reports/sales?from=&to=&every=`, keyed by the range, the
  last answer kept while the next is read.
- `GET /api/v1/reports/sales/csv?from=&to=&every=&report=` on a download:
  `CsvResponse.text` saved as `filename`.
- `GET /api/v1/audit/analysis?from=&to=`, keyed by the range, and
  `GET /api/v1/audit/analysis/csv?from=&to=&by=` on a download.
- A link to the trail is `/audit?from=&to=&kind=` or `&staff=`.

## Constraints

- No accent on this screen: nothing on it is an action that moves money.
- Every number mono, every amount right-aligned and through `Money`.
- The screen adds nothing up. Every figure is the server's; the only
  arithmetic here is a bar's height, a share, a cell's tint, and an
  average bill (the total over the bills, to the nearest minor unit).
- Sorting a table reorders the server's rows; it changes no figure.
- No chart library: the bars are elements with a height.

## Reference

Built from `docs/design/components.md` and the paper tokens. The archive
was not consulted.

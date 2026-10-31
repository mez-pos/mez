# Back office: Sales reports

A range of trading days added up, on three tabs: what was sold and how it was
paid, how each item did and how the stock stands, and who sold it. Built in
phase 10 (ADR-118 to ADR-126) and rebuilt from D15, D16 and D17 in the
rebuild's Tasks 16c to 16e (ADR-184 to ADR-189). `web/backoffice/src/sales/`:
`SalesScreen.tsx`, `ItemsScreen.tsx`, `StaffScreen.tsx`, `SalesHead.tsx`,
`RangePill.tsx`, `SalesCharts.tsx`, the tables under "In detail"
(`SalesSections.tsx`, `SalesTables.tsx`, `SalesCorrections.tsx`,
`SalesClosingStock.tsx`), `report-range.tsx`, `use-range-sales.ts`,
`use-csv-download.tsx`. Section **Sales reports**, `reports.view`, keys
`g s`.

| Tab | Path | Design |
|---|---|---|
| Sales | `/sales` | D15 |
| Items and inventory | `/sales/items` | D16 |
| Staff | `/sales/staff` | D17, the staff half |

## Purpose

Answer "how did last week go?" in one look, and hand the accountant a file
whose numbers are the ones on the screen.

## The head, on every tab

- **The tabs** as pills: **Sales**, **Items and inventory**, **Staff**.
- **The title** in the serif ("Sales reports", "Item and inventory reports",
  "Staff reports"), and under it "Insight into 7 days of trading".
- At the right, **the range** as one pill, "29 Sep – 5 Oct 2026" with a
  chevron. It opens:
  - the quick ranges as toggles: **Today**, **7 days**, **30 days**, **This
    month**. The pressed one is the one the days match; none is pressed for
    days typed by hand;
  - **From** and **To**, trading days;
  - on Sales, **By**: Day, Week or Month, how the range is cut. It follows
    the range until it is chosen by hand;
  - "A sale counts on the trading day its bill was invoiced. A trading day
    runs 4:00 AM to 4:00 AM.", the venue's own day start (ADR-385); the quick
    ranges are trading days on the venue's clock."
- **Export**, beside it: a menu, "Download as CSV", listing that tab's files.

The range is shared by the three tabs and starts at the last seven days,
today included. It applies as it changes; while a new range is read the old
figures stay, dimmed.

## Sales (D15)

- **Four figures**, as the dashboard's: **Total sales** (on the green
  tint), **Total orders**, **Average order value**, **Total guests**, each
  against the same number of days just before ("8% up on the 7 days
  before").
- **Sales trend**, with a legend: a bar for each period, stacked by how it
  was served, Dine-in green, Takeaway terracotta, Delivery mustard, against a
  round axis.
- **Sales by category**, a ring: the total in its middle, and each category
  with its share. Past six, the rest are "Others".
- **Top selling items**, with **View all** to the Items tab: #, Item, Qty,
  Sales.
- **Order type breakdown**, a ring: the orders in its middle, and "Dine-in
  52% (223)".
- **Sales by payment method**: a bar for each method with its share and what
  it took.
- **The rows' widths:** Sales trend beside Sales by category from 1280px,
  the ring's card up to 464px, so its legend stays beside the ring; under
  1280px one over the other. Top selling items, Order type breakdown and
  Sales by payment method are three across from 1280px, three sevenths and
  two and two (about 448px, 299px and 299px at 1366x768), and from 1536px
  the two take the back office's side width (`w-aside`, 369 to 380px; the
  design draws 420px) with Top selling the rest (TECH-17). From 1024px they
  are two across, Top selling over both.

**In detail**, under a small-caps heading, the reports the design does not
draw. Each card has its own **Download CSV**:

- **The range:** the one total, **Sales**, with "12 bills · average 285.00"
  under it; beside it Gross, Discounts, Net sales, Tax, Credit notes (with
  their count), Sales after credit notes, Refunds (with their count), Tips.
- **Sales over time:** a row per period: Period ("1 Oct", or "28 Sept to 4
  Oct"), Bills, Net, Tax, Total. It scrolls inside its card past twelve
  rows.
- **How it was paid** and **Where it came from:** Method, Tenders, Net and
  its share; Channel, Bills, Total and its share.
- **When it sold**, the heatmap: a row per weekday, Monday first; a column
  per hour from the first with a bill to the last, in the order the trading
  day runs them (the venue's day start first), so 01:00 sits after the evening it belongs
  to. A cell shows its count over a green tint in four steps; the count is
  the figure, the tint only repeats it. Under it, "Busiest: Friday 13:00, 3
  bills, 630.00".
- **Corrections:** every void, comp, discount, refund, payment void and
  reopen in the range, added up. "8 corrections worth 330.00", and "What
  each was worth to its bill, not money out of the drawer. A reopen counts
  and is worth nothing." Then **By kind**, **By reason**, **Who did them**
  and **Who let them through**. A kind and a person on the staff are links
  to Audit for the range and that kind or person. It reads on its own, so
  the sales above show while it adds up.

## Items and inventory (D16)

- **Item performance**, with **View all** that scrolls to "What sold": the
  eight items that took the most. Item, Qty, **Net sales**, **Cost**,
  **Margin**, **Trend**.
  - Cost and Margin are the server's (ADR-189): what the stock ledger moved
    for each dish, at the cost it had then. A margin is given only when the
    ledger costed every unit sold; otherwise "3 of 5 costed", "Part costed"
    or "Not costed".
  - Trend is the item's sales against the days before: an arrow and a
    percentage, "Level", or "New in these days".
- **Wastage:** up to five stock items as bars, each with what was written
  off at cost, or "Not costed"; "2 more items written off"; "Total wastage
  ₹2,800.00 over 4 write-offs."; and which reason cost the most.
- **Stock health** (`stock.manage`): three rows with an icon each: "48 items
  healthy · Above their low level", "6 items low · At or under their low
  level", "1 item out" and its name. A link to Inventory under them. From
  1024px it sits beside Wastage at the back office's side width
  (`w-aside`, 288px at 1024, 328px at 1366), Wastage the rest; under that,
  under it.

**In detail:**

- **What sold:** Item, Qty, Gross, Discounts, Net, Tax, Total, and its
  share. Sorted by Total, largest first; the Item, Qty and Total headers
  sort it. **By category:** the same under Category. Items are named as the
  menu stands now.
- **Closing stock** (ADR-125): the shifts in the range whose close counted
  stock. First the variance at cost in words and a tone: "Short by 23.60"
  (danger), "Over by 6.40" (warning), "Counted as expected" (success). Then
  **By item** (Item, Counts, Variance, At cost) and **By shift** (Day, Item,
  Expected, Counted, Variance, Cost, At cost).

## Staff (D17)

- **Staff performance · 29 Sep – 5 Oct 2026**, with "Tips are those on the
  payments each person took": #, Staff, Role, Orders, Sales, Average, Tips.
  By who opened the bill. A guest who ordered for themselves reads "Ordered
  for themselves"; someone gone, "No longer on record".
- **Removed in error:** each person with what the lines they rang up and
  took off were worth.
- **Tips:** the total, "Taken by 3 people, on their payments.", and "Tips
  are owed to staff and are never in sales. How a shift's tips were shared
  out is on its Z-report."

**In detail: Who sold it**, the full table: Opened by, Bills, Gross,
Discounts, Total, Average bill, and their share.

## Data

- `GET /api/v1/reports/sales?from=&to=&every=`, key `["sales", range,
  every]`, the last answer kept while the next is read. All three tabs read
  through one hook (`useRangeSales`), so moving between them asks nothing
  new. The same again for the days just before, cut by month, for the
  comparisons.
- `GET /api/v1/reports/items?from=&to=`: each item's net sales, cost and
  margin.
- `GET /api/v1/reports/wastage?from=&to=`: write-offs by stock item and by
  reason.
- `GET /api/v1/stock`, for stock health.
- `GET /api/v1/reports/closing-stock?from=&to=`.
- `GET /api/v1/audit/analysis?from=&to=`, for Corrections.
- Downloads: `GET /api/v1/reports/sales/csv?from=&to=&every=&report=`
  (`summary`, `over_time`, `payments`, `channels`, `taxes`, `hours`,
  `items`, `categories`, `staff`), `.../closing-stock/csv`, and
  `/api/v1/audit/analysis/csv?by=`. The answer's `text` is saved as its
  `filename`.
- A link to the trail is `/audit?from=&to=&kind=` or `&staff=`.

## States

- **No days picked:** "Pick a first and a last day."
- **Loading:** "Adding up the range". While the last range's figures are
  shown dimmed, Export and Download CSV are off: a file must match its
  screen.
- **Offline or refused:** "Can't reach the mez server. Check this computer's
  connection.", or the server's words for a refused range (over a year, To
  before From), with **Try again**.
- **Nothing in the range:** "No sales in these days.", "Nothing sold in
  these days.", "No bills in these days.", "Nothing was paid in these
  days.", "Nothing was written off in these days.", "Nobody opened a bill or
  took a tip in these days.", each in its own card.
- **The days before could not be read:** the comparisons are left off, or
  read "Not compared".
- **Corrections or closing stock cannot be read:** the words in that
  section, with Try again; the rest of the screen is unaffected.
- **A download that fails** says so in a toast; the screen stays.
- **Without `stock.manage`:** no Stock health card.

## Constraints

- No primary on these screens: nothing on them moves money.
- Every number tabular, every amount right-aligned and through `Money`.
- The screens add nothing up. Every figure is the server's; the only
  arithmetic here is a bar's height, a share, a cell's tint, a change
  against the days before, and an average bill.
- Sorting a table reorders the server's rows; it changes no figure.
- No chart library: a bar is an element with a height, a ring a conic
  gradient. Each picture says its figures in words for a screen reader.
- The chart colours are green, terracotta, mustard, in that order, then the
  quieter inks (tokens.md).
- One download at a time.
- The range is not in the address: a reload returns to seven days.

## Reference

`mez Desktop Management.dc.html`, D15 Sales Reports (line 36), D16 Item
Inventory Reports (line 176) and D17 Staff Customer Reports (line 224). Left
out, and why:

- **All Outlets and All Order Types:** a venue is one outlet, and the server
  does not filter a report by how it was served.
- **Wallet** among the payment methods: there is none.
- **Generate table QR codes:** phase 11.
- **The Customers tab, new and returning guests, top regulars, feedback
  themes and the rating spread:** customers are out of v1, and nothing rates
  anyone before the guest channel.
- **The Rating column** on staff, and "Tips are paid out weekly": how tips
  are shared is the venue's rule, on the Z-report.
- **Raise purchase order:** purchasing is out of v1.
- **"2.2% of food cost" and the advice on ordering:** nothing works them
  out. The card says the total and which reason cost the most.
- **A reports sub-navigation in the sidebar:** the tabs are in the head.

Changed: the design's four tabs are Sales, Item reports, Staff and
Customers; Items and inventory are one tab, as D16 draws them as one screen.
Wastage is by stock item, not by a food category, since a write-off names an
item. "Revenue" is **Net sales**.

Kept, though not drawn (rule 4): everything under "In detail" on each tab,
and each file's Download CSV.

# Back office: Dashboard

Today at a glance: what has sold, which tables are in use, who sold it, and
what is running low. Built from D12 in the rebuild's Task 16b (ADR-183).
`web/backoffice/src/dashboard/`: `Dashboard.tsx`, `StatTile.tsx`, `Panel.tsx`,
`SalesOverview.tsx`, `DashLists.tsx`, `QuickActions.tsx`,
`dashboard-logic.ts`, `use-dashboard.ts`. At `/backoffice/`, keys `g d`.

## Purpose

Open the laptop in the office and know how the day is going without opening a
report.

## Layout

Inside the frame (`backoffice-shell.md`). Three rows of cards. Each card is
for whoever may read what is in it, and a card nobody could fill is not
drawn.

- **The backup band**, over the figures, for whoever holds `settings.edit`,
  only while nothing is being backed up or the backup is failing (BO-11,
  ADR-381 item 7, ADR-399): a warning icon and the backup's own sentence,
  "Nothing is being backed up. If this computer's disk fails, the venue's
  sales, menu and settings go with it." in the warning tone, or "The backup
  is not reaching its destination. ..." in the danger tone; and **Set up
  the backup** or **See why**, to Settings, Backup. It is read every minute
  (`GET /api/v1/settings/backup`, the key Settings reads too).

- **Four figures**, four across from 1280px, two from 640px. Each is an icon
  in a tinted round, what it is, the figure in the serif, and a line under
  it:
  - **Today's sales** (`reports.view`), on the green tint with a leaf
    behind: the total of the bills invoiced today, and "12% up on
    yesterday" with an arrow.
  - **Total orders** (`reports.view`): the paid bills, against yesterday.
  - **Active tables** (`order.open`): "12 / 18", and "67% occupied".
  - **Average order value** (`reports.view`): the total over the bills,
    against yesterday; "No bills yet" before the first.
- **Second row**, three across from 1280px (a wide card, then two lists),
  the wide card three sevenths of the row and each list two (at 1366x768
  about 448px, 299px and 299px). From 1536px the lists take the back
  office's side width (`w-aside`, 369 to 380px; the design draws 400px) and
  the wide card the rest (TECH-17). From 1024px it is two across, the wide
  card over both; under that, one.
  - **Sales overview**, with **View reports**: a bar for each hour of the
    trading day against a round axis, every other hour named under it. One
    green.
  - **Top selling items**, with **View all**: the five that took the most,
    each with its rank, name, "14 sold" and what it took.
  - **Active tables**: up to five tables not free, each with its name, its
    guests, its state as a chip and how long; "3 more tables in use" under
    them.
- **Third row**, the same widths:
  - **Staff performance**: a table, #, Staff (initials and name), Role,
    Orders, Sales, for the five who opened the most in bills today.
  - **Low stock alerts** (`stock.manage`), with **View all**: up to five
    items at or under their level, each with what is left in the warning or
    danger ink and a **Low** or **Out** chip; "2 more items are low". With
    no ingredient tracked it says "No ingredients tracked." and **Add what
    you keep, in Inventory.**, never "Nothing is low." (BO-10).
  - **Quick actions**, two by two: **Edit the menu**, **Inventory**, **View
    reports**, **Manage staff**, each shown to whoever may open that
    section. Under it the brand's card, a leaf and a script line.

## Data

Each read is for whoever may make it, and is asked again every minute: the
dashboard is left open, and nothing here is worth a stream of its own.

- `GET /api/v1/reports/sales?from=&to=&every=day` for today, and again for
  yesterday (`reports.view`). Keyed by the day, so when the trading day
  turns the old figures go. The report carries each person's role beside
  their name.
- `GET /api/v1/floor/live` (`order.open`).
- The stock items, `GET /api/v1/stock` (`stock.manage`).

## Interactions

| Action | Does |
|---|---|
| View reports, View all (items) | Opens Sales reports |
| View all (stock) | Opens Inventory |
| A quick action | Opens that section |

Nothing here changes anything.

## States

- **Reading:** "Reading" in a figure's place; "Reading today's sales" in a
  card.
- **Not read:** "Not read" in a figure's place; the card says so with **Try
  again**.
- **Nothing yet:** "No bill has been paid yet today." under the bars;
  "Nothing has sold yet today."; "Every table is free."; "Nobody has a paid
  bill yet today."; "Nothing is low." where ingredients are tracked and none
  is low, "No ingredients tracked." where none is; "No tables on the floor
  plan".
- **A person gone from the staff:** their role reads "No longer on record".
- **A role that opens nothing:** one card, "Your role opens no back office
  sections. Ask the owner if you need one."
- **Offline:** the last figures stay; the bar says the server is
  unreachable.

## Constraints

- No primary: nothing here moves money.
- Every figure is the server's; the only sums here are a change against
  yesterday, an average, a share and a bar's height.
- A change is an arrow, a percentage and words ("up on yesterday"), never a
  colour alone.
- Today so far is compared with the whole of yesterday, so a morning reads
  as down. A table to clear counts as active.
- Holds together down to 1366 wide.

## On a mirror

The shortcuts "Edit the menu" and "Manage staff" say "Menu" and "Staff"
(ADR-330): the sections open, and nothing in them is changed.

## Reference

`mez Desktop KDS and Dashboard.dc.html`, D12 Dashboard (line 280). Left out,
and why:

- **The bars stacked by Dine-in, Takeaway and Delivery, and their legend:**
  the report's hours are not cut by how it was served. A bar is one green.
- **"View floor" on Active tables:** the floor is the terminal's.
- **The coffee spot art.**

Changed: "vs. yesterday" reads "up on yesterday" or "down on yesterday", so
the direction is in the words. "Add menu item" is **Edit the menu**, since
the link opens the menu, and **Inventory** is a fourth shortcut.

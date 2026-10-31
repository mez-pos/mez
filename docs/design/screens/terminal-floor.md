# Terminal: Tables

The dining room at a glance: every table in service, where it is in its turn,
how many sit at it, how long, and what they owe; and beside it the table
picked, with its order. Built in phase 08 (ADR-087, ADR-088) and rebuilt from
D05 in the rebuild's Task 7 (ADR-155, ADR-158). `web/terminal/src/floor/`:
`TablesScreen.tsx`, `TablesSidebar.tsx`, `FloorPlan.tsx`, `TableList.tsx`,
`TablePanel.tsx`, `TableOrderView.tsx`, `TableActions.tsx`, `floor-logic.ts`,
`tones.ts`. At `/tables`; `/tables?table=7` opens with that table picked.

Laying the floor out is the back office's (`backoffice-floor.md`).

## Purpose

Seat the next party at the right table in two taps, and see which tables need
something now: an order to send, a bill to take, a table to clear.

## Layout

Three columns inside the frame (`terminal-shell.md`) from 1280px wide (xl).
Below that, two: the panel is not beside the floor but over its right edge,
and only once a table is picked (ADR-381, TBS-4).

- **At 1366x768:** sidebar, floor and panel side by side, the panel's empty
  state while nothing is picked. A floor of up to seven columns fits beside
  the panel without scrolling.
- **At 1024x768:** sidebar and floor only, the floor across the rest of the
  width, so a floor of up to seven columns fits and every table is in view.
  A tap on a tile lays the panel over the floor's right edge, with a shadow
  and a round close button (X, 48px, "Show the whole floor" to a screen
  reader) at the right of its head; the tiles
  left of it can still be tapped, and the close button gives the floor back.
  While Move, Join or Part is on, the panel is not drawn, so every tile can
  be tapped; Done brings it back. Under 820px tall (`short:`) the section's
  padding and gaps and the panel's head and foot step down.

- **Sidebar, 240px, chrome:** **Floor Plan** and **Table List**, two ways to
  look at the same tables, the chosen one the terracotta fill; **Order
  History**, which opens the day's orders; a script line at the foot.
- **The floor, the rest:**
  - **Areas** as 48px pills when the venue has more than one, the chosen one
    terracotta, with **All Areas** last. At the right, "9 tables · 3 free · 4
    seated · 1 paying · 1 to clear".
  - **The banner**, while moving or joining: what to tap next, and **Done**.
  - The error line.
  - **Floor Plan:** the area as the back office drew it, on the sunken ground
    in a card. Empty rows and columns are taken out, so the tiles share the
    room. A tile is at least 92px wide and as tall as its lines, and grows to
    fill; a floor too big for that scrolls. Each tile: the table's shape faint in its state's
    colour (round if it is round), its name in the serif, who is at it ("4
    seats", "3 guests", "Seated", "With T4", "Needs clearing"), its state as
    a chip (or, while a guest's phone has asked to order there, the
    terracotta "Request" mark of `terminal-requests.md`), how long ("42m"),
    and under it what is owed ("₹930.00"): side by side they ran past a
    narrow tile's edge. The border is
    the state's tone; the picked tile has the terracotta outline. Under
    "All Areas" each area has its name over its grid.
  - **The legend** under the plan: a dot and a word for each of the five
    states.
  - **Table List:** a card, **Table availability**, with a count and a bar
    for each state; then a table of rows: Table, Area, Guests, Status, Time,
    Balance from 1280px wide. Below that (TBS-5): Table, Guests with the time
    under it ("Guests · Time"), Status, Balance; Area is left out, and
    Balance never leaves. Area by area, and within one by name the way people count ("T2"
    before "T10"). The picked row has the green tint.
- **The panel, 400px, chrome:** the table picked. With none, from 1280px:
  "Pick a table to see its order."; below that it is not drawn.
  - **The head:** "Table T4", or "Table T4 + T7" for a joined party, in the
    serif; the state chip; how long as a pill, terracotta once late; and the
    guests, or the seats of a table with nobody at it.
  - **A free table:** "This table is free.", **Guests** on a stepper (it
    starts at the table's seats, 12 at most; 1 to 99), **Seat 4 guests** (the
    primary, 56px) and **Seat and order**.
  - **A table to clear:** "The guests have paid and gone. Clear it once it is
    laid again, and it is free." and **Clear table**.
  - **A table with one order:** two tabs, **Current Order** and **Notes**
    (with a count). Current Order is the lines, each with its quantity, name,
    choices, a Ready, Void or Comped chip and what it comes to, then the totals as
    on the order screen, tax by tax. Notes lists each note under what it is
    about, an item or "The order". A saved order says "Saved. Add Items
    brings it back."
  - **The foot of an order:** **Add Items** and **Send to Kitchen**; then
    **More Actions** and **Settle Bill** with what is owed (the primary). A
    paid order still open reads **Invoice and close** or **Close order**.
    Under them, why Settle Bill is shut when it is.
  - **A table with several orders:** "2 orders are open at this table. Pick
    one.", each with its id, item count, total and age; once one is picked,
    "All 2 orders at this table" leads back.

**More Actions** opens upward: **Move to another table**, **Join tables**,
**Part tables** for a joined party, **Fire course N** when one is held back,
**Order history**, and **Guests left** for a table seated with nothing
ordered.

## States and their words

| State | Tone | Word |
|---|---|---|
| free | success (green) | Free |
| seated | info (blue) | Seated |
| ordered | danger (the occupied rose, no alarm) | Ordered |
| settling | warning (mustard) | Paying |
| settling, nothing owed | success (green) | Paid |
| dirty | neutral (the cleaning grey) | To clear |

Five states, five tones (tokens.md), always the colour and the word. A
settling table with nothing owed, paid in full and kept open for the
party's next round (ADR-372), says **Paid**, not Paying: a word, not a
sixth state of the turn (ADR-383). The legend and the counts line name it.
Terracotta is for what is picked and what is late: a table's state is never
terracotta.

## Move, Join and Part (ADR-088)

Started from More Actions on the table in the panel, so the party's table is
already chosen. A banner says what to tap next; taps on tiles then do the
mode's work until **Done**.

- **Move:** "Tap a free table for T4's party". One tap moves the order; the
  mode ends and the panel follows the party.
- **Join:** "Tap free tables to join to T4. Tap a joined one to let it go."
  One tap each, until Done.
- **Part:** "Part: tap a joined table to let it go".

A joined table shows the party's state and "With T4"; its guests and balance
show only at the party's own table, and tapping it picks that table.
Refusals are a line of words: "T5 has nobody at it. Tap a table with
guests.", "T7 is joined to T4. Tap that one.", and a table with two orders
cannot be moved from here.

## Data

- `GET /api/v1/floor/live` (`order.open`), key `["floor", "live"]`: `grid`,
  `areas[]`, `tables[]` (id, area_id, name, seats, shape, x, y, status,
  covers, since, balance, orders[], joined_to). Read again on the stream's
  `floor_changed`, on any order event, and on a resync. Held above the
  routes, so the order screen names tables from the same read.
- `GET /api/v1/orders/{id}` for the order in the panel, read afresh each
  time its table is picked.
- `GET /api/v1/orders`: the open orders, to describe a table with several,
  and `late_after_minutes`.
- Seat: `POST /api/v1/orders` `{table_id, covers}`, through the order queue,
  so the new order is the one in front.
- `POST /api/v1/floor/tables/{id}/clear`, and `.../release` (Guests left).
  Both answer with the live floor.
- Move: `POST /api/v1/orders/{id}/table` `{from, to}`. Join and Part:
  `POST /api/v1/orders/{id}/tables/join` and `.../part` `{tables}`.
- Send and Fire: the order's `.../send` and its course's fire.
- The clock ticks every 30 seconds for the ages.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Tap a tile or a row | Picks the table, or the party's own table if it is joined | | The panel shows it; below 1280px it lays itself over the floor |
| The panel's X, below 1280px | Lets go of the table | | The panel goes; the whole floor is in view |
| Seat 4 guests | Opens an order at the table | `order.opened` | Stays on Tables; the button waits until the tile reads Seated, so a second tap cannot open a second order |
| Seat and order | The same, then the order screen | `order.opened` | The order screen, on the new order |
| Add Items | Brings the order to the front, off hold if it was saved | `order.resumed` if saved | The order screen |
| Send to Kitchen | Sends what waits | `ticket.sent`, `course.fired` | "Sent". Shut while this device still has items on their way to that order |
| Settle Bill | Opens `/pay/:id` | | `terminal-tender.md` |
| Guests left | Closes the empty order and frees the table | `order.closed` | The tile turns Free |
| Clear table | | | The tile turns Free |
| Move, then a free table | | `order.table_changed` | The panel follows |
| Join, then free tables | | `order.tables_joined` | "Table T4 + T7" |
| A joined table, in Join or Part | Lets it go | `order.tables_parted` | |
| Order history | Opens the order's history sheet | | |
| Today's orders (sidebar) | Opens `/orders` | | |

The lines are read here and changed on the order screen, where Add Items
goes.

## States

- **Loading:** "Loading the floor".
- **No tables:** "No tables yet. Lay out the floor in the back office."
- **Error:** the server's words where the floor goes, with **Try again**. A
  floor already read stays on screen while the server is away.
- **An area with no tables:** "No tables in this area."
- **The order will not open:** "Opening the order", then why, with Try
  again.
- **Refused:** the server's words in the panel, over its buttons
  ("Something is on this table's order: void it or take the payment
  first.").
- **Settle Bill is shut:** the same words as Pay on the order screen.
- **Permission denied:** the floor needs `order.open`, which everyone who
  reaches this screen holds.

## Constraints

- Colour and a word on every tile and row, never colour alone.
- Every button is 48px; Seat and Settle Bill are 56px; numbers are tabular.
- No drag and no long press: every action is a tap.
- At 1024 and at 1366 wide a floor of up to seven columns fits with no
  table off screen; a bigger one scrolls sideways (ADR-381). Nothing on a
  tile runs past its edge.
- A move or a join works on one party at a time, and the server decides: the
  mode only chooses what to ask.

## Reference

`mez Desktop POS.dc.html`, D05 Floor Plan (line 363), and for Table List the
"Table availability" card of D06 (line 478). Left out, and why:

- **Reservations, Waitlist, Customers** in the sidebar, and a Reserved
  state: out of v1's scope. **Settings** there: it is the back office's.
- **Edit Layout, Add Table and the zoom:** the floor is drawn in the back
  office, and the plan fits the room it is given.
- **The walls, ENTRANCE and BAR:** the back office's grid has tables only.
- **The waiter's name** in the panel: no table is assigned to anyone.
- **The Guests tab:** an order keeps a count of guests, not who they are.
  Notes is drawn because notes are kept.

Changed: the design's states are Free, Occupied, Needs Bill, Reserved and
Cleaning; a table here goes Free, Seated, Ordered, Paying, To clear, the
five the server knows. The counts sit beside the areas, not in the status
bar.

Kept, though not drawn (rule 4): Seat and Seat and order on a free table
(ADR-158); Clear table; Move, Join, Part, Fire course, Order history and
Guests left under More Actions; the choice between several orders at one
table.

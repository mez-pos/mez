# Waiter: Home

Where a waiter lands: the tables as cards, sorted by what each wants. Built
from T01 in the rebuild's Task 11 (ADR-163, ADR-164) with seating and clearing
added (ADR-171). `web/terminal/src/waiter/WaiterHome.tsx`, `TableCard.tsx`,
`SeatTable.tsx`, `waiter-logic.ts`, `table-logic.ts`. At `/waiter`, inside the
waiter's frame (`terminal-shell.md`).

## Purpose

See at a glance which table needs something, and be at it in one tap.

## Layout

Drawn for a tablet at 1366x1024. The cards scroll; the bar along the bottom
does not. A faint leaf sits behind.

- **The greeting:** "Good evening, Kabir!" in the serif, by the hour in the
  venue's zone (morning from 5, afternoon from 12, evening from 5 PM; before 5
  AM is still last night). Under it, "Here are the tables for tonight." At
  the right, a script line from 1280px and a green card ("Same Tables. New
  Stories.") from 1024px, on a screen taller than 820px.
- **The pills**, 52px, each with its count: **All tables**, then
  **Seated**, **Ordered**, **Paying**, **To clear**, **Free**: the till's own
  words, so the tablet and the till name a table one way (TBS-11, ADR-381,
  ADR-394), each with a dot in its tone. The pressed one is green. At the right, the search, "Search
  table number".
- **The cards**, four across from 1280px, three from 1024px, two under it.
  Each is one button, 120px or taller, washed in its group's tone: a chair in
  an 80px block, the table's name in the serif, the group's word with its
  dot, "Wants to order" in terracotta while a guest's phone has asked
  (`terminal-requests.md`), and a line of small print with what is owed after it: "No guests · 4
  seats", "3 guests · No order yet", "3 guests · ₹930.00", "With T4". A
  chevron at the right.
- **The bar, 96px, chrome:** **Active orders** with the count of open orders
  at the venue; a script line; and **New order** (the primary, 64px).

**On a screen 820px tall or less** (`short:`, the 1024x768 tablet and the
1366x768 laptop), the home is a working screen and its decoration gives way
(tokens.md, TBS-12): the green card is not drawn, and the search takes its
place at the right of the greeting, so it costs no row of its own; the pills
are 48px; the paddings step down; the bar is 72px, without its script line,
New order 56px. The tables start under the pills at about y=235 instead of
350.

- **At 1024x768:** the greeting with the search beside it, the five pills in
  one row, then the cards three across: three full rows, nine tables, in
  view.
- **At 1366x768:** the greeting, the script line and the search in one row,
  the pills, then the cards four across: all twelve of the demo venue's
  tables in view.

| Group | Table states | Tone | Word |
|---|---|---|---|
| seated | seated | info | Seated |
| ordered | ordered; settling with nothing owed, whose card says **Paid** in the success tone (ADR-383) | danger | Ordered |
| paying | settling, something owed | warning | Paying |
| clear | dirty | neutral | To clear |
| free | free | success | Free |

## Data

- The live floor, `GET /api/v1/floor/live`, held above the routes and read
  again on the stream's floor and order events.
- `GET /api/v1/orders`: the open orders, for the count.
- `GET /api/v1/venue`: the zone, for the greeting.
- Seat: `POST /api/v1/orders` `{table_id, covers}`. Clear:
  `POST /api/v1/floor/tables/{id}/clear`.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| A pill | Shows that group | | |
| Search | Finds tables by name; "t 10" finds "T10" | | |
| A card with one order | Opens the table's screen | | `waiter-table.md` |
| A free table's card | Opens **Seat Table T4**: "This table is free. It seats 4.", Guests on a stepper, **Seat and order** (the primary) and **Seat 4 guests** | `order.opened` | Seat and order goes to the order screen; Seat stays here |
| A card to clear | Asks "Clear Table T4?" | | Confirmed, the card turns Free |
| A joined table's card | Stands for the party's own table | | |
| A table with several orders | Opens Tables with it picked | | Its panel asks which |
| Active orders | Opens the day's orders | | `terminal-past-bills.md` |
| New order | Starts a fresh order and opens the order screen | | Its bar asks for a table and how it is served |

## States

- **Loading:** "Loading the tables".
- **No tables:** "No tables yet. Lay out the floor in the back office."
- **Nothing in a group:** "No tables here right now." A search with no match:
  "No table matches "9"."
- **Error:** the server's words, with **Try again**.
- **A clear refused:** a toast, "Couldn't clear T4: ...".
- **Offline:** the cards stay as last read; the top bar says "Server
  unreachable" with Retry now.

## Constraints

- Colour and a word on every card and pill, never colour alone.
- Every target is 48px or more; the cards are far past it, for a hand that is
  not looking.
- One primary: New order.
- Every table at the venue is listed, not this waiter's own: no table is
  assigned to anyone. So the line reads "the tables", not "your tables".

## Reference

`mez Tablet Waiter.dc.html`, T01 Waiter Home (line 36). Left out, and why:

- **Table Requests, QR Orders and the bell:** nothing rings them before the
  guest channel (phase 11).
- **Reserved:** not a state a table has (D06 is out of scope). The fifth
  pill is **To Clear**, which is one.
- **"your tables":** see above.

Kept, though not drawn (rule 4): seating a free table and clearing one from
here (ADR-171), so the commonest two things need no trip to the floor.

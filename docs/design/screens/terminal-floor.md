# Terminal: Floor

The dining room at a glance: every table in service, where it is in its
turn, how many sit at it, how long, and what they owe. Where a party is
seated, where a table's ticket is found, and where a table is cleared.
Built in phase 08 Task 2b (ADR-087); Move, Join and Part in Task 2c
(ADR-088).

## Purpose

Seat the next party at the right table in one tap, and see which tables
need something now: a bill to take, a table to clear.

## Layout

Paper surface, inside the register's `SignedInShell`; it replaces the
register's body as the bill and tender screens do, so the tickets on this
till are kept while it shows.

- **Header row:** "Floor" in the serif; **Register** on the right
  (back to the ticket in front). A count per state in words: "3 free · 4
  seated · 1 paying · 2 to clear".
- **Areas**, a row of 48px tabs when the venue has more than one.
- **The grid**: the area as the back office laid it out
  (`backoffice-floor.md`), `grid.columns` by `grid.rows`, empty cells
  blank and empty rows and columns at the edges dropped. Each **tile** is
  88 to 144px square (a round table drawn round), and shows:
  - the table's name (xl, semibold) and its shape's outline
  - its state as a `StatusChip`: **colour and the word**, always
  - occupied: covers ("3 covers", tabular number), the time since seated
    ("42m", tabular), and the balance (`Money`, tabular, right-aligned)
  - dirty: the time since the bill closed ("6m")
  - free: its seats ("4 seats")
  The tile's border takes the state's tone; its fill stays the surface, so
  the word stays readable.
- **Seat sheet** (`Sheet`): "Seat table 4", covers as a `Qty` (1 to 99,
  starting at the table's seats, capped at 12), **Seat** (primary).
- **Table sheet** (`Sheet`), for a table that needs a choice:
  - seated with nothing ordered: **Open ticket** (primary), **Guests left**
  - dirty: **Clear table** (primary)
  - two tickets at one table: each ticket, tap to open

## Move, Join and Part (Task 2c)

Three header buttons, 48px, beside Register. Each turns on a mode with a
banner saying what to tap next and **Done** (primary) to end it; taps on
tiles then do the mode's work instead of their usual one.

- **Move**: tap the party's table, then a free one. The ticket moves;
  the mode ends. Tapping the party's table again picks another.
- **Join**: tap the party's table, then free tables, one tap each, until
  Done. Tapping a table already joined lets it go.
- **Part**: tap joined tables to let them go.

A **joined table** shows the party's state and "With T4"; its covers and
balance are shown only at the party's own table. The ticket reads "Table
T4 + T7 · 6 covers". Refusals are said under the banner in words: "T5 isn't
free. Tap a free table.", "T7 is joined to T4. Tap that one.", and a table
with two tickets cannot be moved from the floor.

## States and their words

| State | Tone | Word |
|---|---|---|
| free | success (green) | Free |
| seated | info (blue) | Seated |
| ordered | danger (the occupied rose, no alarm) | Ordered |
| settling | warning (mustard) | Paying |
| dirty | neutral (the cleaning grey) | To clear |

Five states, five tones (tokens.md). Terracotta stays for "Late", on
tickets: a table is never terracotta.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a free table | Seat sheet | |
| Seat | Opens an order at the table with the covers, now, not on the first item | The register, on the new ticket, labelled "Table 4 · 3 covers" |
| Tap an ordered or paying table (one ticket) | Brings its ticket to the front (resumes it if held) | The register |
| Tap a seated table | Table sheet | Open ticket, or Guests left |
| Guests left | Closes the empty ticket and clears the table in one step | The tile turns Free |
| Tap a dirty table, Clear table | Clears it | The tile turns Free |
| Register | Back to the register | |
| Tickets sheet, **Seat a table** | Opens the floor | Replaces typing a table number |

## Data

- `GET /api/v1/floor/live` (`order.open`): `grid`, `areas[]`, `tables[]`
  (id, area_id, name, seats, shape, x, y, status, covers, since?,
  balance?, orders[]). React Query key `["floor", "live"]`, refetched on
  the stream's `floor_changed`, on any order event, and on a resync.
- Seat: `POST /api/v1/orders` `{ channel: "counter", table_id, covers }`,
  through the ticket queue, so it is the ticket in front.
- `POST /api/v1/floor/tables/{id}/clear`, `.../release` (Guests left).
  Both answer with the live floor.
- Move: `POST /api/v1/orders/{id}/table` `{ from, to }`; Join and Part:
  `POST /api/v1/orders/{id}/tables/join` and `.../part` `{ tables }`.
  A joined table carries `joined_to` in the live floor.
- Ticket labels use the table's name from this read ("Table T4"), not its
  id.

## States

- **No tables:** "No tables yet. Lay out the floor in the back office."
  with Register.
- **Loading / offline:** the words in place of the grid; the grid last
  read stays, marked by the shell's connection chip.
- **Refused:** the server's words in the sheet ("Something is on this
  table's ticket: void it or take the payment first.").

## Constraints

- Colour and a word on every tile, never colour alone.
- Tiles at least 88px, every button 48px; numbers tabular.
- No drag, no long press: every action is a tap and a sheet.

# Terminal: Floor

The dining room at a glance: every table in service, where it is in its
turn, how many sit at it, how long, and what they owe. Where a party is
seated, where a table's ticket is found, and where a table is cleared.
Built in phase 08 Task 2b (ADR-087). Transfer, join and split are Task 2c.

## Purpose

Seat the next party at the right table in one tap, and see which tables
need something now: a bill to take, a table to clear.

## Layout

Ink surface, inside the register's `SignedInShell`; it replaces the
register's body as the bill and tender screens do, so the tickets on this
till are kept while it shows.

- **Header row:** "Floor" in Instrument Serif; **Register** on the right
  (back to the ticket in front). A count per state in words: "3 free · 4
  seated · 1 paying · 2 to clear".
- **Areas**, a row of 44px tabs when the venue has more than one.
- **The grid**: the area as the back office laid it out
  (`backoffice-floor.md`), `grid.columns` by `grid.rows`, empty cells
  blank and empty rows and columns at the edges dropped. Each **tile** is
  88 to 144px square (a round table drawn round), and shows:
  - the table's name (xl, semibold) and its shape's outline
  - its state as a `StatusChip`: **colour and the word**, always
  - occupied: covers ("3 covers", mono number), the time since seated
    ("42m", mono), and the balance (`Money`, mono, right-aligned)
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

## States and their words

| State | Tone | Word |
|---|---|---|
| free | success (sage) | Free |
| seated | info (slate) | Seated |
| ordered | neutral | Ordered |
| settling | warning (gold) | Paying |
| dirty | danger | To clear |

Vermilion stays for "Late", on tickets: a table is never vermilion.

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
- Tiles at least 88px, every button 44px; numbers mono.
- No drag, no long press: every action is a tap and a sheet.

# Back office: Floor plan

Where an owner lays out the venue: its areas (Inside, Terrace) and where
each table stands, how many it seats and how it is drawn. Built in phase 08
Task 1 (ADR-085). The terminal's floor screen (Task 2) draws the same plan
with each table's state on it; this screen only places them.

## Purpose

Get the room onto the screen once, before service, so the floor screen
looks like the floor.

## Layout

Paper surface, inside `SignedInShell`. The back office home links here
("Floor plan") beside Printers.

- **Header row:** the title "Floor plan" in the serif; **Add an
  area** (secondary) and **Add a table** (primary) on the right. No area
  yet: only Add an area.
- **Areas**, a row of 48px tabs, in the order added. A retired area's tab
  is struck. Under the tabs, the chosen area's **Rename** and **Retire**
  (or **Bring back**).
- **The grid**: the chosen area as `grid.columns` by `grid.rows` cells
  (12 by 8), every cell at least 56px, the whole grid scrolling sideways
  on a narrow screen rather than shrinking under that. A free cell is a
  dashed outline; a table fills its cell:
  - its name (lg, semibold) and its seats ("4 seats", sm, tabular number)
  - its shape: square, round (a circle) or long (a wide bar)
- **The chosen table** (a `Card` under the grid): name, seats, shape, and
  where: "Inside, column 3, row 2". **Edit**, **Move**, **Retire**.
- **Retired tables** in this area, a list under that: name and **Bring
  back**.
- **Add / Edit table** (`Modal`): name; seats; shape (square, round,
  long); area (a native select of active areas); **column** and **row** as
  numbers from 1. Errors under their field.
- **Add / Rename area** (`Modal`): name.

## Moving a table: three ways, drag never the only one

1. **Drag** a table onto a free cell (pointer events, so it works with a
   finger, a pen and a mouse). Dropped on a taken cell or off the grid, it
   goes back and says why.
2. **Move, then tap**: choose the table, **Move**, then tap a free cell.
   The grid says "Tap a free cell for T4" with **Cancel**.
3. **Edit**: type the column and row.

Every one is the same `PUT /api/v1/floor/tables/{id}`.

## Data

- `GET /api/v1/floor`: `grid` (columns, rows), `areas[]` (id, name,
  active), `tables[]` (id, area_id, name, seats, shape, x, y, active). `x`
  and `y` count from 0; the screen shows them from 1. React Query key
  `["floor"]`; every change answers with the whole plan, which replaces it.
- `POST /api/v1/floor/areas` `{ name }`, `PUT .../areas/{id}`,
  `POST .../areas/{id}/retire`, `.../restore`.
- `POST /api/v1/floor/tables` `{ area_id, name, seats, shape, x, y }`,
  `PUT .../tables/{id}` with the same, `.../retire`, `.../restore`.
- Every call needs `settings.edit`.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a free cell | Opens Add a table, at that cell, in this area | The table appears there |
| Tap a table | Chooses it | Its card shows under the grid |
| Drag a table to a free cell | Moves it | It stands there; "Moved T4" |
| Drag onto a taken cell | Nothing | "Table T2 is already at column 4, row 3." |
| Move, tap a cell | Moves it | As a drag |
| Retire a table | Takes it off the floor | It gives up its cell; listed under Retired tables |
| Bring back a table | Puts it back in its cell | Refused if another table stands there now: move it first |
| Retire an area | Hides it from the floor screen | Refused while it has tables in service |

## States

- **No areas:** "No areas yet. Add one for each room or terrace, then
  place its tables."
- **An area with no tables:** the empty grid, and "Tap a cell to place a
  table."
- **Refused:** the server's words, under the field in a modal, or under
  the grid for a drag or a tap-move.
- **Loading / offline:** the words in place of the grid.

## Constraints

- One primary: **Add a table** is the primary button. The chosen table is
  shown with the focus outline, not a fill.
- Every control 48px; every cell 56px. Seats, columns and rows in the numeral face.
- No hover-dependent behaviour: a drag is a shortcut, not the way.

## Reference

The archived back office design has no floor editor; this screen is new,
built from the primitives in `docs/design/components.md`.

# Back office: Floor plan

Where an owner lays out the venue: its areas (Inside, Terrace) and where each
table stands, how many it seats and how it is drawn. Built in phase 08
(ADR-085). The rebuild left its layout as it was (ADR-195): it is a grid
editor, not a list, so Task 16's table pattern does not fit it.
`web/backoffice/src/floor/`: `FloorScreen.tsx`, `FloorGrid.tsx`,
`FloorFormModals.tsx`. Section **Floor plan**, path `/floor`,
`settings.edit`, keys `g f`. Settings links here from its rail as "Tables and
floor plan".

The terminal's Tables screen draws the same plan with each table's state on
it (`terminal-floor.md`); this screen only places them.

## Purpose

Get the room onto the screen once, before service, so Tables looks like the
floor.

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Floor plan" in the serif; **Add an area** and **Add a
  table** (the primary) at the right. With no area yet, only Add an area.
- **Areas**, a row of 48px tabs, in the order added. A retired area's tab is
  struck. Under the tabs, the chosen area's **Rename** and **Retire** (or
  **Bring back**).
- **The grid:** the chosen area as 12 columns by 8 rows, every cell at least
  56px, the whole grid scrolling sideways on a narrow screen rather than
  shrinking under that. A free cell is a dashed outline; a table fills its
  cell with its name and its seats ("4 seats"), drawn as its shape: square,
  round or long.
- **The chosen table**, a card under the grid: "Table T4", its seats, shape,
  and where ("Inside, column 3, row 2"). **Edit**, **Move**, **Retire**.
- **Retired tables** in this area, a list under that: the name and **Bring
  back**.
- **Add, Edit table** (a modal): name; seats; shape; area; **column** and
  **row** as numbers from 1. Errors under their field.
- **Add, Rename area** (a modal): name.

## Moving a table: three ways, drag never the only one

1. **Drag** a table onto a free cell (pointer events, so it works with a
   finger, a pen and a mouse). Dropped on a taken cell or off the grid, it
   goes back and says why.
2. **Move, then tap:** choose the table, **Move**, then tap a free cell. The
   grid says "Tap a free cell for T4" with **Cancel**.
3. **Edit:** type the column and row.

Every one is the same `PUT /api/v1/floor/tables/{id}`.

## Data

- `GET /api/v1/floor`, key `["floor"]`: `grid` (columns, rows), `areas[]`
  (id, name, active), `tables[]` (id, area_id, name, seats, shape, x, y,
  active). `x` and `y` count from 0; the screen shows them from 1. Every
  change answers with the whole plan, which replaces it.
- `POST /api/v1/floor/areas` `{name}`, `PUT .../areas/{id}`, `POST
  .../areas/{id}/retire`, `.../restore`.
- `POST /api/v1/floor/tables` `{area_id, name, seats, shape, x, y}`, `PUT
  .../tables/{id}`, `.../retire`, `.../restore`.
- A change reaches every Tables screen through the stream's `floor_changed`.

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
| Retire an area | Hides it from Tables | Refused while it has tables in service |

## States

- **Loading:** "Loading the floor plan".
- **No areas:** "No areas yet. Add one for each room or terrace, then place
  its tables."
- **An area with no tables:** the empty grid, and "Tap a cell to place a
  table."
- **Error:** the server's words, with **Try again**.
- **Refused:** the server's words, under the field in a modal, or under the
  grid for a drag or a tap-move.

## Constraints

- One primary: **Add a table**. The chosen table is shown with the focus
  outline, not a fill.
- Every control 48px; every cell 56px. Seats, columns and rows tabular.
- No hover-dependent behaviour: a drag is a shortcut, not the way.
- The grid has tables only: no walls, doors or a bar, so Tables draws none.

## Reference

The design draws no floor editor. D05's "Edit Layout" and "Add Table" on the
terminal, and D22's "Tables & QR codes" in Settings, point at one; this
screen is where they lead, and it is ours, kept under rule 4. QR codes for
tables are phase 11's.

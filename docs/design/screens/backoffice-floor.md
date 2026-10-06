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
- **Ordering from the table**, a card under all of that, for the tables of
  every area (ADR-244):
  - "Ordering from the table" in the serif; "Each table has a code. A guest
    scans it to see the menu and order to that table. Anyone with a table's
    code can order to it."; and how things stand: "All 12 tables have a
    code.", "3 of 12 tables have no code."
  - **Make the missing codes**, secondary, only while a table has none.
  - **Print the codes**, secondary, where the venue has a `public_url` and
    a table has a code (ADR-245). A modal: "11 cards on 2 pages of A4.";
    "Each card has the table's name on it. Cut along the dashed lines and
    put each on its own table: a card on the wrong table sends that table's
    orders to another."; "These are the codes as they are now. Renew a code
    after printing and its card stops working."; **Close** and **Print**,
    the primary, which opens the browser's own print.
  - **The sheet**, on paper only: a card for each table with a code, two
    across and three down on A4, each inside a dashed line to cut along.
    On a card, centred, black on white: the venue's name in the serif;
    "Scan to see the menu and order"; the QR, 44mm; "Table T4", large; and
    the venue's host, small, so a guest can see whose code it is. A table
    with no code has no card. Nothing else of the back office prints.
  - With no `public_url`, in the warning tone: "The venue has not said
    where guests reach it, so there is no address to print on a table. Set
    public_url in the venue's configuration and restart mez."
  - A table of the tables: the table's name; its code in groups of four,
    "sptd em4x 7abc", or "No code: no orders by phone", muted; the address
    on its sticker, small; and a "..." with what can be done:
    - **Show the QR code** (a modal): the picture, 240px, black on white;
      the address under it; "Scan it with a phone to check it opens T4's
      menu."; **Close**. Not offered with no `public_url`.
    - **Renew the code**, behind a confirm in the danger tone: "Renew the
      code for T4? The code on the table stops working at once, and so does
      any photo of it. A guest ordering from that table now loses their
      place. Print the new code and put it on the table."
    - **Turn the code off**, behind a confirm: "Turn off the code for T4?
      Nobody can order from this table by phone until you give it a code
      again. The code on the table stops working at once, and a guest
      ordering from it now loses their place."
    - **Give it a code**, the only thing a table with none can do. It asks
      nothing: it stops nothing.

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

- `GET /api/v1/table-codes`, key `["table-codes"]`: every table in service
  with its code and its address, and `address`, where guests reach the
  venue. Read when the screen opens and never kept from an earlier visit:
  they are credentials. Read again when the floor changes.
- `POST /api/v1/table-codes` makes the missing ones;
  `POST /api/v1/tables/{id}/code/renew` and `.../revoke`. Each answers the
  whole list, which replaces the cache.
- `GET /api/v1/table-codes/sheet` when Print the codes is opened: every
  table that has a code, each with its address drawn, in one answer. Never
  kept. The venue's name on the cards is the status bar's (`use-venue.ts`).
- `GET /api/v1/tables/{id}/code/qr` when Show the QR code is opened, and
  never kept: the picture is an SVG in the answer, shown as an image and
  never put in the page as markup.

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

| Make the missing codes | Gives each table without a code one | "Every table has a code." The rest keep theirs |
| Renew the code, confirmed | A new code; the old one stops | "T4 has a new code. Print it and replace the one on the table." |
| Turn the code off, confirmed | No code | "T4 takes no orders by phone." |
| Give it a code | A code for a table with none | "T4 has a code." |
| Print the codes | Draws every table's card | The modal, with how many pages |
| Print | The browser's print | Only the sheet is on the paper |

## States

- **Loading:** "Loading the floor plan".
- **No areas:** "No areas yet. Add one for each room or terrace, then place
  its tables."
- **An area with no tables:** the empty grid, and "Tap a cell to place a
  table."
- **Error:** the server's words, with **Try again**.
- **Refused:** the server's words, under the field in a modal, or under the
  grid for a drag or a tap-move.
- **The codes could not be read:** in the card, why, with **Try again**. The
  plan above it is not affected.
- **A code could not be changed:** a toast: "That table is no longer on the
  floor. The list has been read again.", "Your role cannot change the
  tables' codes.", "Can't reach the server. Check the connection and try
  again.", or "That did not work. Try again."

## Constraints

- One primary: **Add a table**. The chosen table is shown with the focus
  outline, not a fill.
- Every control 48px; every cell 56px. Seats, columns and rows tabular.
- No hover-dependent behaviour: a drag is a shortcut, not the way.
- The grid has tables only: no walls, doors or a bar, so Tables draws none.
- **A code is a credential.** It is shown only to a role that may edit
  settings, is never kept from one visit to the next, and nothing that
  stops a working code happens without a confirm that says what stops.
- The address on a sticker is the venue's `public_url`, the server's to
  say. The screen builds none.
- The sheet is in millimetres and black on white: it is paper, and the
  screen's tokens are for screens. It is never shown on a screen.

## Reference

The design draws no floor editor. D05's "Edit Layout" and "Add Table" on the
terminal, and D22's "Tables & QR codes" in Settings, point at one; this
screen is where they lead, and it is ours, kept under rule 4. D22's "Tables
& QR codes" is the card here, since a code belongs to a table and this is
where the tables are.

# Terminal: Register

The till's main screen: find an item, ring it up, see the ticket and its
total. Built in phase 06: the menu side and the ticket rail in Task 2,
modifiers in Task 3, cart editing in Task 4, open tickets and hold in Task 5.
Payment (the Charge button) arrives in phase 07.

## Purpose

Ring up what the guest asked for in as few taps as the item allows, with the
ticket's total in view the whole time.

## Layout

Two columns inside `SignedInShell`, filling the screen below its header.
Nothing but the item grid and the line list scrolls.

- **Menu column** (flexible):
  - Top, fixed: the **category rail**, one 44px chip per category with its
    item count in mono, wrapping onto a second row rather than scrolling
    sideways. The chosen category is the raised-high surface with a strong
    border; the rest are raised. Right of the chips, the **search field**.
  - Below, scrolling: the **item grid**, `repeat(auto-fill, minmax(10.5rem,
    1fr))`, gap 10px. A tile is at least 7rem tall: the name (base, semibold,
    wraps), the station as metadata, and the price in mono at the bottom,
    right-aligned. An item with modifier groups says how many choices it has.
- **Ticket rail** (26rem, chrome surface, hairline on the left):
  - Top, fixed: the metadata trail (`TICKET · COUNTER`), then the order id
    in mono, as written: the trail uppercases, and `ord_` ids are case
    sensitive.
  - Middle, scrolling: the **lines**. Quantity in mono, the name, the chosen
    modifiers under it in muted text, the line's amount in mono on the right.
    A line the kitchen has not seen, whose item has choices, has
    **Modify**. Lines still on their way to the server are shown faint
    with "Adding".
    A line the server refused stays, in danger, with **Retry** and
    **Remove**.
  - Bottom, fixed, never scrolls: **Subtotal**, **Tax**, and **Total** in
    Instrument Serif at `--text-total` via `Money variant="total"`.
- **ModifierSheet** (`Sheet`: bottom on touch, side panel on desktop): the
  item's name, then each group with "Required · Choose 1" or "Optional · Up
  to 3" on the right, its choices two to a row, 44px, a price delta in mono
  where there is one. Fixed at the bottom: a reserved message line and the
  58px primary button, "Add to ticket" or "Save changes", carrying the
  price of one of the item with the choices made, live.

## Data

- `GET /api/v1/menu`, through `useMenu()`: cached for good, refetched only
  when the stream names another menu version (ADR-069). Read through
  `indexMenu()`.
- `POST /api/v1/orders` `{ channel: "counter", covers: 0 }` on the first item
  of a ticket, then `POST /api/v1/orders/{id}/lines` with the item's snapshot
  (`AddLine`: name, price, modifiers, station, course, tax rate, HSN). Each
  tap carries its own `Idempotency-Key`, reused by Retry, so a retry after a
  lost answer never rings an item twice.
- `GET /api/v1/orders/{id}`: the ticket, React Query key `["order", id]`. A
  command's answer replaces the cached order, so the rail redraws from the
  server's projection and never from local arithmetic.
- The current ticket's public id is kept in `sessionStorage`
  (`nofiltr.terminal.ticket`), so a reload comes back to it.
- `POST /api/v1/orders/{id}/lines/{line}/modifiers` `{ from, to }`: change
  a line's modifiers (`line.modified`, ADR-071), sent through the same
  ordered queue as taps, with its own `Idempotency-Key`.
- WebSocket: an `event` for the current order, or a `resync`, invalidates
  `["order", id]`, so a line added or changed on another till appears here.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Tap a category | Shows its items; clears the search | | Grid scrolls to the top |
| Type in search | Matches items in every category by word prefix, ignoring case and accents | | Category chips show none chosen; an empty query goes back to the chosen category |
| Tap an item whose required groups all have defaults | Rings it up with the defaults | `order.opened` (first item only), `line.added` | The line appears faint, then settles when the server answers; the total moves |
| Tap an item with a required group and no default | Opens the ModifierSheet, defaults chosen | | Add is disabled, and the message line names the first group still to choose |
| Tap a choice | Pick-one groups swap; an optional pick-one clears on a second tap; a full pick-several group refuses another | | The live price moves |
| Add to ticket | Rings the item up with the choices | `line.added` | The sheet closes |
| Modify on a line | Opens the sheet with the line's own choices | | "Save changes" |
| Save changes | Sends from and to | `line.modified` | Closes on success; a refusal stays in the message line |
| The line moves on while the sheet is open (another till, sent, voided) | Seen through the stream | | Save disabled, with why |
| Retry on a refused line | Sends the same command with the same key | `line.added` | |
| Remove on a refused line | Forgets it locally | | Nothing was recorded, so nothing is voided |

**Taps to add a common item: one.** Filter coffee is the first item of the
first category, which is chosen when the screen opens; one tap rings it up
with Regular size and Regular sugar. An item whose options must be chosen
takes three: the item, the choice, Add. Changing a default (oat, not dairy)
takes the item, Modify, the choice, Save.

## States

- **Loading:** the grid shows "Loading the menu"; the rail is already
  usable and empty.
- **Empty menu:** "No items on the menu yet. Add them in the back office."
  A category with no items says so in the grid.
- **No search match:** "Nothing matches "latte"." with Clear search.
- **Menu error:** says the server could not be reached, with Try again. A
  menu already cached keeps working while the server is away.
- **Empty ticket:** "No items yet" in Instrument Serif italic, "Tap the menu
  to start a ticket". Totals show zero.
- **Offline:** a tap still shows its line as Adding; when the command fails
  it becomes a refused line with Retry, never a silent loss. The connection
  indicator in the shell header says why.
- **Permission denied:** `order.open` is needed to ring up; a refusal shows
  on the line as its reason.

## Constraints

- The total block never scrolls away. It is outside the scrolling line list.
- Every target is 44px or more: category chips, item tiles (7rem), Retry and
  Remove.
- Every number is mono and right-aligned: counts, prices, quantities, totals.
- Vermilion is only for money and now: the total figure, and nothing else
  on this screen. Categories, tiles and the ticket id do not use it.
- A tap on a tile must never be lost to a slow server: the line appears at
  once as Adding, and taps are sent in order, one at a time, so the first
  item's order exists before the second item is added to it.

## Reference

`docs/design/reference/Nofiltr Terminal.dc.html`, block `s_register`, line
76. Departures:

- **Search replaces "Open items" and "Favourites"** in the top right. Neither
  has a backing feature yet; search is what the task asks for.
- **The rail is 26rem, not 392px.** Labels are 15px here (ADR-014), and
  the design's 11px actions would wrap at 392px.
- **No item note line or coloured top border** on tiles. The menu has no
  descriptions or item tags; the station stands in as metadata.
- **No Discount, Note, Hold, Fire or Charge yet.** They arrive with Tasks 4
  and 5 and phases 07 and 08; a disabled Charge button would be a vermilion
  call to action that does nothing.
- **No "Guest" picker** in the rail header: there are no customers until
  phase 10.
- **The ticket id is not vermilion.** The design colours `#ticket` and the
  total both; one accent per screen means the total keeps it.
- **Quantity steppers and Void on lines** are Task 4.
- **Modify only until the kitchen has the line.** The design shows Modify on
  every line. After a line is sent or its course fired, a change is a void
  and a new line (Task 4), which needs a manager.
- **No negative price deltas** ("no milk, less 20"): the orders domain
  refuses them, so the menu does too (ADR-071).

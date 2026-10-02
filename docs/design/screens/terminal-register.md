# Terminal: Register

The till's main screen: find an item, ring it up, see the ticket and its
total. Built in phase 06: the menu side and the ticket rail in Task 2,
modifiers in Task 3, cart editing in Task 4, open tickets and hold in Task 5.
Charge arrived in phase 07 Task 1 and opens the tender screen
(`terminal-tender.md`).

## Purpose

Ring up what the guest asked for in as few taps as the item allows, with the
ticket's total in view the whole time.

## Layout

Two columns inside `SignedInShell`, filling the screen below its header.
Nothing but the item grid and the line list scrolls.

- **Menu column** (flexible):
  - Top, fixed: the **category rail**, one 48px chip per category with its
    item count in the numeral face, wrapping onto a second row rather than scrolling
    sideways. The chosen category is the raised-high surface with a strong
    border; the rest are raised. Right of the chips, the **search field**.
  - Below, scrolling: the **item grid**, `repeat(auto-fill, minmax(10.5rem,
    1fr))`, gap 10px. A tile is at least 7rem tall: the name (base, semibold,
    wraps), the station as metadata, and the price in the numeral face at the bottom,
    right-aligned. An item with modifier groups says how many choices it has.
- **Ticket rail** (26rem, chrome surface, hairline on the left):
  - Top, fixed: the metadata trail (`TICKET · TABLE 4 · 2 COVERS`), then
    the order id in the numeral face, as written: the trail uppercases, and `ord_` ids
    are case sensitive. On the right, **Tickets** with the count of open
    tickets, and **History** under it. A held ticket shows "Held · On hold: nothing can be added"
    and **Resume** under the header.
  - Middle, scrolling: the **lines**. Quantity in the numeral face, the name, the chosen
    modifiers under it in muted text, the line's amount in the numeral face on the right.
    Each line is one tap target that opens its **LineSheet**. A voided line
    stays, struck through, with a "Void" chip and its reason; a comped one
    has a "Comped" chip; a line discount shows under the line. Notes show
    under their line, and order notes above the lines. Lines still on their
    way to the server are shown faint with "Adding".
    A line the server refused stays, in danger, with **Retry** and
    **Remove**.
  - Bottom, fixed, never scrolls: **Discount**, **Note** and **Hold** for
    the whole order, then, when there are discounts, **Items**, **Line discounts** and
    one row per order discount; then **Subtotal**, **Tax**, and **Total** in
    the serif at `--text-total` via `Money variant="total"`. Once
    anything is paid, **Paid** and **Balance** under it. Last, **Charge**,
    56px and primary, with why it is disabled under it when it is
    (nothing on the ticket, held, lines still Adding, nothing owed).
- **TicketsSheet**: every ticket not closed, oldest first: where it is
  ("Table 4 · 2 covers" or "Counter"), a Held or Settling chip, the order
  id, its item count, its total, and how long it has run in the numeral face ("12m",
  "1h 05m"). Past the venue's threshold (`terminal.ticket_late_minutes`,
  20 by default) the time turns terracotta with a "Late" chip: now. The
  ticket in front says "On screen". Below: **Counter sale**, and a table
  number on the pad with a covers stepper, **Start table 4, 2 covers**.
- **HistorySheet** (Task 6, read-only): the order's events, oldest first,
  through the `Timeline` primitive. Each entry: the time in the numeral face in the
  venue's time zone, a dot in the event's tone, what happened, its detail
  (a void or comp reason, a discount's reason, a note, a modifier change as
  "Dairy → Oat"), and who did it, "authorised by" when someone else
  approved it, and on which till. It refreshes as the order moves.
- **LineSheet**: quantity stepper and a note, saved together; Modify,
  Discount, Comp and Void. A line the kitchen has seen shows its quantity
  without a stepper.
- **ApprovalSheet** (void, comp, discount): first what and why (a reason
  from a fixed list, and a discount's percentage or amount on the pad),
  then who approves: staff names, the signed-in person chosen already, and
  their PIN on the pad.
- **ModifierSheet** (`Sheet`: bottom on touch, side panel on desktop): the
  item's name, then each group with "Required · Choose 1" or "Optional · Up
  to 3" on the right, its choices two to a row, 48px, a price delta in the numeral face
  where there is one. Fixed at the bottom: a reserved message line and the
  56px primary button, "Add to ticket" or "Save changes", carrying the
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
  (`mez.terminal.ticket`), so a reload comes back to it.
- `POST /api/v1/orders/{id}/lines/{line}/modifiers` `{ from, to }`: change
  a line's modifiers (`line.modified`, ADR-071), sent through the same
  ordered queue as taps, with its own `Idempotency-Key`.
- `.../lines/{line}/qty` `{ from, to }`, `.../notes` `{ scope, line_id?,
  text }`, `.../lines/{line}/void` and `/comp` `{ authorised_by, pin,
  reason_code }`, `.../discounts` `{ authorised_by, pin, scope, line_id?,
  kind, basis_points | amount, reason }`: every one through the ordered
  queue with its own key. `GET /api/v1/staff` for the approver's name.
- `GET /api/v1/orders/{id}/events`: the order's events and the names of
  every staff member and device the venue has had (inactive and revoked
  included), plus its time zone. Key `["order", id, "events"]`, under the
  order's key, so the order's invalidation refreshes it.
- `GET /api/v1/orders`: the open tickets and `late_after_minutes`, key
  `["orders", "open"]`, refetched on any event in the stream.
  `POST .../hold` and `.../resume` park and unpark a ticket.
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
| Tap a line | Opens its LineSheet | | |
| Tickets, then a ticket | Brings it to the front; a held one is resumed as it comes | `order.resumed` if held | Any sheet on the old ticket closes; lines still sending there keep going to it |
| Tickets, New: counter or a table | A new ticket in front; the order opens with the table and covers on its first item | `order.opened` | |
| Hold | Parks the ticket, and a counter ticket takes its place | `order.held` | Listed as Held |
| Resume on a held ticket | | `order.resumed` | The banner goes |
| Change quantity or write a note, Save | Sends the quantity, then the note | `line.qty_changed`, `note.added` | |
| Void or Comp, reason, approver and PIN | Sends it with the approval | `line.voided` / `line.comped` | Struck through / marked, and off the total |
| Discount (line or order), size, reason, approver and PIN | Sends it | `discount.applied` | A row under the line, or in the totals |
| A wrong PIN | | | "That PIN isn't right. Try again." The PIN clears; the choices stay |
| Note on the order | | `note.added` | Shown above the lines |
| Retry on a refused line | Sends the same command with the same key | `line.added` | |
| Remove on a refused line | Forgets it locally | | Nothing was recorded, so nothing is voided |
| Charge | Opens the tender screen on this ticket | | Any sheet closes; Back to ticket returns here |

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
- **Empty ticket:** "No items yet" in the serif, "Tap the menu
  to start a ticket". Totals show zero.
- **Offline:** a tap still shows its line as Adding; when the command fails
  it becomes a refused line with Retry, never a silent loss. The connection
  indicator in the shell header says why.
- **Permission denied:** `order.open` is needed to ring up; a refusal shows
  on the line as its reason.
- **Sold out** (phase 10, Task 2d1, ADR-109): the tile stays in its place,
  its name struck and muted, "SOLD OUT" under it in the warning colour, and
  it cannot be tapped (a screen reader hears "Latte, sold out"). The till
  compares the item's `sold_out_until` with its own clock once a minute, so
  the tile comes back by itself when the trading day turns.
- **Photo** (phase 10, Task 2d2, ADR-110): an item with a photo shows it
  as an 80px strip across the top of its tile, above the name, loaded
  lazily; greyed when sold out. It carries no meaning of its own (empty
  alt): the name is the label.

## Constraints

- The total block never scrolls away. It is outside the scrolling line list.
- Every target is 48px or more: category chips, item tiles (7rem), Retry and
  Remove.
- Every number is tabular and right-aligned: counts, prices, quantities, totals.
- Green commits: the Charge button, and the chosen category's pill, which
  says where you are. The total is the heading ink, not green. Tiles and
  the ticket id do not use it.
- A tap on a tile must never be lost to a slow server: the line appears at
  once as Adding, and taps are sent in order, one at a time, so the first
  item's order exists before the second item is added to it.

## Reference

The layout was first drawn from the Nofiltr archive
(`docs/design/reference/Nofiltr Terminal.dc.html`, block `s_register`),
kept as history. Its look is now `docs/design/tokens.md` and
`docs/design/reference/mez/mez Desktop POS.dc.html`. Departures from the
first:

- **Search replaces "Open items" and "Favourites"** in the top right. Neither
  has a backing feature yet; search is what the task asks for.
- **The rail is 26rem, not 392px.** Labels are 15px here (ADR-014), and
  the design's 11px actions would wrap at 392px.
- **No item note line or coloured top border** on tiles. The menu has no
  descriptions or item tags; the station stands in as metadata.
- **Send and Fire** (phase 08 Task 3, ADR-089) sit over Discount, Note and
  Hold. **Send N** sends what is waiting, each station only its own items,
  and fires the first course while none has gone; **Fire course 2** fires
  the next course held back. A line under them says which course Send
  fires, or what is held.
- **No "Guest" picker** in the rail header: there are no customers until
  phase 10.
- **The ticket id is not coloured.** The first design coloured `#ticket`
  and the total both.
- **Line actions live in a sheet, not on the line.** The design puts
  34px `−`, `+`, Modify and Void buttons, in 11px type, on every line. Under
  the 48px and 15px rules that doubles each line's height, and puts Void a
  thumb's width from `+`. One tap on the line opens them instead.
- **Quantity only until the kitchen has the line** (ADR-072); after that,
  void and ring up again.
- **Discount, Note and Hold sit over the totals**, under Send and Fire.
- **Open tickets are a sheet, not a rail.** The design has no ticket list
  on the register; the Orders screen (`s_orders`) is the full list with
  history, and arrives with it.
- **Modify only until the kitchen has the line.** The design shows Modify on
  every line. After a line is sent or its course fired, a change is a void
  and a new line (Task 4), which needs a manager.
- **No negative price deltas** ("no milk, less 20"): the orders domain
  refuses them, so the menu does too (ADR-071).

# Terminal: the order screen, and the item modal

Where an order is taken: find a dish, add it, see the order and its total, and
go on to Save or Pay. Built in phase 06 and rebuilt from D03 and D04 in the
rebuild's Tasks 5 and 6 (ADR-151, ADR-153, ADR-154). `web/terminal/src/
register/`: `OrderScreen.tsx`, `CategorySidebar.tsx`, `MenuHeader.tsx`,
`ItemGrid.tsx`, `OrderRail.tsx` with `RailLines.tsx`, `RailFooter.tsx`,
`OrderTotals.tsx`, `ServiceControl.tsx` and `MoreActions.tsx`,
`ItemModal.tsx`, and the sheets. At `/pos`.

A waiter's is the same screen rearranged (T02): `waiter-order.md`.

## Purpose

Take what the guest asked for in as few taps as the dish allows, with the
order's total in view the whole time.

## Layout

Three columns inside the frame (`terminal-shell.md`). Only the item cards and
the order's lines scroll.

- **Categories, 240px, chrome:** **All Items**, **Popular** when the menu
  marks anything popular, then each category in menu order. Each is 48px: an
  icon guessed from the category's name, the name, and its count in the
  numeral face. The chosen one is the terracotta fill. They stack and scroll
  down, never sideways.
- **The menu, the rest:**
  - **The head:** what is showing in the serif ("All Items", a category's
    name, "Search results"); the dietary filters as pills, **Veg**,
    **Non-Veg**, **Vegan**, **Gluten Free**, each its own tone with a tick
    when on; and the search field at the right.
  - **The cards**, `repeat(auto-fill, minmax(15rem, 1fr))`, gap 16px. A card
    has a 112px block on top, the photo or, with none, its category's icon on
    the sunken ground; a crown top right when popular. Under it: the diet
    mark, the name in the serif, a gluten-free mark, up to two lines of
    description, "6 choices" when it has any, the price, and a round
    terracotta **+**, 48px, bottom right.
- **The order rail, 400px, chrome:**
  - **The head:** where the order is in the serif ("Table T4", "Table T4 +
    T7", "Takeaway", "Delivery"), or **Pick a table** for a dine-in order with
    none; the order's id under it in the numeral face; "4 guests" as a pill;
    and the round **...**.
  - **How it is served:** Dine-in, Takeaway, Delivery as one segmented
    control, the chosen one green.
  - **A saved order**, on hold: the Held chip, "Saved: nothing can be
    added." and **Resume**.
  - **The lines**, scrolling. Each: the diet mark and the name, what it comes
    to, its choices in muted text, its notes, a line discount; at the right a
    stepper for how many and a bin. A line the kitchen has seen shows "×2"
    and no stepper. A Ready chip when its ticket is on the pass. A voided
    line stays, struck through, with its chip and reason. A line still on its
    way to the server is faint with "Adding"; one the server refused is in
    danger with "Not added", **Retry** and **Remove**.
  - Under the lines: each note on the order, and **Add a note to this
    order**.
  - **The foot, never scrolls:** **Send to Kitchen**, only while something
    waits to go; the totals; then **Save Order** and **Pay** with what is owed
    and an arrow (the one primary, 56px). Under them, why Pay is shut when it
    is.

**The totals** (`OrderTotals`, shared with Tables): with discounts, Items,
Line discounts and a row per order discount with its reason; Subtotal; a row
for each tax the server names ("CGST 2.5%", "SGST 2.5%"), or one "Tax"; Paid,
once anything is; then a rule and **Total**, or **Balance**, in the serif as
the one total on the screen.

**The "..." menu** holds what the design draws no button for: **Orders** with
the count open, **Order history**, **Discount the order**, **Fire course N**
when a course is held back, **Past bills**.

## The item modal (D04)

A dialog across the middle, up to 1020px wide.

- **Left, 400px, from 1024px wide:** the photo, or the category's icon; a
  **Popular** pill top left.
- **Right:**
  - The diet mark and the name in the serif, the description, and at the
    right the price as a total and a round close button.
  - **Each group of choices**, its name in the serif and what it asks at the
    right: "Required" in danger, "Required · Choose 2", "Optional", "Select
    multiple", "Up to 3". Where one is chosen, radio tiles three across;
    where several may be, checkbox tiles two across. A choice with a price
    shows "+₹30". The chosen tile is the green tint.
  - **Course**, a select, for a new line on a menu with more than one course.
  - **Kitchen note**, up to 200 characters.
  - **The foot:** a reserved line for what is in the way; the stepper (1 to
    99); **Cancel**; and **Add to Order · ₹240.00**, or **Save changes ·
    ₹240.00** for a line being changed, with the line's price live.

## Sheets

Side panels on a desktop, bottom sheets on touch; one at a time.

- **A line** (tap it): the stepper, or "2 × with the kitchen: void to
  change"; its notes and a field to add one; **Modify** (opens the item
  modal on the line), **Discount**, **Comp**, and **Remove (rung up by
  mistake)** before the kitchen has it, **Void** after. **Save**, or **Done**
  when nothing changed.
- **Void, Comp, Discount** (`ApprovalSheet`): first what and why, a reason
  from a fixed list, and for a discount Percentage or Amount keyed on the
  pad; then "Who is approving this?", a person and their PIN.
- **Note on the order:** "Everyone working this order sees it", up to 200
  characters, **Add note**.
- **Orders:** every order not closed, oldest first: where it is, a Held or
  Settling chip, its id, its item count, its total, and how long it has run; a
  Late chip and terracotta time past the venue's limit (20 minutes by
  default); "On screen" on the one in front. Under them **Past bills**,
  and **New order**: **Counter sale** and **Seat a table**. Closing the
  day is not here: it is under the profile, for someone with
  `shift.manage` (ADR-197).
- **History:** the order's events, oldest first, through `Timeline`: the time
  in the venue's zone, what happened and why, who did it, who approved it,
  and on which device. Read-only.

## Data

- `GET /api/v1/menu`, through `useMenu()`: kept for good, read again only
  when the stream names another menu version (ADR-069).
- `POST /api/v1/orders` on the first item of an order (the service type, and
  the table and guests if it has them), then `POST /api/v1/orders/{id}/lines`
  with the item's snapshot: name, price, choices, station, course, tax rate,
  HSN. Each tap has its own `Idempotency-Key`, reused by Retry.
- `GET /api/v1/orders/{id}`, key `["order", id]`. A command's answer replaces
  the cached order, so the rail redraws from the server's projection.
- The order in front is kept in `sessionStorage` (`mez.terminal.ticket`) and
  above the routes, so a reload or another screen comes back to it.
- On a line: `.../lines/{line}/modifiers` `{from, to}`, `.../qty` `{from,
  to}`, `.../void` and `/comp` with the approval, and a remove, which is a
  void needing no manager (ADR-073). On the order: `.../notes`,
  `.../discounts`, `.../send`, a course's fire, `.../hold`, `.../resume`. All
  go through one ordered queue, one at a time.
- `GET /api/v1/orders`, key `["orders", "open"]`: the open orders and
  `late_after_minutes`, read again on any order event.
- `GET /api/v1/orders/{id}/events`: the history, with the names of every
  person and device the venue has had.
- `GET /api/v1/staff`: who can approve.
- WebSocket: an event for the order in front, or a resync, reads it again, so
  a line added on another device appears here.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Tap a category | Shows its items; empties the search | | |
| Type in search | Finds items in every category whose name has a word starting with each word typed, ignoring case and accents | | "Search results"; no category shows chosen |
| A filter pill | Toggles it. Veg, Non-Veg and Vegan add up; Gluten Free narrows what they leave | | |
| **+** on a card | Adds one with its default choices | `order.opened` (first item), `line.added` | The line appears faint, then settles. If the defaults leave a required choice open, the modal opens instead |
| Tap a card | With choices: opens the item modal, defaults chosen. Without: adds it | | |
| Tap a choice | One-of groups swap; an optional one-of clears on a second tap; a full group refuses another | | The button's price moves |
| Add to Order | Adds the line with its choices, quantity and course, then its note | `line.added`, `note.added` | The modal closes |
| Save changes | Sends what differs, in turn | `line.modified`, `line.qty_changed`, `note.added` | Closes. If only part was saved it closes and a toast says so |
| Stepper on a line | Changes how many | `line.qty_changed` | |
| Bin on a line | Before the kitchen has it, takes it off; after, opens Void | `line.voided` | Struck through, and off the total |
| Void or Comp, reason, approver, PIN | | `line.voided`, `line.comped` | Struck through, or marked |
| Discount, size, reason, approver, PIN | | `discount.applied` | A row under the line, or in the totals |
| Pick a table | Opens Tables | | Seating there puts the order in front |
| Dine-in, Takeaway, Delivery | Before the order opens, starts it as that | | Once open, the other two are dimmed: "Set when the order started" |
| Send to Kitchen | Sends what waits, each station its own items, and fires the first course while none has gone | `ticket.sent`, `course.fired` | The button goes until something else waits |
| Fire course N | Fires the next course held back | `course.fired` | |
| Save Order | Parks the order | `order.held` | A new order takes its place; this one is under Orders |
| Resume | | `order.resumed` | The banner goes |
| Orders, then an order | Brings it to the front, off hold if it was saved | `order.resumed` if saved | Any sheet on the order left closes |
| Pay | Opens `/pay/:id` on this order | | `terminal-tender.md` |

**Taps to add a common dish: one**, the "+" on its card. A dish whose choices
must be made takes three: the card, the choice, Add to Order.

## States

- **Loading:** "Loading the menu" where the cards go; the rail is there and
  empty.
- **Empty menu:** "No items on the menu yet. Add them in the back office." A
  category with nothing in it: "Nothing in this category yet."
- **No match:** "Nothing matches "latte"." with **Clear search**; "Nothing
  here matches the filters." with **Clear filters**.
- **Menu error:** "Can't reach the mez server to load the menu." with **Try
  again**. A menu already read keeps working while the server is away.
- **Empty order:** "No items yet" in the serif, "Tap a dish to start the
  order". The totals show zero.
- **Offline:** a tap still shows its line as Adding; when the command fails
  it becomes "Not added" with Retry, never a silent loss.
- **Sold out** (ADR-109): the card stays in its place, faded, its name
  struck, "Sold out" in the warning colour, and neither it nor its "+" can be
  tapped. A screen reader hears "Latte, sold out". The clock is checked once
  a minute, so it comes back when the trading day turns.
- **A line moved on while its modal is open:** saving is shut, with why:
  "This has gone to the kitchen. Void it and add it again."
- **Pay is shut:** "Nothing to pay yet.", "Resume this order to pay it.",
  "Wait for every item to be added." A paid order still open reads **Invoice
  and close** or **Close order** in place of Pay.
- **A wrong approval PIN:** "That PIN isn't right. Try again." The PIN
  clears; the choices stay.
- **Permission denied:** a refusal shows on the line as its reason, or as a
  toast naming what was tried.

## Constraints

- The totals and the two buttons never scroll away.
- Every target is 48px or more: categories, filters, the "+", the bin, the
  stepper. Save Order and Pay are 56px.
- Every number is tabular and right-aligned.
- Green is the one commit, Pay. Terracotta says where you are (the category)
  and what to tap next (the "+"). The total is the heading ink.
- A card looks finished with no photo.
- A tap is never lost to a slow server: the line shows at once, and taps go
  in order, so the order exists before its second item.
- Quantity and choices change only until the kitchen has the line (ADR-071,
  ADR-072); after that it is voided and added again.
- No negative choice prices: the orders domain refuses them (ADR-071).

## Reference

`mez Desktop POS.dc.html`, D03 POS Order (line 175) and D04 Item Modal (line
293). Left out, and why:

- **"Sort: Popular" and the list or grid switch** over the cards: nothing
  does either. Popular is a category instead.
- **Search in the top bar, with Ctrl + K:** the search is in the menu's head,
  beside what it searches, and has no shortcut.
- **"Also ordered with"** in the modal: nothing records what sells together.
- **The modal's photo in an arch:** a plain block, so a card and its modal
  match with no photo.
- **Spice level** is not a section of its own: it is a group of choices if
  the menu has one.

Changed: the design's "Bestseller" pill reads **Popular**, the word the menu
uses. "Add Note" at the rail's head is the row under the lines only.
**Send to Kitchen** on this screen is ours: the design draws it on Tables,
and a counter order has no other way to reach the kitchen (ADR-151).

Kept, though not drawn (rule 4): Orders, Order history, Discount the order,
Fire course and Past bills under "..."; the line's sheet, with Comp and Void;
the approval by a second person's PIN.

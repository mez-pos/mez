# Waiter: a table, and Transfer and merge

One table's own screen: who is at it, what they ordered and where it stands in
the kitchen, and what to do next. Built from T03 and T06 in the rebuild's Task
13 (ADR-169 to ADR-171). `web/terminal/src/waiter/TableDetail.tsx`,
`TableDetailOrder.tsx`, `TableItems.tsx`, `OrderProgress.tsx`,
`MoveTable.tsx`, `table-logic.ts`, `move-logic.ts`. At `/waiter/table/:id`.
The top bar there is **Back** alone (`terminal-shell.md`).

## Purpose

Standing at a table, know what has reached the kitchen and what has not, and
send, bill or move the party without leaving it.

## Layout

Three columns under the bar; only the notes and the items scroll.

- **Left, 296px:**
  - **The table**, a green card: "Table T4" (or "Table T4 + T7") in the
    serif, the guests, how it is served, and "Started 7:14 PM · 1h 12m".
  - Its group as a pill in its tone (**Occupied**, **Needs Bill**, or
    **Saved** for an order on hold), and **Merged** beside it for a joined
    party.
  - **Table Notes (2):** each note on the order, and **Add note**; a script
    line under them.
- **Middle, the rest:** a card.
  - **All Items (8)** as a green pill, and **Group by Course**, a toggle, on
    to start.
  - By course: each course with an icon, its name and count, a chip (With
    the kitchen, To send, Held), and **Fire** on the next one held back.
    Each line: its quantity, name, choices, a chip (**Ready**, **In the
    kitchen**, **Not sent**) and what it comes to. Ungrouped: the same
    lines as one list.
  - **Add Item** along the foot.
- **Right, 300px:**
  - **Order Status**, with a "Live" pill: four steps, each done (green),
    where the order is now (mustard, ringed) or not reached: **Order
    placed**, **Sent to kitchen**, **In preparation**, **Ready to serve**,
    each with a time where the order has one and a line of detail ("3
    items", "2 items to send", "2 items at the pass").
  - Four tiles: **Bill and pay** (terracotta tint), **Details**,
    **Transfer**, **Merge**. Under them, why the bill is shut when it is.
  - The totals, tax by tax.
  - **Send to Kitchen (3)** (the primary, 56px), or "Sent".

## Transfer and merge (T06)

A modal, "Move Table T4", with "8 items · ₹2,445.00 · 6 guests" under the
title.

- **Transfer** and **Merge** as a segmented control; it opens on whichever
  tile was tapped.
- A line saying what to pick: "Choose the destination table", or "Choose free
  tables to join to this party. Tap a joined one to let it go."
- **Every other table as a tile**, area by area: its name and a word. A free
  one is green, "Available · 4" (its seats), and can be taken. One joined to
  this party is blue, "Joined", and can be let go in Merge. The rest are
  dimmed and say why not: "Occupied", "Needs Bill", "To Clear".
- **Cancel**, and a button that says what it will do: **Move to T5**, **Join
  T5 and T8**, **Let go T7**, **Join T5 and let go T7**.

## Data

- The live floor, held above the routes, for the table and its party.
- `GET /api/v1/orders/{id}`, read afresh on opening and kept current by the
  stream.
- `GET /api/v1/menu` for the courses' names; `GET /api/v1/venue` for the
  zone.
- The order's `.../send`, its course's fire, and `.../notes`.
- Move: `POST /api/v1/orders/{id}/table` `{from, to}`. Join and let go:
  `.../tables/join` and `.../tables/part`, one request for each table, in
  turn.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Back | The tables | | |
| Add note | The order's note sheet | `note.added` | Listed under Table Notes |
| Group by Course | Toggles the grouping | | |
| Fire | Fires that course | `course.fired` | |
| Add Item | Brings the order to the order screen, off hold if it was saved | `order.resumed` if saved | `waiter-order.md`; its back arrow returns here |
| Send to Kitchen (3) | Opens Confirm and send | `ticket.sent` on sending | `waiter-order.md` |
| Bill and pay | Opens the bill | | `waiter-bill.md`. Reads **Invoice and close** or **Close order** for a paid order still open |
| Details | The order's history sheet | | |
| Transfer, a free table, Move to T5 | Moves the party | `order.table_changed` | This screen follows it to T5 |
| Merge, tiles, Join | Joins or lets go, a table at a time | `order.tables_joined`, `order.tables_parted` | The title reads "Table T4 + T5", with Merged |

## States

- **Loading:** "Opening the table", then "Opening the order".
- **A table with no one order** (free, to clear, or with several): this
  screen is not for it and sends on to Tables with the table picked. A
  waiter's card for a free table or one to clear never leads here
  (`waiter-home.md`).
- **The table is gone from the floor:** "This table is not on the floor any
  more." with **Back to the tables**.
- **Error:** the server's words, with **Try again**.
- **Refused:** the server's words over Send, or in the modal.
- **A merge part done:** if one table is refused, the ones before it stand,
  and the modal says what was refused.
- **Items still on their way** from this tablet to this order: Send is shut
  until they land, so it cannot overtake them.

## Constraints

- Colour and a word on every chip and step.
- The tiles are 56px; Send to Kitchen is 56px; everything else 48px or more.
- One green commit: Send to Kitchen. Bill and pay is the terracotta tint.
- Lines are read here and changed on the order screen.
- A note keeps neither a time nor a name, so none is shown.

## Reference

`mez Tablet Waiter.dc.html`, T03 Table Detail (line 247) and T06 Transfer
Merge (line 489). Left out, and why:

- **Guest QR orders, the "Rohan" and "QR Order" tabs, the QR chip on a line,
  and "(5 waiter, 3 QR)":** nothing feeds them before the guest channel
  (phase 11), and a line does not say who rang it up.
- **Preferences, Split Bill, Print KOT, and the pencil on the name:** nothing
  does them from here. A split is made on the bill.
- **Mark as Served, and the Completed step:** no such step is kept
  (ADR-090).
- **"~6 min" on In preparation:** nothing estimates it.
- **A note's time and author.**
- **"Guest QR sessions follow the table"** in the modal: phase 11. **Notify
  the kitchen of the new table number:** the kitchen's board reads the table
  from the order, so it follows untold.

Changed: "Request Bill" is **Bill and pay**, since the waiter takes the
payment. The design's "Active" pill is the table's group, **Occupied**. Its
second "Course View" button under the items is the Group by Course toggle
above them.

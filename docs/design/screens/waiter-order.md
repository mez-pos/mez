# Waiter: Order taking, Confirm and send, Course view

The order screen as a waiter has it: the same screen as the cashier's,
rearranged for a tablet at the table, ending in Send to kitchen where the
cashier's ends in Pay. Built from T02 and T04 in the rebuild's Task 12
(ADR-165 to ADR-168). The waiter's variant of `web/terminal/src/register/`
(`OrderScreen.tsx`, `CategorySidebar.tsx`, `OrderRail.tsx`, `RailFooter.tsx`),
with `ConfirmSend.tsx`, `CourseView.tsx`, `send-logic.ts` and
`waiter/WaiterOrderBar.tsx`. At `/pos`.

**Read `terminal-register.md` first.** This spec says only what differs.

## Purpose

Take a table's order standing at it, read back what will reach the kitchen,
and send it.

## What differs from the cashier's order screen

- **The top bar is the order's** (`WaiterOrderBar`), in place of the
  navigation: a round back arrow; the table as a terracotta pill ("Table T4",
  or **Pick a table**), which opens Tables; the guests as a pill; and how it
  is served. Before the order opens that is a choice, Dine-in, Takeaway,
  Delivery: wide buttons from 1280px, narrow ones below. Once it is open the
  service cannot change (ADR-146), so it is a pill with one word and its
  icon ("Dine-in"), not three buttons (TBS-3). The table's pill is the one
  that gives way: its name is cut short with an ellipsis before anything in
  the bar runs under the connection, Not printed or the profile at the
  header's right. At 1024x768 an open order's bar reads: back, "Table T5",
  "4 guests", "Dine-in", and none of it overlaps.
- **The rail has no service control of its own**, and in the waiter's frame
  its head drops the guests chip, the service chip and the order's id: the
  bar above says the first two, and the room goes to the table's name
  (TBS-13).
- **Categories, `w-side`:** the search is on top of them, "Search" (the
  cashier's register has it there too since ADR-381); the chosen one is
  green, not terracotta; a script line at the foot. The menu's head has the
  title and the diet filters only: beside the title from 1280px, behind one
  **Diet** button below it ("Diet", "Diet: Veg", "Diet: 2 on").
- **Cards keep the crown beside the price.** A card with no photo has no top
  band on a screen 820px tall or less; a photo keeps a 64px band.
- **At 768 tall the rail folds its totals** into Subtotal and one "incl.
  GST" row that opens on a tap, and each line puts its stepper and bin side
  by side under the name.
- **Popular is where it starts**, on a menu that marks anything popular.
- **Cards are narrower**, `minmax(13rem, 1fr)`, so three fit beside the rail
  at 1366 wide.
- **The rail has two tabs:** **Current order (5)** and **Course view**.
- **Each line says its course** in terracotta small print ("Main Course").
- **Clear**, a danger pill under the lines, when the kitchen has not seen
  something.
- **The foot is Save and Send to kitchen.** Sending is the one green here;
  there is no Pay button. Below 1280px Send to kitchen drops its icon so the
  words fit, and the two sit 8px apart.
- **The "..." menu** leads with **Bill and pay**, and has no Fire course: a
  waiter fires from Course view, where the course is named (ADR-168).

## Course view

The order by course, to see what the kitchen has and what is held, and to let
the next course go. Nothing is changed here but by Fire.

- A section for each course: its name in terracotta, and a chip for where it
  stands. Under it each line: "2×", the name, and "Ready", "With the
  kitchen" or "Not sent".
- **Fire Main Course**, under the next course held back.

| Course state | Tone | Word |
|---|---|---|
| sent | success | With the kitchen |
| waiting (goes with the next Send) | warning | To send |
| held (until its course is fired) | info | Held |

## Confirm and send (T04)

A modal from Send to kitchen, here and on the table's screen: what one tap
will put in front of the kitchen, read before it goes.

- **A green banner:** "3 new items will be sent to the kitchen"; "5 items are
  already with the kitchen and won't be sent again."; "1 item of a later
  course waits until that course is fired."
- **Kitchen ticket preview**, a card in the numeral face: "KITCHEN ORDER
  TICKET" and the venue; the table and how it is served, with the date and
  time; each line going, "2×" and its name in capitals, its choices, its
  notes, and its course in terracotta; then "WAITER: Kabir", "GUESTS: 6", and
  each note on the order in a mustard box.
- **Back**, and **Send 3 items to kitchen** (the primary, 56px).

## Data

As `terminal-register.md`. `sendPreview` (`send-logic.ts`) works out from the
order which lines go, which the kitchen has, and which are held; the server
decides again on Send.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| The back arrow | An order at a table: its table's screen. Otherwise the tables | | Nothing is saved or lost; the order stays as it is |
| The table pill | Opens Tables | | A table is picked, or the party moved |
| Clear | Asks "Clear the order?": "The 2 items the kitchen has not seen will be taken off. Anything already sent stays." | `line.voided` each | They stay on the rail, struck through |
| Send to kitchen | Opens Confirm and send | | |
| Send 3 items to kitchen | Sends | `ticket.sent`, `course.fired` | A toast, "Order sent to kitchen · Table T4"; the button reads "Sent" |
| Fire Main Course | Fires that course | `course.fired` | Its chip turns With the kitchen |
| Save | An order at a table is left as it is; any other order is parked (`saveParks`) | `order.held` for one with no table | The tables |
| Bill and pay, under "..." | Opens the bill | | `waiter-bill.md` |

## States

As the cashier's, and:

- **Nothing to send:** Send to kitchen is shut; once everything has gone it
  reads "Sent".
- **A saved order:** Fire is not offered, and Clear is not drawn.
- **Send refused:** the server's words in the modal; nothing was sent.
- **No course on the menu:** a line with none sits under "No course".

## Constraints

- The top bar's controls are 56px; a waiter's hand is not steady.
- One green: Send to kitchen, and in the modal Send N items.
- Items added and not sent stay unsent after Save, and the table's card does
  not say so: the table's screen does, under Order status.

## Reference

`mez Tablet Waiter.dc.html`, T02 Order Taking (line 101) and T04 KOT Preview
(line 377). Left out, and why:

- **Add More:** the menu is beside the rail already.
- **T02's item sheet with the photo on top:** the item modal is D04's, one
  modal for both roles.
- **The ticket's number ("KOT #0412"):** the server gives it on sending.
- **Kitchen printer or KDS screen:** each item's station decides where it
  goes.
- **Notify me when items are ready, and Undo:** nothing does either.
- **"HOLD UNTIL CALLED" on a line:** a held course's lines are not on the
  ticket at all; the banner counts them.

Changed: the design has no way back in the order's bar, and a waiter needs
one, so an arrow stands first. Course view's layout is ours: the design
draws the tab's name and nothing under it.

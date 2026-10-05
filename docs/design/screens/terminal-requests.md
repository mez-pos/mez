# Terminal: Table requests

What guests have asked for from their phones and nobody has answered. Built
in phase 11 Task 4b2 (ADR-209, ADR-211, ADR-212).
`web/terminal/src/requests/`: `RequestsSheet.tsx`, `request-logic.ts`,
`use-guest-requests.ts`, `use-request-chime.ts`. The pill is in
`shell/TerminalShell.tsx`; the mark on a table is `WantsToOrder` in
`floor/FloorPlan.tsx`.

## Purpose

A guest at a table nobody has seated places an order from their phone. It
is a request until a member of staff says someone is really there. This is
where staff hear of it, see what it asks for, and accept or decline it.

## Layout

- **Where it opens from:** a terracotta-tint pill in the top bar, a bell and
  "1 table request" or "3 table requests", on every screen of both frames.
  It is there only while something is waiting.
- **The sheet** (`Sheet`: bottom on touch, a side panel with a mouse), titled
  "Table requests": the requests, oldest first. Each:
  - "Table T4" in the serif, and at the right "Waiting 4 min"
  - each line: the count, the dish, its choices in small print under it, and
    what the line comes to
  - "3 dishes, before tax" and the subtotal
  - **Accept and send**, the primary, and **Decline**
  - an error line when the server refused
- **On the floor:** a table with a request waiting shows a terracotta mark,
  a bell and a word, where its state chip is: "Request" on a tile under
  1536px wide and "Wants to order" from there, in the Table List's state
  column, and on the waiter's card under the group's word. The tile's
  spoken label ends ", wants to order".

## Data

- `GET /api/v1/guest-requests`, key `["guest-requests"]`, read once and
  again on the stream's `floor_changed` and on a resync. Read only by
  someone with `order.open`.
- `POST /api/v1/guest-requests/{id}/accept` and `.../decline`. After either,
  whatever the answer, the requests, the live floor and the open tickets are
  read again.
- Held above the routes (`TerminalProvider`: `guests`, `wanting`), so the
  pill, the sheet and the floor read one list.
- Nothing here is worked out on the terminal: the lines, their prices and
  the subtotal are the server's, as priced when the guest asked.

## Interactions

| Action | Does | Then |
|---|---|---|
| The pill | Opens the sheet | |
| Accept and send | Puts the lines on the table's ticket, opening one if it has none, and sends them to their stations | The request leaves the list; the table reads Ordered |
| Decline | Arms: the button reads "Tap again to decline" for four seconds | A second tap declines; the request leaves the list |
| Close | Closes the sheet | The pill stays while anything waits |

Each request is answered on its own, also when two are from one table.

## The sound

- The kitchen's chime, two rising notes, when a request arrives, and again
  every 30 seconds while any waits.
- A terminal opened onto requests already waiting chimes for them.
- A browser plays nothing until the page has been touched. The first tap or
  key anywhere is taken as that touch, and signing in is one. A page nobody
  has touched since it loaded is silent, and the pill is what says so.
- There is no switch to turn it off.

## States

- **Nothing waiting:** no pill. The sheet, if open: "No table is waiting to
  order from a phone."
- **Loading:** "Loading the requests".
- **Error reading:** the terminal's words for it, in danger.
- **Refused:** the row says why, and the list is read again:
  "Someone has already answered this request.", "This request is from an
  earlier shift. Decline it.", "This table's ticket is on hold or already
  invoiced. Resume it, or decline the request."
- **Offline:** the list stays as last read; an answer fails with "Not sent:
  the server can't be reached." Nothing is queued: an accept sent late could
  send food for a guest who has gone.
- **Permission:** someone without `order.open` has no pill and no mark.

## Constraints

- Everything tapped is 48px or taller.
- Attention is terracotta and a word, never the colour alone.
- A request never opens the sheet by itself: it would land on top of a
  payment being taken.
- Decline is asked twice, on the button. It tells a guest no.
- Accept is not asked twice: it is the common answer, and a ticket opened by
  mistake is one staff can void.

## Reference

`mez Desktop POS.dc.html`, D03's top bar: the bell and "Table Requests".
D13, the notifications screen, was not read.

**Left out:**
- A list of notifications of other kinds: only requests exist.
- Opening the table's ticket after Accept: staff are usually mid something
  else. The table on the floor leads to it.

**Changed:**
- The bell is a pill with a count and words, and is absent with nothing
  waiting, as Not printed is.

**Kept that the design does not draw:** how long each has waited; Decline
asked twice; the repeat of the chime; the mark on the table.

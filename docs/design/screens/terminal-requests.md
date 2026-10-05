# Terminal: From the tables

What guests have asked for from their phones and nobody has answered, and
which tables say they have paid and nobody has looked (ADR-218). Built
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

- **Where it opens from:** a terracotta-tint pill in the top bar, a bell, a
  count and words, on every screen of both frames, only while something is
  waiting: "1 table request" or "3 table requests"; "1 table says paid" or
  "2 tables say paid"; and "3 from tables" when there is some of each.
- **The sheet** (`Sheet`: bottom on touch, a side panel with a mouse), titled
  "From the tables".
- **Say they have paid,** first, when any table does. Under the heading:
  "Nothing is paid until you record it. Check the venue's phone for the
  payment first." Each:
  - "Table T5" in the serif, and at the right "Said 4 min ago"
  - "Says paid by UPI" and the amount: what the table owed when the guest
    said so, which is the figure to look for
  - "The table owes now" and that figure, in terracotta, only when it
    differs: a round was added since, or part has been recorded
  - **Take payment**, the primary, and **No payment found**
  - an error line when the server refused
- **Want to order,** the requests, oldest first, under that heading when
  both lists show. Each:
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
- The same answer carries `paid`, the tables that say they have paid.
- `POST /api/v1/guest-requests/{id}/accept` and `.../decline`, and
  `POST /api/v1/paid-notices/{id}/seen` for No payment found. After any of
  them, whatever the answer, the requests, the live floor and the open
  tickets are read again.
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
| Take payment | Closes the sheet and opens `/pay/{order}` | The table's payment screen, where the UPI payment is recorded as any other. Recording what settles the bill takes the notice off the list |
| No payment found | Arms: "Tap again: no payment" for four seconds | A second tap puts the notice away. Nothing on the bill changes, and the guest may say so again |
| Close | Closes the sheet | The pill stays while anything waits |

Each request is answered on its own, also when two are from one table.

## The sound

- The kitchen's chime, two rising notes, when a request or a table saying
  it has paid arrives, and again every 30 seconds while either waits.
- A terminal opened onto requests already waiting chimes for them.
- A browser plays nothing until the page has been touched. The first tap or
  key anywhere is taken as that touch, and signing in is one. A page nobody
  has touched since it loaded is silent, and the pill is what says so.
- There is no switch to turn it off.

## States

- **Nothing waiting:** no pill. The sheet, if open: "No table is waiting to
  order from a phone, and none says it has paid."
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
- **A table saying it has paid changes nothing on its bill**, and the sheet
  says so above the list. Take payment records nothing: it goes to where a
  payment is recorded.
- No payment found needs `payment.take`. Someone without it is refused in
  words, and can still tell a colleague.
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

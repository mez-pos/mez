# Waiter: Your shift

What the person signed in has done in the open shift, and the way to sign out
at the end of it. Built from T08 in the rebuild's Tasks 14b and 14c (ADR-173,
ADR-174). `web/terminal/src/waiter/ShiftSummary.tsx`,
`shift-summary-logic.ts`; the figures are `shift.MineOf` in
`internal/domain/shift/mine.go`. At `/waiter/shift`, under **More** in the
waiter's navigation.

## Purpose

See your own tables, orders, sales and tips for the shift before you go.

## Layout

One column, 660px, as a card in the middle of the screen. Its head and foot
stay; the middle scrolls.

- **The head, chrome:** "Your shift" in the serif, and at the right the
  shift's day and span in the numeral face, "Sat, 19 Sep 2026 · 4:00 PM –
  11:30 PM", from its opening until now.
- **Who:** initials in a round mark, the name in the serif, the role under
  it, and a green pill, "On shift · 7h 26m".
- **Four figures**, two by two, each in the serif:
  - **Tables served** (green tint)
  - **Orders taken**
  - **Sales handled**
  - **Tips collected** (mustard tint)
- A line under them: "Orders and tables are the ones you opened; sales and
  tips are on the payments you took. Tips are shared out after the shift
  closes."
- **Tips by table:** a row for each table, its name, a mustard bar against
  the largest, and the tips. Orders with no table are "No table".
- **The foot, chrome:** "End shift signs you out. A manager counts the
  drawer and closes the shift.", and **End shift** (terracotta, 56px).

## Data

- `GET /api/v1/shift/mine`, key `["shift", "mine"]`: for whoever asks, in
  the open shift: `opened_at`, `first_at` (when they first touched an
  order), `tables`, `orders`, `sales`, `tips`, and the tips by table.
- The live floor, for table names. `GET /api/v1/venue` for the zone.
- `DELETE /api/v1/sessions/current`: End shift.

## Interactions

| Action | Does | Then |
|---|---|---|
| End shift | Signs the person out | Staff login. The shift itself stays open |

## States

- **Loading:** "Adding up your shift" under the person's card.
- **Error:** the server's words, with **Try again**.
- **No tips:** "No tips yet this shift."
- **Someone who has done nothing:** the figures are zero, and "On shift"
  counts from the shift's opening.
- **Offline:** the figures stay as last read; End shift still signs out
  here.

## Constraints

- Every figure is the server's: nothing is added up on the tablet.
- "On shift" runs from the person's first order, since nobody clocks in (no
  timesheets in v1).
- A refund is not taken off Sales handled. A table the cashier opened counts
  for the cashier, not the waiter who served it.
- Tips are those taken, not this person's share of the pool: the pool is
  worked out after the close (`tips.md`).
- One button, and it is terracotta: it ends something, it takes no money.

## Reference

`mez Tablet Waiter.dc.html`, T08 Shift Summary (line 552). Left out, and why:

- **Guest ratings** ("4.9 average from 11 guest ratings"): nothing collects
  them before phase 11.
- **The area beside the role** ("Main Dining"): nobody is assigned an area.
- **Print summary:** nothing prints one.

Changed: "Tips earned" is **Tips collected**, because what a person takes
and what they are paid from the pool are two figures. **End shift** signs
the person out and no more: closing the shift is a manager's, with the
drawer count (`terminal-close.md`).

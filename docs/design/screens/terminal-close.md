# Terminal: Open the till, and close the day

The start and end of a shift at the till (phase 09, Task 2b; ADR-093 to
ADR-095). Opening counts a float into the drawer; closing counts the
drawer blind, shows how it came out, takes the reason when one is owed,
and ends the shift.

## Purpose

Nobody sells from a drawer nobody counted, and nobody at the till learns
what the drawer should hold until they have said what it does hold. A
closing takes a manager under three minutes with a full drawer.

## Layout

- **Till closed** takes the register's place whenever no shift is open.
  One centred column, 32rem:
  - "The till is closed" in Instrument Serif, and under it "Count the float
    into the drawer to start the day."
  - **The counter** (below), titled "Float".
  - **Open the till** with the float's figure, "Open the till with
    Rs. 2,000.00" (the one accent, 58px). A float of nothing is allowed.
- **Close the day** is in the Tickets sheet, under Past bills: **Close the
  day** and "Count the drawer and end the shift." It takes the register's
  place in two columns, like the tender screen:
  - **Left:** the counter, titled "Count the drawer". Nothing on this
    screen says, hints or rounds towards what is expected.
  - **Right rail** (26rem): the shift's trading day and when it opened, in
    mono; "Counted" and the counter's total as the one total; **Submit
    count** (the accent); **Back to the register**. Submit asks first:
    "The count can't be changed once it's in. Submit Rs. 9,520.00?"
- **The reveal** replaces the counter once the count is in, and is what a
  counted shift opens on (after a reload, or on another till):
  - **Expected**, **Counted** and the **variance** in mono, right-aligned.
    The variance is a word and a colour, never colour alone: "Balanced",
    "Short Rs. 9.50" (danger), "Over Rs. 5.00" (gold, the warning tone).
  - When the variance is past the venue's threshold and has no reason:
    "Rs. 9.50 is more than the Rs. 0.00 allowed. Say why before
    closing." and a reason field (up to 500 characters) with **Save
    reason**. Within the threshold the field is there but optional.
  - A saved reason shows as a quote with who gave it; it cannot be changed.
  - **Close the shift** (the accent) once the count is in and any reason
    owed is saved. Then "Shift closed" and the till is closed.
- **The counter:** one row per note and coin of the venue's currency,
  largest first (Rs. 500 down to Rs. 1 for INR): the face in mono, the
  count, and the row's value. Tapping a row picks it; the pad (`NumPad`,
  amount layout, `lg`) types that row's count, and **Next** moves to the
  row below. The picked row has the focus ring and a raised fill. Under the
  rows, the total. A currency with no table here is counted as one total
  on the pad instead.

## Data

- `GET /api/v1/shift`, key `["shift"]`: `{shift: null}` or the open
  shift. Read on the register's start and every 30 seconds (the stream
  does not carry shifts), and after every command below.
- `POST /api/v1/shift/open` `{total, denominations}`: needs shift.manage.
- `POST /api/v1/shift/{id}/count` `{total, denominations}`: the answer is
  the first place `reconciliation` appears.
- `POST /api/v1/shift/{id}/reason` `{reason}`.
- `POST /api/v1/shift/{id}/close`: needs shift.manage.
- One `Idempotency-Key` per attempt, reused on a retry of the same body.
- Denominations are sent as counted, zero rows left out; the server adds
  them up again and refuses a total that does not match.

## Interactions

| Action | Does | Then |
|---|---|---|
| Tap a row | Picks it | The pad types its count |
| Pad keys | Sets the picked row's count | The row's value and the total redraw |
| Next | Picks the row below | Past the last row, stays |
| Open the till | Opens the shift | The register, selling |
| Submit count, confirm | Records the count | The reveal |
| Save reason | Records it | The quote; Close the shift enabled |
| Close the shift | Ends the shift | "Shift closed"; the till is closed |


## Closing stock (phase 10, Task 3d, ADR-114)

- After the drawer is counted and revealed, the stock items marked **Count
  at close** in the back office are listed under it: "Count the stock",
  one field each in its unit, mono, right-aligned. **Blind**, as the
  drawer is: what the stock record says is not on the till until the
  count is in.
- **Save the stock count** sends every item together; an item left
  empty, or typed in words, is named under its field before anything is
  sent. It is taken once per shift.
- Once it is in: "Closing stock", each item's count and what it moved
  (the count against the record), a loss in the danger colour.
- **Close the shift** waits for it while there is a list to count. With
  nothing marked, the step is not shown.
- `GET` and `POST /api/v1/shift/{id}/stock-count`, `shift.manage`.

## States

- **Loading:** "Checking the till" in place of the register.
- **Offline:** "Can't reach the Nofiltr server to check the till." with Try
  again. Nothing opens or closes offline.
- **Not allowed:** a cashier's Open or Close gets "You don't have
  permission to do that. Ask a manager."; counting and the reason are open
  to anyone who takes payments.
- **Counted elsewhere:** a till that submits after another did gets
  "The drawer has already been counted." and the reveal.

## Constraints

- **Blind:** the expected figure exists on this screen only in the reveal,
  and the reveal only from the server's answer to the count.
- The only arithmetic in the UI is adding up the counter for display;
  the server's total is the one recorded.
- One accent per screen: Open the till, Submit count, Close the shift.
- Every number mono and right-aligned; every target 44px, the pad 64px.

## Reference

None in the archived design; this screen is new.

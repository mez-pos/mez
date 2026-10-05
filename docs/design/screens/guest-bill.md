# Guest: the bill

What the table has had and what it comes to. Built in phase 11 Task 4d
(ADR-214). `web/guest/src/`: `Bill.tsx`, `bill-logic.ts`, `useBill` in
`use-guest.ts`. The hash `#bill` on a table's path. Paying from the phone
is Task 4e.

## Purpose

Let a guest see the table's bill as the venue has it, tax and all, before
anyone brings it: so the figure at the counter is no surprise.

## Layout

- **The bar,** stuck to the top: back to the menu, "The bill" in the serif,
  and the table at the right.
- **The lines,** a card: each line's count and dish, its choices in small
  print, and what it comes to. A line the venue gave away reads "On the
  house" in green where its amount would be. A voided line is not shown.
- **The summary,** a card:
  - "Items (3)" and what the charged dishes come to
  - "Discount" with a minus, only when something was taken off
  - each tax by name and rate, "CGST (2.5%)". When the venue's prices
    already hold the tax, each reads "Includes CGST (2.5%)", smaller and
    muted, so nobody adds it to a total that already has it.
  - a hairline, then "Bill total" and the figure in the serif
  - "Paid" and "Left to pay", only once something has been paid
- **Pay by UPI,** a card, only when the venue has given its UPI address,
  the bill is in rupees and something is owed (ADR-215):
  - **Pay ₹540.00 by UPI**, a link drawn as the primary button, 58px, for
    exactly what is left. It opens the phone's UPI app with the venue's
    address and the amount filled in.
  - "To Demo Cafe, democafe@okbank", small, so the guest can check who is
    being paid, or type it in.
  - "After paying, show the payment to a member of staff. This bill is
    marked paid once they have seen it."
- **How to pay,** on the sunken ground:
  - owed: "To pay, ask a member of staff or pay at the counter.", or under
    the UPI card "Or pay a member of staff, or at the counter."
  - settled: "This bill is paid. Thank you."
  - nothing open: "Nothing is on this table's bill yet. What you order
    appears here once it is accepted."
- "This is the whole table's bill, whoever ordered."

## Data

- `GET /order/api/t/{code}/bill`, key `["guest", "bill", code]`: the table's
  open ticket with the till's parts taken out. Read when the screen opens,
  every 15 seconds while it is shown, and when the phone comes back. Never
  shown from an earlier look.
- Every figure is the server's projection of the ticket. The phone decides
  which rows to draw and adds nothing up.
- With no ticket at the table the answer is a bill with nothing on it, not
  an error.
- `upi` in the answer: the link, the address, the payee and the amount.
  The link is the server's, built from the balance. The phone follows it
  only if it begins `upi://pay?`.

## Interactions

| Action | Does | Then |
|---|---|---|
| See the table's bill, on the order placed once it is sent | Opens this screen | |
| See the table's bill, on an empty cart | Opens this screen | A table staff seated, whose guest ordered nothing by phone, can still read it |
| Back, the phone's Back | The menu | |
| Try again | Reads the bill again | |
| Pay by UPI | Hands the link to the phone's UPI app | This page learns nothing of it. The bill changes when staff record the payment, at the next read |

## States

- **Reading:** "Reading the bill".
- **Not reached:** "Can't reach the kitchen to read the bill. Check your
  connection and try again." with **Try again**.
- **Failed:** "The bill could not be read. Try again, or ask a member of
  staff." with **Try again**.
- **Nothing open:** no lines and no summary, and the sentence above. A
  request nobody has accepted is not on any bill.
- **The bill closed while it was shown:** at the next read it is "nothing
  open". Task 5's receipt is what a guest keeps.

## Constraints

- No figure on this screen is worked out on the phone.
- Money is integer minor units, and a rate is words from an integer, never
  a float.
- **Paying by UPI does not mark the bill paid, and the screen says so.**
  UPI tells the venue's server nothing; a member of staff records the
  payment on the terminal as they would one made at the counter.
- A phone with no UPI app does nothing on the tap: the address is there to
  read, and the counter is named under it.
- One guest sees the whole table's bill. The table's code is the table's,
  a paper bill would show the same, and nothing records whose phone
  ordered which dish.
- The guest's bundle is 39.1 KB gzipped of 60.

## Reference

`mez Mobile Customer.dc.html`, M08 Request Bill, Split, UPI (lines 389 to
436).

**Left out:**
- The QR, for paying from another phone: 4e3. Card and wallet: 4f.
- "Waiting for payment" and a bill that turns paid by itself: that needs a
  gateway (4f). Until then staff confirm.
- **Split the bill**, evenly or by item: 4g.
- The bill's number: an open bill has none; an invoice number comes with
  the payment that settles it.
- "Request bill", a call to staff: there is no call-waiter yet (M07).

**Changed:**
- "Items total" is "Items", and "Bill total" is kept.
- Amounts in full, with paise, as the invoice will show them.

**Kept that the design does not draw:** each line of the bill, On the
house, the discount row, tax that is already in the prices, Paid and Left
to pay, and the state with nothing open.

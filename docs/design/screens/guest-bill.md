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
- **Pay online,** a card, first, only at a venue with a payment gateway and
  with something owed (ADR-222):
  - **Pay ₹540.00 online**, the primary button, 58px. It starts a charge
    and leaves for the payment provider's own page. While that is being
    asked for it reads "Opening the payment page".
  - "Card, UPI and more, on the payment provider's own page. This bill is
    marked paid by itself."
  - **How a payment this phone started stands,** over the button, in the
    state's tone, with the amount, announced when it changes:
    - waiting, in the warning tone: "Waiting for your payment. If you have
      paid, this will say so in a moment. If you closed the payment page,
      you can open it again.", with **Open the payment page again**. The
      button is not offered while one waits.
    - paid, in the green tint: "Payment received. It is on this bill. Thank
      you."
    - failed, in the danger tint: "That payment did not go through. Nothing
      was taken. Try again, or pay a member of staff."
    - taken and not on the bill, in the danger tint: "Your payment was
      taken, and is not on this bill. Please show this to a member of
      staff. They will put it right."
    Once answered it has **OK**, which puts it away.
  - Why it could not be started, in danger, under the button.
- **Pay by UPI,** a card, only when the venue has given its UPI address,
  the bill is in rupees and something is owed (ADR-215):
  - **Pay ₹540.00 by UPI**, a link drawn as the primary button, 58px, or as
    the secondary where Pay online is offered above it, for exactly what is
    left. It opens the phone's UPI app with the venue's
    address and the amount filled in.
  - "To Demo Cafe, democafe@okbank", small, so the guest can check who is
    being paid, or type it in.
  - "After paying, show the payment to a member of staff. This bill is
    marked paid once they have seen it."
  - **I have paid**, a secondary button, full width (ADR-218). It tells
    staff to look for the payment and pays nothing. While it is being sent
    it reads "Telling staff". Once staff are told, the button and the
    sentence above it give way to one line in the green tint: "Staff have
    been told you paid. This bill is marked paid once they have seen the
    payment."
  - **Pay from another phone**, closed until tapped (ADR-217): the same
    payment as a QR code, 232px, black on white with its quiet band, and
    "Scan with any UPI app. It pays ₹540.00, what is left on this bill
    now."
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
- `online` on the bill: whether what is left can be paid online.
  `POST /order/api/t/{code}/bill/charge` starts a charge and answers the
  page to pay it at; `GET .../bill/charge/{id}` answers how it stands, as
  the venue's server found out from the gateway. The phone keeps which
  charge it started under `mez.charge.<code>`, for 45 minutes, and nothing
  else about it. It asks every 4 seconds while the charge is open and the
  bill is shown, and reads the bill again at an answer.
- The phone follows the payment page's address only if it is `https`.
- `POST /order/api/t/{code}/bill/paid`, with no body, for I have paid.
  Whether staff have been told is `paid_told` on the bill, the server's to
  say: it reads the same after a reload and on every phone at the table.
  The bill is read again after telling, told or refused.
- `GET /order/api/t/{code}/bill/upi.svg`: that link as a QR code, drawn on
  the server. Asked for only when Pay from another phone is opened. The
  amount is in its address so the picture is asked for again when the
  balance moves; the server reads no amount from the phone.

## Interactions

| Action | Does | Then |
|---|---|---|
| See the table's bill, on the order placed once it is sent | Opens this screen | |
| See the table's bill, on an empty cart | Opens this screen | A table staff seated, whose guest ordered nothing by phone, can still read it |
| Back, the phone's Back | The menu | |
| Try again | Reads the bill again | |
| Pay online | Starts a charge and leaves for the provider's page | Back at the table's address, the app opens on the bill and says how the payment stands |
| Open the payment page again | The same page | |
| OK, on an answered payment | Puts it away | Pay online is offered again if anything is owed |
| I have paid | Tells the terminals | "Staff have been told you paid." Tapped on two phones, staff are told once |
| Pay from another phone | Opens, and the code is fetched | Tapped again, it closes |
| Pay by UPI | Hands the link to the phone's UPI app | This page learns nothing of it. The bill changes when staff record the payment, at the next read |

## States

- **Reading:** "Reading the bill".
- **Not reached:** "Can't reach the kitchen to read the bill. Check your
  connection and try again." with **Try again**.
- **Failed:** "The bill could not be read. Try again, or ask a member of
  staff." with **Try again**.
- **Nothing open:** no lines and no summary, and the sentence above. A
  request nobody has accepted is not on any bill.
- **Paying online could not be started:** the server's sentence when the
  gateway is not working ("Paying online can't be reached right now. Try
  again, or pay a member of staff."), or one that names staff or the
  counter.
- **The phone forgot its charge** (storage refused, or another phone paid):
  the bill still turns paid at its next read. The venue's own sweep puts
  the payment on the bill whether or not any phone asks.
- **Staff could not be told:** under the button, "Can't reach the kitchen
  to tell staff. Show them the payment yourself.", or "Staff could not be
  told from here. Show them the payment yourself.", or, when the bill was
  settled meanwhile, "This bill has nothing left to pay by UPI."
- **Staff looked and found no payment:** the green line goes and the button
  is back. Nothing tells the guest why: staff do, at the table.
- **The bill closed while it was shown:** at the next read it is "nothing
  open". Task 5's receipt is what a guest keeps.

## Constraints

- No figure on this screen is worked out on the phone.
- Money is integer minor units, and a rate is words from an integer, never
  a float.
- **Whether an online payment was made is never the phone's to say.** What
  a gateway adds to the address it sends a guest back to is not read.
- **One primary on the screen:** Pay online where it is offered, else Pay
  by UPI.
- **Saying "I have paid" does not mark the bill paid either.** No figure
  on the bill moves until a member of staff records the payment.
- **Paying by UPI does not mark the bill paid, and the screen says so.**
  UPI tells the venue's server nothing; a member of staff records the
  payment on the terminal as they would one made at the counter.
- **The code is closed until asked for.** A code left on a screen on the
  table is one the next table can scan, and it pays this bill.
- The code's picture has a description for a screen reader, with the
  amount and the payee.
- A phone with no UPI app does nothing on the tap: the address is there to
  read, and the counter is named under it.
- One guest sees the whole table's bill. The table's code is the table's,
  a paper bill would show the same, and nothing records whose phone
  ordered which dish.
- The guest's bundle is 40.7 KB gzipped of 60. The QR encoder is on the
  server and costs it nothing.

## Reference

`mez Mobile Customer.dc.html`, M08 Request Bill, Split, UPI (lines 389 to
436).

**Left out:**
- A choice of card, UPI or wallet on this screen: the provider's page asks.
- A bill that turns paid by itself needs a gateway; without one, staff
  confirm a UPI payment.
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

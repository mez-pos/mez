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
  - **Split the bill** (ADR-235), a heading in the serif and three choices
    in a row, 48px, one chosen, in the green tint:
    - **All of it**, the choice to begin with: what is left on the bill. It
      reads **The rest** once some of the bill is paid, or being paid by
      someone else at the table.
    - **Split evenly**: under the row, minus and plus either side of "Among
      3 people", from 2 to 20.
    - **An amount**: under the row, "The amount you are paying, up to
      ₹540.00" over a field for figures, right aligned in the mono.
    Drawn only where this phone can take a share: it has a name, and
    something is left that nobody else is paying.
  - **Pay ₹540.00 online**, the primary button, 58px, with the figure of
    the choice above it: the rest, the part, or what was typed. It starts a
    charge and leaves for the payment provider's own page. While that is
    being asked for it reads "Opening the payment page".
  - In the button's place, when the choice asks for nothing that can be
    paid: "Type the amount you are paying.", "That is not an amount. Type
    it in figures, like 250 or 250.50.", "That is more than the ₹540.00
    left to pay.", or "Others at the table are paying the rest of this
    bill."
  - "Card, UPI and more, on the payment provider's own page. This bill is
    marked paid by itself."
  - **How a payment this phone started stands,** over the button, in the
    state's tone, with the amount, announced when it changes:
    - waiting, in the warning tone: "Waiting for your payment. If you have
      paid, this will say so in a moment. If you closed the payment page,
      you can open it again.", with **Open the payment page again** and
      **I am not paying this**, a secondary button, full width (ADR-234).
      Neither the choices nor Pay is offered while one waits.
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
- **See your receipt,** a button, 58px, once the bill is paid or there is
  none open, and the phone remembers a bill (ADR-241). The primary where
  nothing else on the screen is; the secondary under Pay.
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
- `bill` in the answer, while a bill is open: its own id. The phone keeps
  the latest under `mez.bill.<code>`, for the receipt (`guest-receipt.md`).
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
- `split` on the bill, wherever `online` is: `rest`, what Pay asks for
  with no share; `unclaimed`, the most this phone can take; and `parts`,
  an equal part for each number of people. The bill is asked for with
  `?phone=`, the name this phone gave itself, so its own open payments do
  not count against it. Every figure on the choices is one of these.
- The phone's name is made once from random bytes and kept under
  `mez.phone`. It says nothing of who holds the phone. With storage refused
  it lasts until the page goes; where no random name can be made, the phone
  pays the rest and is offered no split.
- A share is the body of `POST .../bill/charge`: the phone's name, and
  `people` or `amount`. The rest is the same request with no body. For
  `people` the server works the figure out again, and the charge's own
  amount is what the waiting line says.
- `POST .../bill/charge/{id}/let-go` gives a waiting payment's page up. The
  answer is the charge as it stands. Given up and not paid (`let_go`, and
  still open), the phone forgets it and reads the bill again; paid after
  all, it says so. A charge another phone gave up is forgotten the same
  way, at the next ask.
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
| See your receipt | The receipt | Of the bill this phone last saw open at this table |
| Pay online | Starts a charge and leaves for the provider's page | Back at the table's address, the app opens on the bill and says how the payment stands |
| Open the payment page again | The same page | |
| OK, on an answered payment | Puts it away | Pay online is offered again if anything is owed |
| All of it, Split evenly, An amount | Chooses what Pay asks for | The figure on Pay changes. Nothing is sent |
| Minus, plus | One person fewer or more | Stops at 2 and at 20 |
| I am not paying this | Gives the page up | The choices and Pay are back. That part of the bill is free for the table |
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
- **Others are paying that part** (two phones chose at once): under Pay,
  "Others at the table are paying that part of the bill. Look at what is
  left, and choose again.", and the bill is read again.
- **The bill moved while paying started:** "The bill changed just now.
  Look at it again, then pay."
- **The page could not be given up:** "That could not be done just now. Try
  again, or pay a member of staff."
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
  open", and See your receipt is under it.

## Constraints

- No figure on this screen is worked out on the phone.
- Money is integer minor units, and a rate is words from an integer, never
  a float.
- **Whether an online payment was made is never the phone's to say.** What
  a gateway adds to the address it sends a guest back to is not read.
- **One primary on the screen:** Pay online where it is offered, else Pay
  by UPI.
- **A share is never worked out on the phone.** The phone reads what a
  guest typed and compares it with `unclaimed`; every other figure is the
  server's.
- **A share is paid online only.** Pay by UPI is for all that is left: a
  UPI link tells the venue nothing, so it cannot be held as anyone's part.
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
- The guest's bundle is 43.3 KB gzipped of 60. The QR encoder is on the
  server and costs it nothing.

## Reference

`mez Mobile Customer.dc.html`, M08 Request Bill, Split, UPI (lines 389 to
436).

**Left out:**
- A choice of card, UPI or wallet on this screen: the provider's page asks.
- A bill that turns paid by itself needs a gateway; without one, staff
  confirm a UPI payment.
- **Split by item**: nothing records whose phone ordered which dish
  (ADR-232).
- "Split 2" and "Split 3" with their figures on the choices: any number
  from 2 to 20 is chosen with the stepper, and the figure is on Pay.
- The bill's number: an open bill has none; an invoice number comes with
  the payment that settles it.
- "Request bill", a call to staff: there is no call-waiter yet (M07).

**Changed:**
- "Items total" is "Items", and "Bill total" is kept.
- Amounts in full, with paise, as the invoice will show them.

**Kept that the design does not draw:** each line of the bill, On the
house, the discount row, tax that is already in the prices, Paid and Left
to pay, and the state with nothing open.

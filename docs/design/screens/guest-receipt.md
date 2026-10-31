# Guest: the receipt

A settled bill, for a guest to keep. Built in phase 11 Task 5e1 (ADR-240,
ADR-241). `web/guest/src/`: `Receipt.tsx`, `receipt-logic.ts`,
`use-receipt.ts`. The hash `#receipt` on a table's path.

## Purpose

Give a guest the invoice the venue issued for their table: the same number
the counter would print, the venue's name and tax registration, what was
had, the tax, and how it was paid. It is the venue's document, shown, and
not a second working-out of the bill.

## Layout

- **The bar,** stuck to the top: back to the bill, "Your receipt" in the
  serif, and the table at the right.
- **The invoice,** a card, centred: the venue's name in the serif; its legal
  name under it, small, when it is another name; "GSTIN" ("VAT number";
  "Tax registration" under a profile with no name for it, as the invoice's
  profile names it, GST-15) and the number in the mono. Then, under a hairline, two rows: "Invoice" and its
  number in the mono; "Issued" and when, as "Tue, 6 Oct 2026 · 1:17 PM".
- **The lines,** a card: each line's count and dish, its choices in small
  print, and what it comes to. Only what was charged: a dish given away or
  voided is on no invoice.
- **The summary,** a card:
  - "Items (8)" and what the dishes come to
  - "Discount" with a minus, only when something was taken off
  - each tax by name and rate, or "Includes CGST (2.5%)", smaller and
    muted, when the prices already hold it
  - a hairline, then "Total" and the figure in the serif
  - how it was paid, a row for each payment: "Paid by cash", "Paid by
    card", "Paid by UPI", "Paid another way", and the amount
  - "Taken off since, by credit note" and "Refunded", each with a minus,
    only when the venue has since done either
- **At the foot,** small and muted: "This is the invoice Dev Cafe issued for
  your table. To keep it, take a screenshot: this page is here for a few
  weeks."

## Data

- `GET /order/api/t/{code}/receipts/{bill}`, key
  `["guest", "receipt", code, bill]`: by the table's code and the bill's own
  id together. Read when the screen opens and when the phone comes back.
  Never shown from an earlier look: a credit note can change it.
- **Which bill** is what the phone remembers under `mez.bill.<code>`: the
  id the open bill named (`bill` on `GET .../bill`), or the one an order it
  placed was on (`bill` on `GET .../orders/{round}`), whichever it saw
  last, and when. Forgotten after 45 days, which is as long as the venue
  gives a receipt. What is read back is not trusted: an id that is not a
  bill's is nothing.
- The time is the invoice's, said in the venue's own zone (`time_zone`) and
  not the phone's.
- Every figure is the invoice's own, from its snapshot. The phone chooses
  rows and words.

## Interactions

| Action | Does | Then |
|---|---|---|
| See your receipt, on the bill | Opens this screen | |
| Back, the phone's Back | The bill | |
| Try again | Reads the receipt again | |

## States

- **Reading:** "Reading your receipt".
- **None to give** (the bill is not settled, is not this table's, is older
  than the venue gives, or the phone remembers none): "There is no receipt
  for this bill here. A bill has one once it is paid in full. Ask a member
  of staff for a copy." No Try again: asking again changes nothing.
- **Not reached:** "Can't reach the kitchen to read your receipt. Check
  your connection and try again." with **Try again**.
- **Failed:** "Your receipt could not be read. Try again, or ask a member
  of staff for a copy." with **Try again**.
- **Storage refused:** the bill is remembered as long as the page.

## Constraints

- No figure on this screen is worked out on the phone.
- Nothing of the till is shown, because none is sent: no payment's
  reference, no member of staff, no line or table id.
- Money is integer minor units, and a rate is words from an integer.
- The invoice number is shown whole and may wrap; it is never shortened.
- The guest's bundle is 45.3 KB gzipped of 60.

## Reference

`mez Mobile Customer.dc.html`, M09 Success, Rating (lines 437 to 478). Its
"Get receipt" is this screen.

**Left out:**
- "Payment successful!" and the amount as a screen of their own: the bill
  already says a payment was received (`guest-bill.md`).
- The rating, the tags and "Submit feedback": nothing in the venue takes
  feedback.
- "Save favourites": M10, and no guest has an account.
- Sending the receipt by email or message: mez asks a guest for no address.
- Printing, or saving a file: a screenshot is what the screen says.
- The credit note's own number and lines.
- A tip.

**Kept that the design does not draw:** the invoice number and when it was
issued, the venue's tax registration, each line, the tax, how it was paid,
and every state with no receipt.

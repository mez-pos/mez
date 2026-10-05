# Terminal: Orders, and refunds

The trading day's orders in one table, open and settled together, with the one
picked down the right: where a bill is found again, to give money back, fix a
payment, or print another copy. Built in phase 07 (ADR-083, ADR-084) and
rebuilt from D09 in the rebuild's Task 10 (ADR-160 to ADR-162).
`web/terminal/src/bills/`: `OrdersScreen.tsx`, `OrderPanel.tsx`,
`RefundSheet.tsx`, `BillSheets.tsx`, `orders-logic.ts`, `bills-logic.ts`,
`use-bills.ts`. At `/orders`, and `/orders/:id` with one picked.

## Purpose

A guest back at the counter with a receipt, a cold dish or a card charged
twice gets the right answer in under a minute, with a second person's PIN,
and leaves with a printed credit note.

## Layout

Inside the frame (`terminal-shell.md`). Reached from **Order History** on
Tables, **Past bills** under the order screen's "...", and a waiter's
**Orders** tile.

- **The list, the rest of the width:**
  - **Orders** in the serif, and under it "86 orders today · ₹24,580.00
    collected". At the right, **Print queue**, in danger with "(2 not
    printed)" while anything has failed, and **New Order** (terracotta).
  - **The tabs**, as pills with their counts: **All**, **Open**, **Paid**,
    **Refunded**, **Voided**. At the right, the search, "Search order # or
    table".
  - **The table**, in a card that scrolls: Order #, Time, Table, Type, Items,
    Amount, Status, and **View**. A row is one button, 56px; the picked one
    has the green tint. The number is the invoice's once there is one.
  - **Before today**, under the day's rows, when a search finds settled
    bills from earlier.
- **The order, 384px, 440px from 1536px, chrome.** With none picked: "Pick an
  order to see it."
  - **The head:** the invoice number, or the order's id, in the serif; where
    and when; and its status chip.
  - Order (its id, once there is an invoice), Guests, and **Payment**, the ways it
    was paid ("Cash + Card"), or "Not paid yet".
  - **Items:** each with its quantity, name, choices, "Voided" or "On the
    house", "1 refunded" in danger, and what it comes to.
  - Subtotal, a row for each tax, Total, Credited and Refunded in danger,
    Tip, then **Paid** or **Amount due** as the one total.
  - **Payments:** each with its method, reference, a chip when it is not
    plainly taken, its amount, and what has gone back on it.
  - **Credit notes:** each with its number, reason and amount.
  - **The foot:** for an order still open, **Open order** and **Pay**; for a
    reopened bill that owes, **Take the balance**; then **Reprint receipt**,
    **Invoice** and **History**, and **Refund** and **Void** as the danger
    pair.

**The four statuses**, a colour and a word each:

| Status | Tone | Word |
|---|---|---|
| open | warning (mustard) | Open |
| paid | success (green) | Paid |
| refunded | danger | Refunded |
| voided | neutral, struck | Voided |

## The refund modal (D09)

A modal, "Refund INV/26-27/000042?", in two steps like every approval.

- **What goes back**, three choices, each a radio row with the server's
  figure at its right:
  - **Partial refund, selected items:** each line with anything left, on a
    stepper from 0 to what is left on it.
  - **Full refund:** "Everything on this bill not yet credited".
  - **An amount:** "A part of the bill, not tied to an item", keyed on the
    pad.
- **The credit note's figures**, from the server's preview: each line, each
  tax, and the total.
- **Back to:** the payments with anything left, one to choose and what each
  has left; with one payment it is named, not asked.
- **Reason (required):** Dish was wrong, Cold or late, Guest changed their
  mind, Overcharged, Goodwill.
- "Refunds appear on the Z-report and in the manager's audit trail."
- Then who approves, their PIN, and **Refund ₹169.00**.

## Sheets

- **Void a payment:** "Void the card payment", "The money was never taken",
  a reason (Card declined after, Charged twice, UPI reversed, Wrong method
  recorded), then the approval. On a closed bill: "The bill reopens so the
  balance can be paid again. Its invoice keeps its number." With several
  payments, **Void which payment?** asks first.
- **Reprint receipt:** a reason (Guest asked for a copy, Printer jammed,
  Receipt lost, For the accounts), then the approval.
- **Invoice:** the invoice as issued (`terminal-tender.md`).
- **History:** the order's events (`terminal-register.md`).
- **Print queue:** `terminal-print-queue.md`.

## Data

- `GET /api/v1/orders/today`, fresh for 15 seconds and read again on any
  order event: the trading day's orders (id, number, opened, table, service,
  items, total, status) and `collected`. The tabs are counted and the rows
  filtered here.
- `GET /api/v1/orders/closed?q=`, only while something is searched: settled
  bills whose invoice or credit note number is `q`, or whose invoice's
  number is the digits in `q` ("42" finds INV/26-27/000042 in any year). A
  barcode scanner types into the search: the receipt's barcode is the
  invoice number.
- `GET /api/v1/orders/{id}` for the panel, kept current by the stream. Each
  line carries `credited_qty` and `due` from the server, so the steppers
  never count.
- `POST .../credit-notes/preview`, on every change of what goes back;
  `POST .../credit-notes` with the approval, the chosen payment and the
  preview's total. One `Idempotency-Key` per attempt.
- `POST .../payments/{id}/void` with the approval; a reprint with its
  approval; `GET .../invoices/{seq}`.
- The credit note prints on the receipt printer through the print queue;
  when any of it goes back in cash, the drawer opens first.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| A tab | Narrows the rows by status | | |
| Type or scan in the search | Filters the day's rows by number or table, and asks for earlier bills by number | | "Before today" under the rows |
| Tap a row | Opens it in the panel | | The path is `/orders/:id`; picking another replaces it, so Back leaves the screen |
| New Order | Opens the order screen | | |
| Open order | Brings it to the order screen, off hold if it was saved | `order.resumed` if saved | |
| Pay | Opens payment on it | | Shut for a saved order and one that owes nothing |
| Refund, a choice, a stepper or the pad | Asks for a preview | | The figures redraw |
| Refund, the approval | Issues the credit note | `credit_note.issued`, `refund.issued` | A toast, "Credit note CN/26-27/000001 issued. It is printing."; the panel shows it |
| Void, reason, the approval | Voids the payment, reopening a closed bill | `order.reopened`, `payment.voided` | **Take the balance** |
| Take the balance | Opens payment on the bill | | The balance is paid again; no new invoice |
| Reprint receipt | | `invoice.reprinted` | A DUPLICATE prints |

## States

- **Loading:** "Loading today's orders" under the title; "Loading the order"
  in the panel.
- **Empty:** "No orders yet today."; on a tab, "No orders here yet today.";
  on a search, "No order matches "42"."
- **Error:** the server's words in the table, or in the panel.
- **Nothing left to credit:** Refund shut, "Everything on this bill has been
  credited."
- **No invoice** (a bill closed at nothing): Refund, Reprint and Invoice
  shut, "This bill has no invoice."
- **A bill that owes:** Refund is shut until the balance is taken.
- **The refund is not ready:** "Choose what goes back.", "Choose the payment
  it goes back to.", "Choose a reason.", or "That is more than this payment
  has left. Refund it in parts, one payment each." (ADR-084).
- **Refused:** the server's reason in the modal ("That's more than is left
  to credit").
- **Offline:** the list and the panel stay as last read; nothing is issued.
- **Permission denied:** the approver's PIN is refused in the approval, in
  words; the person signed in needs no more than to reach the screen.

## Constraints

- No arithmetic in the UI: every figure in the refund modal is the
  preview's.
- Refund and Void are the danger button, brick on its tint. The one green
  here is the way on for an order still open, or Take the balance.
- Every number is tabular and right-aligned; every target 48px, a row 56px.
- A refund goes back to one payment per credit note; a larger one is made in
  parts.
- The table holds together to 1366 wide, where it scrolls sideways inside
  its card before a column is dropped.

## Reference

`mez Desktop POS.dc.html`, D09 Orders Refund (line 790). Left out, and why:

- **Filters and Export:** the tabs are the filters there are, and the
  terminal exports nothing (the back office does). Print queue and New Order
  stand in their places.
- **Cashier and Waiter** in the panel: an order names people by an id this
  screen cannot read. **"₹1,200 tendered"** on the Payment line: the row is
  the ways it was paid; the amounts are under Payments.
- **Email to guest:** it does not exist.
- **Search by guest:** an order keeps no guest's name.
- **"The kitchen and the guest are both notified":** neither is. A credit
  note is what is issued.
- **"Return everything including tip":** a tip is not on the bill, so a full
  refund is everything on the bill not yet credited.

Kept, though not drawn (rule 4): An amount, as a third way to refund; which
payment it goes back to; the invoice as issued; History; Void on a payment
with Take the balance; bills from before today, by number.

# Terminal: Past bills

Where a settled bill is found again, to give money back, fix a payment, or
print another copy. Built in phase 07 Task 6b (ADR-083, ADR-084).

## Purpose

A guest back at the counter with a receipt, a cold dish or a card charged
twice gets the right answer in under a minute, with a manager's PIN, and
leaves with a printed credit note.

## Layout

- **Opening it:** **Past bills** in the Tickets sheet, under the open
  tickets.
- **Past bills sheet** (`Sheet`): a search field, "Invoice or credit note
  number", which a barcode scanner types into (the receipt's barcode is
  the invoice number) and which takes the digits alone ("42" finds
  INV/26-27/000042 in any year). Under it, the last 30 closed bills, newest
  first: the invoice number in mono, where ("Table 4", "Counter"), when
  closed in mono, the total, and a "Credited" chip with the amount when
  any of it has been credited. A bill opens with one tap.
- **The bill** takes the register's place, like the tender screen, in two
  columns:
  - **Left:** the invoice preview (`InvoicePreview`, as on Paid in full),
    then the credit notes against it, each with its number, reason and
    amount.
  - **Right rail** (26rem): the order id, **Total**, **Credited** and
    **Refunded** rows, then each payment: method, reference, amount, how
    much has gone back on it, and **Void** for one that has not. At the
    bottom: **Refund** (the one accent), **Reprint**, **History** (the
    order's timeline, as on a ticket), and **Back to the register**.
- **Refund** (`Sheet`, two steps like the approval sheet):
  - **What goes back:** "Items" or "An amount". Items: each line with a
    quantity stepper from 0 up to what is left to credit on it ("2 of 3
    left"). An amount: the pad. Under either, the credit note's figures
    from the server's preview: each credited line, the tax by component,
    and the total, in mono, with "Credit note total" in Instrument Serif.
  - **Back to:** the payments that have anything left, one to choose; the
    whole credit goes back to it.
  - **Why:** a reason from a fixed list (dish was wrong, cold or late,
    guest changed their mind, overcharged, goodwill).
  - **Continue to approval:** a manager, their PIN, and **Refund
    Rs. 126.00**.
- **Void a payment** (the approval sheet): a reason (card declined after,
  charged twice, UPI reversed, wrong method), then a manager and PIN. On a
  closed bill it reopens it in the same command. Then **Take the balance**
  brings the bill to the register, where Charge takes what is now due; the
  invoice keeps its number.

## Data

- `GET /api/v1/orders/closed?q=`: the last 30 closed bills, or those whose
  invoice or credit note number is `q`, or whose invoice's number is the
  digits in `q`. Key `["orders", "closed", q]`.
- `GET /api/v1/orders/{id}` and `GET .../invoices/{seq}` for the bill.
  Each line carries `credited_qty` and `credited` from the server, so the
  steppers never count.
- `POST .../credit-notes/preview`, on every change of what goes back, key
  under the order's; `POST .../credit-notes` with the approval, the chosen
  payment and the preview's total. One `Idempotency-Key` per attempt.
- `POST .../payments/{id}/void` with the approval.
- The credit note prints on the receipt printer through the print queue,
  laid out by `escpos.CreditNote`; when any of it goes back in cash, the
  drawer opens first.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Type or scan a number | Searches | | The matching bill, or "No bill with that number" |
| Tap a bill | Opens it | | The invoice and payments |
| Refund, Items, a stepper | Asks for a preview | | The figures redraw |
| Refund, An amount, the pad | Asks for a preview | | The figures redraw |
| Continue, PIN, Refund | Issues it | `credit_note.issued`, `refund.issued` | "Credit note CN/26-27/000001 issued"; it prints; the bill shows it |
| Void on a payment, reason, PIN | Voids it, reopening a closed bill | `order.reopened`, `payment.voided` | **Take the balance** |
| Take the balance | The bill comes to the register | | Charge takes the balance; no new invoice |
| Reprint | As on Paid in full | `invoice.reprinted` | A DUPLICATE prints |
| History | Opens the order's timeline | | Every event in words, the credit note and refund among them |

## States

- **Nothing left to credit:** Refund disabled, "Everything on this bill
  has been credited."
- **No invoice** (a bill closed at nothing): Refund disabled, "This bill
  has no invoice."
- **Refused:** the server's reason in words in the sheet ("That's more than
  is left to credit", "That payment has less left to refund than this").
- **Offline:** search and the preview say the server cannot be reached;
  nothing is issued.

## Constraints

- No arithmetic in the UI: every figure on the refund sheet is the
  preview's.
- One accent on the bill: Refund. Void is the danger button, outlined.
- Every number mono and right-aligned; every target 44px.

## Reference

The archived design's `s_invoice` block shows reprint and duplicate; it
has no refund or credit note flow. This screen is new.

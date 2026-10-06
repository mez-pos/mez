# Waiter: the bill

Settling a table at the table: split it between the guests, collect each
share, close the table. Built from T05 in the rebuild's Task 14a (ADR-172).
`web/terminal/src/waiter/WaiterBill.tsx`, `BillShares.tsx`, `BillCollect.tsx`,
`bill-logic.ts`; the split panel and the payment rules are the payment
screen's (`terminal-tender.md`). At `/pay/:id` for whoever works from the
waiter's frame. The top bar there has no navigation: the bill carries its own
way back.

## Purpose

Take the money for a table in one place, a guest at a time, and leave the
table ready to clear.

## Layout

One column, 660px, as a card in the middle of the screen. Its head and foot
stay; the middle scrolls.

- **The head, chrome:** a round back arrow, "Settle bill · Table T4" in the
  serif, and the bill's total at the right.
- **Split:**
  - **Split equally**, **By item**, **Custom** as a segmented control, the
    chosen one green.
  - For an equal split: "Split between" a stepper "guests" (1 to 20; it
    starts at 1), and "₹815.00 each" at the right.
  - **A row for each share:** its number in a round mark, its name ("Whole
    bill" for one, else "Guest 2", or "The rest"), a word under it
    (**Awaiting payment**, **Collecting now**, **Paid · UPI**), its amount,
    and **Collect** on one still waiting. A paid row is the green tint with
    a tick; the one being collected has the green border.
  - **Change the split**, once the shares are settled.
- **Collect ₹815.00 from Guest 2:**
  - **UPI**, **Card**, **Cash** as three tiles.
  - **Tip:** None, 5%, 10%, 15% as chips, each with what it comes to. A tip
    over the venue's ceiling says "Over 50%: needs a manager", and the
    payment button opens the approval first, as on the payment screen
    (ADR-261).
  - For UPI and card, a card: "Show the venue's UPI code to Guest 2" or "Key
    this on the card machine", the sum (and the tip in it), "Waiting for you
    to see it paid", a field for the reference, optional, and **Mark as
    paid** (the primary, 56px).
  - For cash: **Cash received**, a field with "Tap an amount, or here to key
    one"; the quick amounts, exact first; the pad once the field is tapped;
    **Change to return**; and **Cash received** as the button.
- **Paid in full**, in place of all that once nothing is owed: the heading in
  the serif, how it was paid and "Invoice INV/26-27/000042", **Change to
  return** when cash is owed back, "The receipt went to the print queue with
  the payment.", and a script line.
- **The foot, chrome:** "Collected so far" and "₹815.00 of ₹2,445.00";
  **Print receipt** once there is an invoice; and **Close table** (the
  primary, 56px; **Close order** for one with no table). Under them, why it
  is shut.

## Data

- The order in front, as on the payment screen.
- `POST /api/v1/orders/{id}/split`: an equal split is the server's live
  answer, asked again as the guests are counted, until a payment begins.
  From then its shares are settled: what is left would divide differently.
- `POST /api/v1/orders/{id}/payments`, with the same amount, tip, reference
  and `Idempotency-Key` rules as the payment screen. A share is paid whole,
  in one payment.
- `POST .../close`; a reprint with its approval.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| The stepper | Counts the guests | | The rows and "each" redraw from the server |
| By item, Custom | Opens the payment screen's split panel over this screen | | **Complete split** brings its shares back as rows |
| Collect on a row | Makes it the share being collected | | Nothing keyed for the last one carries over |
| A method | Chooses it | | Cash starts with nothing keyed |
| A tip chip | Sets the tip | | The sum to ask for moves |
| Mark as paid, Cash received | Sends the payment | `payment.tendered` | A toast, "UPI payment recorded · Guest 2"; the row reads Paid; the next waiting share is collected |
| The last share | The settling payment issues the invoice | `invoice.issued` | Paid in full |
| Print receipt | A reason, then an approver and their PIN | `invoice.reprinted` | A DUPLICATE prints |
| Close table | Closes the order | `order.closed` | A toast, "Table T4 closed"; the tables, where T4 is To Clear |
| Change the split | Lets go of the settled shares | | Split equally over what is left |
| The back arrow | The table's screen, or the tables | | |

## States

- **Working it out:** the rows dim while the server's answer is on its way.
- **Part paid:** By item is shut, "By item is for a bill with nothing paid
  yet."
- **The split went stale:** "The bill changed after it was split, or money
  was taken elsewhere. Split what is left again."
- **A payment not recorded:** the reason, and "Trying again sends the same
  payment, so it cannot be recorded twice."; the button reads **Try again**.
- **Cash not enough:** the payment screen's words, under the field.
- **Close table is shut:** "Collect the whole bill to close the table.", or
  "The guest is owed money back. A manager settles that on the Orders
  screen."
- **A bill already paid:** opens on Paid in full.
- **Offline:** nothing is recorded; what was keyed stays.

## Constraints

- One primary at a time: Mark as paid or Cash received while money is owed,
  then Close table.
- A share's state is a word under its name, never a colour alone.
- Every figure is the server's but the change and a tip's percentage, as on
  the payment screen.
- **Nothing here needs the internet.** The UPI code and the card machine are
  the venue's own; the waiter says when the money has come.
- A toast sits over the foot's buttons for its few seconds.
- With three or more shares the column scrolls to reach Collect.

## Reference

`mez Tablet Waiter.dc.html`, T05 Bill Split Payment (line 425). Left out, and
why:

- **Wallet:** no wallet is a payment method.
- **A QR made for the share, "mez@upi", and "Waiting for confirmation":**
  the design draws a gateway and a sale must finish with none
  (`terminal-tender.md`). Mark as paid is what records it.
- **Print bill before payment:** no bill is printed until it is paid and
  numbered (ADR-078). **Print receipt** comes with the invoice.
- **"8:31 PM" on a paid share:** the row says how it was paid.
- **A keyed tip:** three percentages and None; a custom tip is the payment
  screen's.

Changed: the design has no "Complete split" step, so an equal split is made
here by counting guests and tapping Collect. By item and Custom borrow the
desktop's panel, which does have that step.

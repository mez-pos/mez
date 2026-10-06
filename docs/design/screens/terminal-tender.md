# Terminal: Payment

Where an order becomes money: a method, the amount, a tip kept apart, a split
when the guests pay separately, and the invoice at the end. Built in phase 07
and rebuilt from D07, D08a and D08b in the rebuild's Tasks 8 and 9 (ADR-156,
ADR-159). `web/terminal/src/tender/`: `PayScreen.tsx`, `PaySummary.tsx`,
`PaySteps.tsx`, `AmountCard.tsx`, `TipCard.tsx`, `PayStates.tsx`,
`SplitPanel.tsx`, `SplitShares.tsx`, `SplitParts.tsx`, `InvoicePanel.tsx`,
`InvoicePreview.tsx`, `PaidPanel.tsx`, `tender-logic.ts`, `split-logic.ts`.
At `/pay/:id`.

A waiter's `/pay/:id` is the bill at the table (T05): `waiter-bill.md`.

## Purpose

Take what the guest is paying, by the means they are paying it, and show at a
glance what is still owed and what change to hand back.

## Layout

Inside the frame (`terminal-shell.md`), from **Pay** on the order screen or
**Settle Bill** on Tables. The order is down the left, the work in the
middle, and once the bill is paid a panel on the right.

- **The order, 400px, chrome:** **Back to Order**; where it is in the serif
  ("Table T4") with how it is served as a pill; the guests and the order's
  id; the lines that are not voided, each with its choices, "×2" and what it
  comes to; the totals, tax by tax, as on the order screen; **Payments
  taken**, a row each ("Paid · Card · 4417", and "Tip, apart from the bill"
  under it); then **Add Discount** and **Add order note**, until the bill is
  paid or invoiced. A script line at the foot from 1536px.
- **The steps**, across the top of the middle: **1 Payment**, **2 Split
  Bill**, **3 Complete**. The one in hand is green. Payment and Split Bill
  are buttons; Complete is where a paid bill arrives, never a button.

**Step 1, Payment:**

- **Select payment method:** three tiles, **Cash**, **Card**, **UPI**, the
  chosen one the green tint.
- Under them, cards side by side, three across from 1536px, two with the pad
  under them below that:
  - **Amount:** "Amount to collect" (or "This share") and the figure large;
    a field the pad types into, "Amount received" for cash, "Amount" for card
    and UPI. For cash, the quick amounts: the exact sum, then the next round
    sums above it, and **Custom**; and **Change to return** in a green box.
    For card and UPI, one line: "Collect, take it on the card machine, then
    mark it paid."
  - **Add a tip?:** 5%, 10% and 15%, each with what it comes to, the chosen
    one mustard; **Custom amount**, a field the pad types into; then Bill,
    Tip and **To collect**.
  - **The pad** (`NumPad`, amount layout), typing into whichever field was
    last tapped.
- **The foot, kept in sight:** what is in the way, or "A part payment:
  ₹200.00 stays owed."; and the one primary, 56px: **Complete payment ·
  ₹1,158.00** for cash, **Collect by Card · ₹1,158.00** for card and UPI.

**After Collect, for card and UPI** (D08b), one card in the middle:

- **Waiting:** "Card · Waiting on the card machine" and "Insert or tap card",
  or "UPI · Waiting" and "Scan to pay"; the amount; what to do ("Key this
  amount on the card machine.", and that it has a tip in it); "Waiting for
  payment · 0:24"; a field for the slip reference or the UPI transaction ID,
  optional; **Cancel** and **Mark as paid**.
- **Not recorded:** in the danger tint, the server's reason and "Trying again
  sends the same payment, so it cannot be recorded twice."; **Try another
  method** and **Try again**.
- **Received**, when the payment leaves some owed: "Cash · Confirmed",
  "₹500.00 received", whose share it was and its reference, the tip, the
  change to return, "₹553.00 is still owed on this bill.", and **Next
  payment** or **Next share**.

**Step 2, Split Bill** (D08a), a card filling the middle:

- **The head:** "Split bill · Table T4" and the total, or "Still owed" after
  a payment.
- **Left, the mode:** **By item**, **Equally**, **Custom amounts**.
  - By item: **Guests** on a stepper (2 to 19), then **Unassigned items**, each a
    button that gives it to the guest picked and a **Share** that gives it to
    every guest.
  - Equally: Guests on a stepper (2 to 20).
  - Custom amounts: "Guest 1 pays", the pad and **Add guest**; what is left
    is one more guest, "The rest".
- **Right, a card for each guest:** their number and name, **Pending** or
  **Paid**, their items (an item shared says between how many), and **Due**.
  By item a card is tapped to pick that guest. **Add another guest** after
  the last.
- **The foot:** what is in the way ("Assign every item before completing the
  split."), **Back**, and **Complete split**.
- **A split in use:** the same cards, with **Collect ₹355.00** on each not
  yet paid, the next one green; "1 of 3 paid"; and **Change the split**.
  Collect goes to step 1 for that share alone, which carries a strip saying
  whose share it is and **All shares**.

**Step 3, Complete:**

- **Middle:** the invoice as issued (`InvoicePreview`), a paper slip the
  shape of the 80mm receipt: the seller and registration, "Tax invoice" and
  its number, when and where; each line with its choices and HSN; the taxable
  value, each tax, the total; the payments and tips; how many times it has
  been reprinted. Under it: "The receipt went to the print queue with the
  payment. Print receipt prints a copy marked DUPLICATE and keeps the
  number."
- **Right, 400px, on the sunken ground:** **Payment successful** in the
  serif, the amount paid large, "Paid for ord_..." with where, how served and
  by what; tips, apart; **Change to return** when cash is owed back; a script
  thank-you; **Print receipt**; and **New Order** (terracotta, 56px).

## Data

- The order is the one in front (`useTicket()`), key `["order", id]`, redrawn
  from every command's answer and from the stream. What is owed is
  `totals.balance_due`; payments are `tenders` not voided.
- `POST /api/v1/orders/{id}/payments` `{method, amount, tip, reference?}`,
  through the order's queue.
  - **amount** settles the bill and is never more than the balance. For cash
    it is the lesser of the balance and what was handed over less the tip;
    the rest is change.
  - **tip** is its own field and never part of the amount.
  - **Idempotency-Key:** one per payment. A retry of the one that failed
    sends the same key; changing the method, an amount, the tip or the
    reference makes a new payment with a new key (`attemptKey`).
- `POST /api/v1/orders/{id}/split` `{kind, ways, amounts, items}` answers the
  total and one share per payer. It records nothing. Asked again on every
  change and whenever the order moves; the last answer stays up, dimmed,
  while the next is on its way. Equally and Custom amounts divide the
  balance; By item divides the lines, so it is refused once money is taken
  and the mode is shut then.
- **A share is paid whole, in one payment.** Card and UPI take exactly it;
  cash takes at least it, with change.
- The settling payment issues the invoice in the same transaction and queues
  the receipt, with the drawer kick for cash (ADR-081).
  `GET /api/v1/orders/{id}/invoices/{seq}` for the preview.
- `POST .../invoice` for a bill paid with no invoice; `POST .../close`; a
  reprint with its approval.
- The sums done here, and tested in `tender-logic.ts`: the change, the bill's
  part of a cash payment, a tip as a percentage, to the nearest whole unit
  ("nobody tips in paise"), and the tip ceiling. Every other figure is the
  server's.
- **A tip over the venue's ceiling needs a manager** (ADR-261). The ceiling is
  `tips_max_percent` of the payment's amount, rounded up to the paisa, read
  with the venue (`GET /api/v1/venue`); 0 means every tip. Over it, the tip
  card says "Over 50%: needs a manager" (a warning chip, the colour and the
  words), and the paying button opens the approval sheet ("Approve the tip",
  someone holding `tip.approve` and their PIN) before anything is sent. Cash
  goes with the approval at once. A card or UPI payment carries it to **Mark
  as paid**, so the manager says yes before the machine is keyed; the server
  checks the PIN then, and a refused approval is asked for again. The body
  gains `authorised_by` and `pin`. When the server answers
  `tip_needs_approval` (403), because the till had not read the ceiling or
  it changed, the sheet opens then. The waiter's bill does the same.

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Tap a method | Chooses it; clears the tip and the reference | | Cash starts at nothing; card and UPI at the balance |
| Tap a field | The pad types into it | | |
| Pad keys | Digits shift in from the right | | Card and UPI cannot type past the balance; the first digit starts a following amount afresh |
| A quick amount | Cash received becomes it | | The change shows at once. With a tip, the exact sum is the bill and the tip |
| A tip percentage | Sets the tip; a second tap takes it off | | To collect moves |
| Complete payment (cash) | Sends the payment | `payment.tendered` | Paid in full: step 3. Otherwise the Received card |
| Collect by Card or UPI | Starts the wait; nothing is sent | | The Waiting card |
| Mark as paid | Sends the payment, with the reference | `payment.tendered` | As above. Refused: the Not recorded card |
| Cancel (waiting) | Nothing recorded | | Back to the amount |
| Split Bill | Opens the split, By item, 2 guests (Equally once anything is paid) | | The cards come from the server |
| Complete split | Fixes the shares | | Collect on each card; Guest 1's is next |
| Collect on a card | Step 1 for that share | | Paid, the card says so and the next share is green |
| Change the split | Drops the split | | The whole balance again |
| The balance reaches zero | The settling payment issues the invoice | `invoice.issued` | Step 3 |
| Print receipt | A reason, then an approver and their PIN | `invoice.reprinted` | A DUPLICATE prints; "Reprinted 1 time" |
| New Order | Closes the order | `order.closed` | The order screen, on a new order |
| Add Discount, Add order note | The order screen's sheets | `discount.applied`, `note.added` | The totals redraw |
| Back to Order | Returns to the order screen | | The order shows its payments and balance |

## States

- **Partly paid:** Payments taken lists what came in; Amount to collect is
  what is left.
- **A payment not ready:** the foot says why and the button is shut: "Enter
  the cash handed over.", "The tip is more than the cash handed over.", "A
  share is paid in one go: take at least the share.", "Enter the amount."
- **Sending:** the button says "Sending" and every control waits, so a
  second tap cannot send a second payment.
- **Offline or refused:** cash shows the reason in the foot; card and UPI
  show the Not recorded card. Nothing is recorded, and Try again reuses the
  key.
- **The split went stale** (an item added, a payment taken elsewhere): "The
  bill changed after it was split. Change the split and make it again." and
  no share can be collected.
- **Paid with no invoice:** step 3 offers **Issue the invoice**, and New
  Order waits with "Issue the invoice first."
- **Moved on another device:** the balance redraws from the stream; a card
  amount above the new balance is lowered to it.
- **Permission denied:** a payment needs `payment.take`; a reprint needs an
  approver who may, and so does a tip over the ceiling (`tip.approve`).

## Constraints

- The paying button is 56px. Every other target is 48px or more.
- Every number is tabular and right-aligned, but the large figures: Amount to
  collect, Change to return, and the amount paid.
- One green commit per view: the paying button, Mark as paid, Complete split,
  or the next share's Collect. New Order is terracotta: it starts something,
  it does not take money.
- A tip is never added into the amount the server records. The button names
  the sum the guest hands over, because that is what a card machine is keyed
  with, and the payment row shows the two apart.
- **Nothing here needs the internet.** The card machine and the UPI code are
  the venue's own; the screen waits to be told the money came.
- At 1366x768 the payment step scrolls to reach the pad; the foot does not.
- A split and which shares are paid live on this screen only: a reload loses
  them, and the balance is still right.

## Reference

`mez Desktop POS.dc.html`, D07 Payment (line 574), D08a Split Bill (line 700)
and D08b UPI Card States (line 751). Left out, and why:

- **Wallet:** no wallet is a payment method.
- **A QR made for the bill, "mez@upi", the terminal's name, "Retry
  terminal", and the bank's decline code:** the design draws a payment
  gateway, and a sale must finish with none. Mark as paid is what records a
  card or UPI payment, and a refusal is the server's, not a bank's.
- **Send the receipt by SMS or email:** neither exists.
- **"100% of tips go to the floor and kitchen team":** who shares a tip is
  the venue's rule, not this screen's to promise.
- **"Share of tax" on a guest's card:** a line's figure already has its tax
  in it, as the bill's does.
- **"Table 6 is now marked for cleaning":** the table turns To clear when its
  order closes, and Tables shows it.

Changed: "Payment Successful!" in script is **Payment successful** in the
serif, since a status is never script (tokens.md), with the script on the
thank-you. The tip card's last row is **To collect**, not "Charge".

Kept, though not drawn (rule 4): the pad, which the design leaves to a
keyboard; a part payment and its Received card; the reference field; the
invoice as issued on step 3; the approval behind Print receipt.

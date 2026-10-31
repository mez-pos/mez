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

- **The order, `w-rail`, chrome:** **Back to Order**; where it is in the
  serif ("Table T4") with how it is served as a pill; the guests and the
  order's id; then, scrolling, the lines that are not voided, each with its
  choices, "×2" and what it comes to, and **Add Discount** and **Add order
  note** under them until the bill is paid or invoiced. **Pinned at the
  foot**, as the order rail's are, so the Balance never scrolls away
  (ADR-381): the totals, tax by tax, as on the order screen, and **Payments
  taken**, a row each ("Paid · Card · 4417", and "Tip, apart from the bill"
  under it), which scroll on their own past about five rows.
- **The steps**, across the top of the middle: **1 Payment**, **2 Split
  Bill**, **3 Complete**. The one in hand is green. Payment and Split Bill
  are buttons on the sunken fill; Complete is where a paid bill arrives,
  never a button, so it is a plain label with no fill. A paid bill shows no
  strip: "Paid in full" heads the invoice (ADR-381), so nothing wraps beside
  the paid panel at 1024.

**Step 1, Payment:**

- **Select payment method:** three tiles, **Cash**, **Card**, **UPI**, the
  chosen one the green tint.
- Under them, three cards. From 1536px they sit side by side: the amount,
  the tip (`--rail`) and the pad (`--side`). Below 1536px (ADR-381) the pad
  (`--panel`) sits beside the amount, so an odd sum is keyed with the amount
  to collect in view, and the tip goes under both, as wide as the two, its
  choices and its sums in two columns:
  - **Amount:** "Amount to collect" (or "This share") and the figure large;
    a field the pad types into, "Amount received" for cash, "Amount" for card
    and UPI. For cash, the quick amounts: the exact sum, then the next round
    sums above it, and **Custom**, as many to a row as the card holds; and,
    from 1536px, **Change to return** in a green box. For card and UPI, one
    line: "Collect, take it on the card machine, then mark it paid."
  - **Add a tip?:** 5%, 10% and 15%, each with what it comes to, the chosen
    one mustard; **Custom amount**, a field the pad types into; then Bill,
    Tip and **To collect**.
  - **The pad** (`NumPad`, amount layout), typing into whichever field was
    last tapped.
- **The foot, kept in sight:** once the cash handed over covers what is
  owed, **Change ₹40.00** in the total's serif and the success ink, at
  every size (ADR-381); otherwise what is in the way, or "A part payment:
  ₹200.00 stays owed."; and the one primary, 56px: **Complete payment ·
  ₹1,158.00** for cash, **Collect by Card · ₹1,158.00** for card and UPI.

**At 1024x768 and 1366x768** (`short:`): the step strip is 48px, the
method tiles 56px with the icon beside the word, and the cards' padding
and gaps step down. With cash and a quick amount chosen, the amount to
collect, the quick amounts, the pad, the change in the foot and the paying
button are all in view without a scroll; the tip card starts under them
and is a scroll away. At 1024 the quick amounts take two rows (three,
then two); at 1366 one row.

**After Collect, for card and UPI** (D08b), one card in the middle:

- **Waiting:** "Card · Waiting on the card machine" and "Insert or tap card",
  or "UPI · Waiting" and "Guest pays by UPI" over a phone, not a code: the
  guest scans the venue's own code (TILL-23, ADR-393); the amount; what to do ("Key this
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

- **The head:** "Split bill · Table T4", the mode, and the total, or "Still
  owed" after a payment. The mode is a segmented control (ADR-381): **By
  item**, **Equally**, **Custom amounts**, the chosen one the green tint,
  their icons from 1280px. Below 1280px "Split bill ·" gives way (it is
  still read aloud) so the head stays one row: the step strip above says it.
- **Left, a column (`w-panel`) for what the mode needs:**
  - By item: **Guests** on a stepper (2 to 19), "Tap an item to give it to
    Guest 1, or share it between everyone.", and "Assign every item before
    completing the split." while any is left.
  - Equally: Guests on a stepper (2 to 20).
  - Custom amounts: "Guest 1 pays", the pad and **Add guest**; what is left
    is one more guest, "The rest".
  - Once anything is paid, "By item is for a bill with nothing paid yet."
- **Right, the wide column.** By item it starts with the work (ADR-381): the
  guests as a row of chips, "Guest 1 ₹190.00", the picked one the green
  tint, then **Unassigned items**, each a button that gives it to the guest
  picked and a **Share** that gives it to every guest. Under them, and in
  every mode, **a card for each guest**, as many across as the column holds:
  their number and name, **Pending** or **Paid**, their items (an item
  shared says between how many), and **Due**. By item a card is tapped to
  pick that guest too. **Add another guest** after the last.
- **The foot:** what is in the way, **Back**, and **Complete split**.
- **A split in use:** the mode named in the head as a plain pill; the same
  cards, with **Collect ₹355.00** on each not yet paid, the next one green;
  "1 of 3 paid" and **Change the split** in the left column. Collect goes to
  step 1 for that share alone, which carries a strip saying whose share it
  is and **All shares**.

**At 1024x768 and 1366x768** the split card fills the middle under the
step strip, its head, padding and foot trimmed (`short:`). By item, the
chips and four or more unassigned items show without a scroll; the guests'
cards are under them, one across at 1024 and two at 1366.

**Step 3, Complete:**

- **Middle:** the invoice as issued (`InvoicePreview`), a paper slip the
  shape of the 80mm receipt: the seller and registration, "Tax invoice" and
  its number, when and where; each line with its choices and HSN; the taxable
  value, each tax, the total; the payments and tips; how many times it has
  been reprinted. Under it, what became of the receipt, from the print
  queue: "The receipt printed on Counter.", "...is printing on Counter.",
  "...has not printed yet: Counter did not answer. It tries again by
  itself.", or, in danger, "No receipt printed: this venue has no receipt
  printer. A manager adds one in the back office." (TILL-19, ADR-394); then
  "Print receipt prints a copy marked DUPLICATE and keeps the number."
- **Right, `w-rail`, on the sunken ground:** **Payment successful** in the
  serif's heading ink (ADR-381: terracotta asks for attention a paid bill
  does not need), the amount paid large, "Paid for INV/26-27/000041" (the
  order's name: its invoice number, never its `ord_` id, TILL-8) with where, how
  served and by what; tips, apart; **Change to return** when cash is owed
  back; a script thank-you; a table's lines the kitchen has not seen, as a
  warning with **Send now** (TILL-1); **Print receipt**; and **Next order**
  (terracotta, 56px), New Order until ADR-383. Its gaps step down on a short
  screen.
- **Lines the kitchen has not seen** (TILL-1, ADR-383), over Select payment
  method: on a counter sale with no table a quiet line, "2 items go to the
  kitchen with the payment"; on a table's ticket a warning in the warning
  tint, "2 items have not gone to the kitchen", with **Send now** (48px).

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
  the receipt, with the drawer kick for cash (ADR-081). On a counter sale
  with no table it also sends what is waiting, before the money, and closes
  the order after the invoice (ADR-383). The till keeps that closed order in
  front, with this panel, until Next order or until the screen is left.
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
| A quick amount | Cash received becomes it | | The change shows at once, in the foot. With a tip, the exact sum is the bill and the tip |
| A tip percentage | Sets the tip; a second tap takes it off | | To collect moves |
| Complete payment (cash) | Sends the payment | `payment.tendered` | Paid in full: step 3. Otherwise the Received card |
| Collect by Card or UPI | Starts the wait; nothing is sent | | The Waiting card |
| Mark as paid | Sends the payment, with the reference | `payment.tendered` | As above. Refused: the Not recorded card |
| Cancel (waiting) | Nothing recorded | | Back to the amount |
| Split Bill | Opens the split, By item, 2 guests (Equally once anything is paid) | | The cards come from the server |
| Complete split | Fixes the shares | | Collect on each card; Guest 1's is next |
| Collect on a card | Step 1 for that share | | Paid, the card says so and the next share is green |
| Change the split | Drops the split | | The whole balance again |
| The balance reaches zero | The settling payment issues the invoice; on a counter sale it also sends what waits and closes it (ADR-383) | `invoice.issued`; on a counter sale `ticket.sent`, `order.closed` too | Step 3 |
| Print receipt | A reason, then an approver and their PIN | `invoice.reprinted` | A DUPLICATE prints; "Reprinted 1 time" |
| Next order | Closes a table's ticket; a counter sale is closed already, so it only moves on | `order.closed` for a table's | The order screen, on a new order |
| Send now | Sends a table's waiting lines to their stations | `ticket.sent` | The warning goes |
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
  collect, Change to return (in the foot and the cards), and the amount
  paid.
- No fixed-width column: the order and paid panels take `w-rail`, the pad
  `--panel` (or `--side` from 1536px), the tip `--rail` from 1536px, the
  split's left column `w-panel`; the quick amounts and the guests' cards are
  `auto-fill` grids (tokens.md, ADR-381).
- One green commit per view: the paying button, Mark as paid, Complete split,
  or the next share's Collect. Next order is terracotta: it starts something,
  it does not take money.
- A tip is never added into the amount the server records. The button names
  the sum the guest hands over, because that is what a card machine is keyed
  with, and the payment row shows the two apart.
- **Nothing here needs the internet.** The card machine and the UPI code are
  the venue's own; the screen waits to be told the money came.
- At 1024x768 and 1366x768 the amount to collect, the pad, the change due
  and the paying button are in view together, and the order's Balance is
  pinned in the summary. ADR-381 reverses what this spec accepted before
  (the payment step scrolling to reach the pad at 1366, the change in the
  card alone): the change is in the foot at every size.
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

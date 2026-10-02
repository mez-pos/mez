# Terminal: Tender

Where a ticket becomes money. Built in phase 07 Task 1: cash, card and UPI
tenders against the ticket in front, with a tip taken apart from the bill.
Splits in Task 2. The invoice and closing the ticket are Task 3.

## Purpose

Take what the guest is paying, by the means they are paying it, and show at
a glance what is still owed and what change to hand back.

## Layout

Opened from **Charge** on the register. It takes the register's place inside
`SignedInShell`, in two columns; nothing scrolls but the payments list.

- **Tender column** (flexible):
  - Top: the three **methods**, Cash, Card and UPI, one 96px tile each in a
    row, the chosen one on the raised-high surface with a strong border.
    Each has its name and a hint in muted text ("Drawer", "Slip reference",
    "Transaction ID").
  - Below, two panels side by side:
    - **Amount**: two fields, each one tap target of at least 48px, that
      the pad types into: the amount (for cash, "Cash handed over"; for card
      and UPI, "Amount") and **Tip**. The field the pad is typing into has
      the strong border. Under them, for cash only, the **quick-tender**
      buttons: Exact, then the next round amounts above the balance. Then
      the `NumPad` (`layout="amount"`, `lg`).
    - **Details**: for card and UPI, the **reference** field (a text input,
      16px, optional). For cash, nothing: the drawer does not need one.
- **Summary rail** (26rem, chrome, hairline on the left), the same width
  as the ticket rail it replaces:
  - Top: the metadata trail (`CHARGE · TABLE 4 · 2 COVERS`) and the order
    id in the numeral face.
  - **Total**, then one row per payment already taken: its method, its
    reference in muted text, the amount in the numeral face, and its tip on a second
    row when there is one. Scrolls if there are many.
  - **Amount due** in the serif at `--text-total`: the ticket's
    balance, from the server. It never leaves the screen.
  - **Change due**, cash only, in the serif at `--text-total` in the
    success colour, in a dashed box: what to hand back.
  - Bottom, fixed: the 56px **tender button**, primary, naming what it will
    do and how much: "Take ₹500.00 cash", "Charge ₹450.00 to card",
    "Record ₹450.00 UPI". Under it, **Back to ticket**.

- **Split the bill** (Task 2), over the methods until a split is in use.
  It opens the **split panel** in the tender column; the rail keeps the
  amount due but no tender button, so the panel's **Use this split** is
  the one primary.
  - Top: Evenly, By amount, By item, By guest, one 48px segment each.
  - Left: the choices. Evenly: the number of payers (2 to 20). By amount:
    the amounts keyed so far, each with Remove, and the pad with **Add
    payer**; whatever is left is one more payer, "The rest". By item and by
    guest: the number of payers or guests, then every line that costs
    something with its due and a 48px chip per payer ("Payer 1", several
    may share a line) or per seat ("Table", "Guest 1": one each).
  - Right, "Shares, from the server": one row per share, then "In all"
    and the server's total, so a three-way split of an odd total reads
    down to the paisa (₹410.00 is ₹136.67, ₹136.67, ₹136.66).
- **A split in use**: a bar over the methods naming the split, a 48px
  button per share with its amount (a paid share struck through with a
  "Paid" chip), and **Clear split**. The chosen share is what the next
  tender is for.

## Data

- The ticket is the register's `useTicket()`: `["order", id]`, redrawn from
  every command's answer and from the stream. Amount due is
  `totals.balance_due`; payments are `tenders` that are not voided.
- `POST /api/v1/orders/{id}/payments` `{ method, amount, tip, reference? }`
  (`TenderPayment`), sent through the ticket's ordered queue.
  - **amount** is what settles the bill, never more than the balance: the
    server refuses an over-tender. For cash it is the lesser of the balance
    and what was handed over less the tip; the rest is change.
  - **tip** is its own field and never part of the amount: the amount
    settles the bill, the tip is owed to staff (`PaymentTendered` in
    `internal/domain/order/payloads.go`).
  - **Idempotency-Key**: one per tender attempt. A retry of the same tender
    after a failure sends the same key, so a lost answer can never charge
    twice. Changing the method, an amount, the tip or the reference makes
    it a new tender with a new key.
- `POST /api/v1/orders/{id}/split` (`SplitRequest`: `kind`, `ways`,
  `amounts`, `items` with payers, `seats` with a guest, 0 the table) runs
  `internal/domain/payment` over the order's own lines and answers
  `SplitResponse`: the total and one share per payer. It records nothing
  and takes no key. Evenly and by amount divide the balance, so they work
  after a part payment; by item and by guest divide the lines, which sum
  to the whole bill, so they are refused with `split_after_payment` once
  money is taken. The panel asks again on every change and whenever the
  order moves (its key holds `last_seq`).
- **A share is paid whole, in one tender.** Card and UPI take exactly the
  share; cash takes at least it, with change. So no share is ever part
  paid, and a split needs no ledger of its own.
- Every figure shown is the server's except the change due and the amount
  a cash tender will record, which are the only sums done here
  (`tender-logic.ts`, tested).

## Interactions

| Action | Does | Emits | Then |
|---|---|---|---|
| Charge on the register | Opens the tender screen on Cash | | The pad types into Cash handed over, which starts at zero |
| Tap a method | Chooses it; clears the tip and the reference | | Card and UPI start with the amount at the balance; cash at zero |
| Tap a field | The pad types into it | | |
| Pad keys | Digits shift in from the right (`applyAmountKey`) | | Card and UPI cannot type past the balance; while the amount is following the balance, the first digit starts it afresh |
| Exact | Cash handed over becomes the balance | | Change due is zero |
| A quick-tender amount | Cash handed over becomes it | | Change due shows at once |
| Tender button | Sends the tender | `payment.tendered` | The fields clear; a cash tender leaves its change on screen until the next tender or Back |
| Split the bill | Opens the split panel on Evenly, 2 payers | | The shares come from the server |
| Change the kind, a count, an amount, a line's payer or seat | Asks the server again | | The shares redraw; a refusal shows in words ("Every item needs someone to pay for it.") |
| Use this split | | | The share bar shows; Payer 1's share is chosen |
| Tap a share | Chooses it | | The tender is for that share |
| A share's tender | Pays the share | `payment.tendered` | Struck through, "Paid"; the next unpaid share is chosen |
| Clear split | Back to the whole balance | | |
| Balance reaches zero | The settling tender issues the invoice in the same transaction | `payment.tendered`, `invoice.issued` | The invoice preview in place of the pad; the rail's button becomes **Close ticket** |
| Close ticket | Closes the order | `order.closed` | The register, with a new counter ticket |
| Reprint | A reason (guest copy, printer jammed, receipt lost, for the accounts), then a manager's name and PIN | `invoice.reprinted` | "Reprinted 1 time" on the preview; the number never changes |
| Invoice and close, on a ticket paid before Task 3 | Opens the paid screen with **Issue the invoice** | `invoice.issued` | The preview |
| Back to ticket | Returns to the register | | The ticket shows its payments and balance |
| Refused | | | The server's reason in words under the tender button; the fields stay, and a retry reuses the key |

## States

- **Nothing to charge:** Charge is disabled on the register while the ticket
  has no active line, is held, has lines still Adding, or owes nothing.
- **Partly paid:** the payments list shows what was taken; amount due is
  what is left. Back on the register, the rail shows Paid and Balance under
  the total, and Charge takes the balance.
- **Paid in full:** the tender column shows "Paid in full", **Reprint**,
  and the **invoice preview**: a paper slip (`data-surface="paper"`) the
  shape of the 80mm receipt, drawn from the invoice's own snapshots. Seller,
  registration, "Tax invoice" and the number, when and where issued; each
  line with its modifiers and HSN/SAC; taxable value, each tax component,
  the total; the payments and tips; how many times it has been reprinted.
  The rail keeps the last change due, and its button is **Close ticket**.
  An invoice issued before prices were kept says so instead of the lines.
- **Invoiced:** on the register, Discount is disabled and a line rung up
  is refused with "This bill is invoiced. Close it and start a new
  ticket." Charge reads **Close ticket** (or **Invoice and close** for a
  bill paid before invoices came with the payment).
- **Moved on another till:** the balance redraws from the stream. A card
  amount above the new balance is lowered to it; a cash tender records at
  most the new balance.
- **Offline or refused:** "Not sent: the server can't be reached." under
  the button, nothing recorded; the same key goes with the retry.
- **Split made stale:** when the shares left no longer add up to the
  balance (a line added, a payment taken on another till), the share bar
  says "The bill changed after it was split. Clear the split and split it
  again." and nothing can be tendered against it.
- **Sending:** the tender button says "Sending" and every control is
  disabled, so a second tap cannot send a second tender.

## Constraints

- The tender button is 56px (`size="charge"`). Every other target is 48px
  or more: method tiles, fields, quick-tender buttons, the pad's 64px keys.
- Every number is tabular and right-aligned, except Amount due and Change due,
  which are the screen's totals in the serif.
- One primary: the tender button. Amount due is the heading ink, and
  change due is the success colour with the words "Change due" beside it.
- A tip is never added into the amount the server records. The card and UPI
  button says the sum the guest's card will be charged ("Charge ₹495.00 to
  card", ₹450 and a ₹45 tip) because that is what the card machine is keyed
  with, and the payment row shows the two apart.

## Reference

The layout was first drawn from the Nofiltr archive
(`docs/design/reference/Nofiltr Terminal.dc.html`, block `s_pay`), kept as
history. Its look is now `docs/design/tokens.md` and
`docs/design/reference/mez/mez Desktop POS.dc.html`. Departures from the
first:

- **Tiles are 96px, not 92px** (`h-24`), on the spacing scale.
- **Three methods, not six.** Wallet, Gift card and House tab have no
  backing feature; `other` exists in the domain but has no screen yet.
- **The split panel is a whole column, not a card beside the pad**, and
  has by amount and by guest as well. "By seat" is by guest: lines carry
  no seat until tables arrive (phase 08), so the seat is chosen here.
- **Tip is an amount on the pad, not 5/10/15/20% buttons.** A percentage is
  money arithmetic in the UI; the pad needs none. Percent presets can come
  back once the server offers them.
- **No receipt choice.** The settling payment queues the receipt on the
  receipt printer (ADR-081), with the drawer kick for cash; WhatsApp and
  email have no feature. Reprint queues a copy marked DUPLICATE.
- **"Settle & print" is split in two.** The settling tender issues the
  invoice; Close ticket closes. Printing joins in with the print queue.
- **No "Void bill" here.** Voiding a payment is Task 6's refund and void
  flow, behind a manager.
- **Quick-tender is the next round amounts above the balance**, not fixed
  ₹500 to ₹2000 notes: a ₹180 coffee is paid with a ₹200 note, not ₹500.
- **Amount due is not green.** It is the heading ink; the tender button
  is the screen's one green.

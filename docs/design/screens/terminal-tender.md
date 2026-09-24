# Terminal: Tender

Where a ticket becomes money. Built in phase 07 Task 1: cash, card and UPI
tenders against the ticket in front, with a tip taken apart from the bill.
Splits are Task 2, the invoice and closing the ticket are Task 3.

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
    - **Amount**: two fields, each one tap target of at least 44px, that
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
    id in mono.
  - **Total**, then one row per payment already taken: its method, its
    reference in muted text, the amount in mono, and its tip on a second
    row when there is one. Scrolls if there are many.
  - **Amount due** in Instrument Serif at `--text-total`: the ticket's
    balance, from the server. It never leaves the screen.
  - **Change due**, cash only, in Instrument Serif at `--text-total` in the
    success colour, in a dashed box: what to hand back.
  - Bottom, fixed: the 58px **tender button**, primary, naming what it will
    do and how much: "Take ₹500.00 cash", "Charge ₹450.00 to card",
    "Record ₹450.00 UPI". Under it, **Back to ticket**.

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
| Balance reaches zero | | | "Paid in full" in place of the pad; Back to ticket. The invoice and closing arrive with Task 3 |
| Back to ticket | Returns to the register | | The ticket shows its payments and balance |
| Refused | | | The server's reason in words under the tender button; the fields stay, and a retry reuses the key |

## States

- **Nothing to charge:** Charge is disabled on the register while the ticket
  has no active line, is held, has lines still Adding, or owes nothing.
- **Partly paid:** the payments list shows what was taken; amount due is
  what is left. Back on the register, the rail shows Paid and Balance under
  the total, and Charge takes the balance.
- **Paid in full:** the tender column says "Paid in full" in Instrument
  Serif with the last change due, and Back to ticket.
- **Moved on another till:** the balance redraws from the stream. A card
  amount above the new balance is lowered to it; a cash tender records at
  most the new balance.
- **Offline or refused:** "Not sent: the server can't be reached." under
  the button, nothing recorded; the same key goes with the retry.
- **Sending:** the tender button says "Sending" and every control is
  disabled, so a second tap cannot send a second tender.

## Constraints

- The tender button is 58px (`size="charge"`). Every other target is 44px
  or more: method tiles, fields, quick-tender buttons, the pad's 64px keys.
- Every number is mono and right-aligned, except Amount due and Change due,
  which are the screen's totals in Instrument Serif.
- One accent: the tender button. Amount due is the default text colour, and
  change due is the success colour with the words "Change due" beside it.
- A tip is never added into the amount the server records. The card and UPI
  button says the sum the guest's card will be charged ("Charge ₹495.00 to
  card", ₹450 and a ₹45 tip) because that is what the card machine is keyed
  with, and the payment row shows the two apart.

## Reference

`docs/design/reference/Nofiltr Terminal.dc.html`, block `s_pay`, line 184.
Departures:

- **Tiles are 96px, not 92px** (`h-24`), on the spacing scale.
- **Three methods, not six.** Wallet, Gift card and House tab have no
  backing feature; `other` exists in the domain but has no screen yet.
- **No split panel.** Splits are Task 2.
- **Tip is an amount on the pad, not 5/10/15/20% buttons.** A percentage is
  money arithmetic in the UI; the pad needs none. Percent presets can come
  back once the server offers them.
- **No receipt choice.** Printing arrives with Tasks 4 and 5; WhatsApp and
  email have no feature.
- **"Settle & print" is split in two.** This screen records tenders. The
  invoice and print are Task 3 onwards, and the button says what it does.
- **No "Void bill" here.** Voiding a payment is Task 6's refund and void
  flow, behind a manager.
- **Quick-tender is the next round amounts above the balance**, not fixed
  ₹500 to ₹2000 notes: a ₹180 coffee is paid with a ₹200 note, not ₹500.
- **Amount due is not vermilion.** The design shows the ticket id in
  vermilion; one accent per screen means the tender button keeps it.

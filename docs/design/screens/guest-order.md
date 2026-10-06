# Guest: the order placed

How the order a guest placed stands. Built in phase 11 Task 4c (ADR-213).
`web/guest/src/`: `Order.tsx` (the screen and the strip), `order-logic.ts`,
`use-order.ts`. The hash `#order` on a table's path. Task 5's tracking
takes this screen over. Paying first was added in Task 4e5c (ADR-228):
`use-charge.ts` and `charge-logic.ts`, shared with the bill.

## Purpose

Tell a guest who pressed Place Order the truth about it: the venue has not
answered yet, the kitchen has it, or it was not accepted. Never a timer, and
never "being prepared" for an order nobody has accepted.

## Layout

- **The bar,** stuck to the top: "Your order" in the serif, and the table at
  the right.
- **How it stands,** a card in the state's tone, announced when it changes:
  - waiting, in the warning tone: "Waiting for Demo Cafe to accept your
    order. A member of staff will confirm it in a moment. Nothing is being
    made yet."
  - sent, in the green tint: "Your order is with the kitchen. You can order
    more from the menu at any time. You pay when you are ready to leave."
  - declined, in the danger tint: "This order was not accepted. Nothing was
    sent to the kitchen. Please ask a member of staff."
  - unpaid, where guests pay first, in the warning tone: "Pay to send your
    order. Nothing is sent to the kitchen until this order is paid for."
  - sent after paying first, in the green tint: "Paid, and with the kitchen.
    You can order more from the menu at any time." It never says "you pay
    when you are ready to leave".
  - the payment taken and not usable, in the danger tint: "There is a
    problem with your payment. Your order has not been sent to the kitchen.
    Please show this to a member of staff."
- **What was ordered,** a card: each line's count and dish, its choices in
  small print, and what it comes to.
- **The total,** a card: "Total (3 items)", the figure, and "Before any
  taxes and charges, which are on the bill."
- **To pay,** on an unpaid order: under the total, "To pay, with taxes and
  charges" and the server's figure. Then a card with **Pay ₹360.00**, the
  primary, and "Card, UPI and more, on the payment provider's own page. Your
  order is sent as soon as it is paid." Above the button, once this phone
  has started a payment, how it stands, with its amount:
  - open, warning: "Waiting for your payment", and **Open the payment page
    again**. No Pay button: a second payment is money to refund.
  - failed, danger: "That payment did not go through. Nothing was taken, and
    nothing was sent to the kitchen. You can try again." Pay is back.
  - taken and not usable, danger: "Your payment was taken, and something
    went wrong. Please show this to a member of staff. They will put it
    right. Do not pay again." No Pay button.
- **At the foot:** **Order more**, the primary, once sent, with **See the
  table's bill** under it (`guest-bill.md`); **Back to the menu**,
  secondary, while waiting or declined.
- **The strip,** over the menu and the cart while the phone remembers an
  order: "Your order: Waiting to be accepted", "Sent to the kitchen" or "Not
  accepted", in the state's tone, and **View**. The whole strip is the
  button.

## Data

- `POST /order/api/t/{code}/orders` with the cart as dish ids, choice ids
  and counts, and an `Idempotency-Key`. Everything shown here is from its
  answer: the names, the prices and the subtotal are the server's.
- **The key** is drawn the first time a cart is sent and kept with what was
  sent, in `localStorage` under `mez.order-key.<code>`. The same cart sent
  again goes with the same key and is the same order; a cart that changed
  gets another. It is forgotten once the venue has the order.
- **The order placed** is kept under `mez.placed.<code>`: the key, what was
  sent, and the answer. It is forgotten after three hours, and what is read
  back is not trusted. The phone remembers one order, the latest.
- **While waiting,** the same order is sent again with its key every 8
  seconds, and when the phone comes back from a locked screen: the server
  answers with the order as it stands (ADR-211). Not while the page is
  hidden.
- The cart is emptied when the venue has the order, and not before.
- **Paying,** `POST /order/api/t/{code}/orders/{round}/charge` (ADR-227): the
  phone goes to the address it answers, if it is https. The charge it
  started is kept under `mez.round-charge.<code>` with the order's id, for
  45 minutes, and is asked about every 4 seconds while it is open
  (`GET .../bill/charge/{id}`). When it has an answer the order is asked
  about again at once, and reads sent.
- **Back from the gateway** the address is the table's alone: a phone that
  started a payment for the order it remembers opens on this screen.
- An unpaid order asks again every 8 seconds, as a waiting one does, so a
  payment the venue found out about another way still shows.

## Interactions

| Action | Does | Then |
|---|---|---|
| Place Order, on the cart | Sends the cart | This screen, as the venue answered |
| Order more, Back to the menu | The menu | The strip stays over it |
| View, on the strip | This screen | |
| The phone's Back | The cart, now empty, or the menu | |
| Pay | Starts the order's charge | The gateway's page; then this screen |
| Open the payment page again | The gateway's page | |

## States

- **Placing:** the cart's button reads "Placing your order" and is off;
  Clear is hidden. A second tap sends nothing.
- **Not placed:** the cart stays as it was, with why above the button:
  - a dish or its choices moved: the server's own sentence ("Chocolate
    brownie is no longer available. Take it out of your cart to order the
    rest."), and the menu is read again, so the cart takes the dish out and
    says so
  - the venue is shut: "Ordering is closed right now. Ask a member of
    staff."
  - the table's bill is being settled, or it has too many orders waiting:
    the server's sentence, which names a member of staff
  - not reached: "Can't reach the kitchen, so your order was not placed.
    Check your connection and try again."
  - anything else: "Something went wrong and your order was not placed. Try
    again, or order at the counter."
- **Paying could not be started:** under the button. The gateway refusing
  or unreachable: the server's sentence. The order already paid, gone, or
  from a closed shift: said, and the order is asked about again.
- **No order remembered:** `#order` shows the menu.
- **Storage refused:** the order is remembered as long as the page.

## Constraints

- A guest is never told an order is being made before the venue accepted it.
- No price on this screen is worked out on the phone.
- Money is integer minor units, never a float.
- The same order is never sent as two: one key for one cart, and a second
  tap while placing does nothing.
- Whether an order was paid for is the venue's server's to say. Nothing in
  the address a gateway sends a guest back with is read.
- Pay is never offered while a payment may yet land, or has.
- The guest's bundle is 41.7 KB gzipped of 60.

## Reference

`mez Mobile Customer.dc.html`, M04's Place Order (line 245). M05, Order
Tracking, is Task 5 and was not read.

**Left out:**
- "Your order is secure".
- The tip, and the taxes by name: the bill has them (4d).
- **Special instructions.** The kitchen has nowhere to read one yet; the
  cart says "Allergies or a special request? Please tell a member of staff."
- Kitchen stages, a pickup code, and a receipt: Task 5.

**Kept that the design does not draw:** the waiting and declined states
(ADR-209), the strip, and every refusal.

# Guest: ordering to collect

Ordering with no table, by the venue's pickup code, paid for first and
fetched from the counter by a code. Built in phase 11 Task 5f3a (ADR-251), on
the API of Task 5f2 (ADR-250). It is not a set of screens of its own: it is
the landing, the menu, the cart and the order's screen (`guest-landing.md`,
`guest-menu.md`, `guest-cart.md`, `guest-order.md`) told there is no table.
`web/guest/src/`: `route.ts` (`wayFrom`), `use-guest.ts` (`enterCollect`,
`useTable`), `App.tsx`, and a `collect` flag on `Landing`, `Cart`, `Order`,
`OrderStrip` and `Problem`. Path `/order/p/<code>`, "p" where a table's has
"t" (ADR-247).

## Purpose

Let someone at the door, or on their way, order and pay on their phone and
know what to listen for. Never tell them anything about a table, a bill, or
food being brought to them.

## Layout

Each screen is as its own spec draws it, with these differences.

- **The landing:** "Order ahead" where a table's says "You're at", **Order to
  collect** where the table's name is, and under "Welcome to <venue>": "Order
  and pay here. We give you a code, and call it at the counter when your
  order is ready."
- **The menu** and **the order's bar:** "To collect" where the table is named.
- **The cart:** under the total, "Before any taxes and charges, which are
  added before you pay." No "See the table's bill".
- **The order placed** is always held for its payment (ADR-248): it opens on
  "Pay to send your order", with **Pay ₹378.00**, as a table's order paid
  first does.
- **The order sent:**
  - how it stands, with the kitchen's three steps, as a table's: "Paid, and
    with the kitchen", "Your order is being made", "Your order is ready".
    Ready says "Collect it at the counter with your code." A cancelled one
    says "Please ask at the counter."
  - **Your code,** a card under it: "Your code" and the code at the display
    size (44px) in the numeral face, bold, green, letter-spaced. It is read
    at arm's length and said aloud.
  - under it, small and muted: "Your code is called at the counter when your
    order is ready."
  - the total says "Before any taxes and charges, which were in what you
    paid."
  - at the foot: **Order something else**, the primary, and **Order this
    again**. No bill.
- **The strip** over the menu and the cart ends with the code: "Your order:
  Being made · B7K".

## Data

- `GET /order/api/p/{code}`: the venue's name and currency, and `ordering`.
  The menu, the order, its charge and how each stands are asked under the
  same `/order/api/p/{code}`, at the paths the table's routes have, but for
  the charge, which has no bill: `GET .../charge/{id}`.
- **Which API a code is asked under** is said once, when the page opens
  (`enterCollect`), from the letter in the path. A page has one code for its
  life, and a pickup code is never the same string as a table's.
- The cart, the order's key, the order placed and the charge started are
  kept on the phone under the code, as a table's are. **The order's id is all
  that finds the order again**: the venue's code is on a door, and says
  nothing of anyone's order.
- `pickup_code` is on the order's answer once it is sent. Only three or four
  digits and capitals are shown or kept; anything else in storage is not
  trusted, and the order is forgotten.
- **Back from the gateway** the address is `/order/p/<code>` alone: a phone
  that started a payment for the order it remembers opens on the order.
- **An order the venue answers 404 for is not asked about again.** What the
  phone remembers of it, its code with it, stays on the screen.

## Interactions

| Action | Does | Then |
|---|---|---|
| Start Ordering | The menu | |
| Place Order, on the cart | Sends the cart | The order, unpaid, with Pay |
| Pay | Starts the order's charge | The gateway's page; then the order |
| Order something else | The menu | The strip stays over it, with the code |
| Order this again | Adds the order's dishes to the cart | The cart. Nothing is placed |

## States

- **Finding the venue:** "Finding the menu", centred.
- **The code leads nowhere** (404 `pickup_code_unknown`): "This code does not
  lead anywhere. Scan the code again, or order at the counter." No button.
- **The venue takes no orders to collect** (`ordering: false`, or 409
  `pickup_off` from any route): "Not taking orders to collect online. Please
  order at the counter." and **Try again**. An order already placed still
  shows on its own screen.
- **Not placed:** the server's sentence, as for a table. A venue with too
  many orders waiting to be paid for says so and names the counter.
- **`#bill` and `#receipt`:** the menu. There is neither.
- Every other state is the screen's own spec's.

## Constraints

- No sentence on these screens says "table", "bill" or "bring". A test holds
  the order's words to it.
- The code is never the only thing that says an order is ready: the words
  and the steps say it.
- Whether an order was paid for is the venue's server's to say.
- The guest's bundle is 46.3 KB gzipped of 60.

## Reference

`mez Mobile Customer.dc.html` draws no ordering to collect. Not read.

**Left out:**
- A receipt: one is asked by a table's code (ADR-240).
- A name or a phone number for the order: the code is what is called.
- A time to collect at, and a time until it is ready: nothing knows one.
- A sound or a notification when it is ready: the page says so when it is
  looked at.
- A list of this phone's orders: it remembers one, the latest. Ordering
  something else before collecting forgets the first order's code.

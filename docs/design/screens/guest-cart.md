# Guest: the cart

What the guest has chosen and what it comes to. Built in phase 11 Task 3c
(ADR-208). `web/guest/src/`: `Cart.tsx`, `cart-logic.ts`, `use-cart.ts`,
`Notice.tsx`. The hash `#cart` on a table's path. It is the phone's own
list: nothing here is sent anywhere until Task 4.

## Purpose

Let a guest see their order before it is one: every dish, the choices on
each, how many, and the total.

## Layout

- **The bar,** stuck to the top: back to the menu, "Your cart" in the
  serif, the count in a green-tint pill ("3 items"), and **Clear** at the
  right. The count and Clear are left out of an empty cart.
- **A notice,** when a dish was taken out (see States).
- **Each line,** a card: the diet mark and the dish's name in the serif,
  the line's total at the right; its choices in words under it ("Large,
  Regular sugar"); "₹100.00 each" when there is more than one; and the
  count with − and +.
- **Add more dishes,** a green-tint row back to the menu.
- **The total,** a card: "Total (3 items)", the figure in the serif, and
  "Before any taxes and charges, which are on the bill."
- **How to order,** for now: "To order, show this list to a member of
  staff at Table T4, or order at the counter."

## Data

- The cart is on the phone, in `localStorage` under `mez.cart.<code>`: for
  each line the dish's id, the chosen choices' ids and a count. No names
  and no prices.
- Names and prices are read from the menu each time it is shown, so a
  price that changed while the phone was in a pocket is never shown as it
  was.
- Where storage is refused (a private window) the cart lasts as long as
  the page.

## Interactions

| Action | Does | Then |
|---|---|---|
| + | One more, up to 20 | The line and the total follow |
| − | One fewer | At none the line is taken out: it is the way to remove a dish |
| Clear | Arms: the button reads "Tap again to clear" for four seconds | A second tap empties the cart |
| Add more dishes, Back | The menu | The cart's bar is at its foot |
| OK on a notice | Dismisses it | |

The same dish with the same choices is one line, however many times it is
added; with other choices it is another.

## States

- **Empty** (M11a): "Your cart is empty. Add something delicious and it'll
  show up right here." and **Browse the menu**.
- **A dish can no longer be ordered:** each time the menu is read the cart
  is held against it. A line is taken out when its dish is gone or sold
  out, a choice it was made with is gone, or its choices no longer satisfy
  a group. The guest is told, on the menu and on the cart, in a notice
  that stays until dismissed: "Chocolate brownie is no longer available
  and was taken out of your cart."

## Constraints

- **Nothing on this screen says it sends anything.** There is no Place
  Order until Task 4 builds one.
- Money is integer minor units from the menu to the total, never a float.
- What is in storage is not trusted: anything that is not a line is left
  out, a count is held to 20 and a cart to 50 lines.
- The total is before tax, and says so. The guest's menu carries no tax
  rates; the bill is the venue's to work out.
- A notice does not fade. A toast that fades is how someone finds out at
  the counter.

## Reference

`docs/design/reference/mez/mez Mobile Customer.dc.html`, M04 (lines 194 to
250) and M11a (line 524).

**Left out, all of it Task 4's:**
- **Place Order**, "Your order is secure", the tip, and paying at the
  table or the counter.
- CGST and SGST in the summary: the tax is worked out when the order is.
- The photo thumbnail on each line, and M11a's chef illustration.

**Changed:**
- The summary is one figure and a sentence about tax, where the design
  itemises a bill.
- Where Place Order is, a sentence telling the guest how to order today.
- Clear is asked twice, on the button itself. The design asks nothing.
- − at one removes the line; the design draws no way to remove a dish.

**Kept that the design does not draw:** the notice for a dish that can no
longer be ordered, and "each" under a line of more than one.

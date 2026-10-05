# Guest: a dish, opened

One dish with the choices it comes with. Built in phase 11 Task 3c
(ADR-208). `web/guest/src/ItemSheet.tsx`, on the rules in
`web/shared/modifier-selection.ts`, which are the till's. A sheet over the
menu (`guest-menu.md`); it has no path of its own.

## Purpose

Let a guest make a dish theirs, see what that costs, and add it.

## Layout

A sheet: from the bottom on a phone, at most 85% of the height, the menu
dimmed behind it.

- **The head:** the dish's name in the serif, and a close control.
- **The body,** which scrolls:
  - its photo, 4 by 3, when it has one;
  - its price in the serif;
  - its description, in full;
  - pills, each only when true: **Vegetarian**, **Vegan** or
    **Non-vegetarian**; **Gluten free**; **Popular** in mustard;
  - **each group of choices,** in the dish's order: its name in the serif,
    and at the right what it asks for ("Required", "Optional", "Select
    multiple", "Up to 2", "Required · Choose 2 to 3"), in brick while a
    required group is still unmet. Under it one row for each choice: a
    round mark for a choose-one group and a square one for choose-several,
    the choice's name, and what it adds ("+₹40.00"), takes off ("−₹10.00")
    or nothing when it costs the same. A chosen row is the green tint with
    a green border.
- **The foot,** which stays: how many (1 to 20), and **Add to cart ·
  ₹200.00**, the primary button, with the figure for the dish, its choices
  and the count.

## Data

The dish and its groups, from the menu already read. Each group starts on
its defaults. Nothing is sent.

## Interactions

| Action | Does | Then |
|---|---|---|
| A choice in a choose-one group | Swaps to it | Tapping the chosen one clears it only if the group is optional |
| A choice in a choose-several group | Adds or removes it | When the group is full the others are dimmed until one is let go |
| − and + | Changes how many | The button's figure follows |
| Add to cart | Adds the line and closes | Onto the line with the same dish and choices, if the cart has one |
| Close, the scrim, Escape | Closes | Nothing is added |

## States

- **A required group unmet:** the button is disabled and says what is
  missing: "Choose size".
- **Sold out,** including a dish that sells out while its sheet is open:
  no choices, no count, and "Sold out for now" where the button was.

## Constraints

- A guest can build nothing a cashier could not: the same rules, the same
  file.
- Every row is 48px at least and is a radio or a checkbox to a screen
  reader, whatever it looks like.
- The figure on the button is for showing. The server prices the order
  (Task 4).
- No drag to dismiss: a drag is never the only way to do something.

## Reference

`docs/design/reference/mez/mez Mobile Customer.dc.html`, M03 (lines 142 to
190).

**Left out:**
- **Special instructions.** An order line has no note the kitchen would
  see until Task 4 says how a guest's order is sent. A field that goes
  nowhere is worse than none.
- "Chef's Special", the star rating and the script line over the photo:
  the menu records none of them.
- The cart icon on the button, and the drag handle.

**Changed:**
- **Add to cart**, where the design says "Add to order": nothing is
  ordered by it.
- Choices are full-width rows, one under another, where the design sets
  sizes three across and extras two across: a row holds a long name and a
  price at 48px, and a third of a phone's width does not.
- "Bestseller" is **Popular**, the menu's own word, and sits with the
  other pills.
- Prices in full, as on the menu.

**Kept that the design does not draw:** the button naming the unmet
group, the dimmed rows of a full group, and the sold-out state.

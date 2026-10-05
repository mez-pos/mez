# Guest: the landing, and when the venue cannot be shown

Where the code on a table leads. Built in phase 11 Task 3b (ADR-207).
`web/guest/src/`: `App.tsx`, `Landing.tsx`, `Problem.tsx`, `route.ts`,
`use-guest.ts`. Path `/order/t/<code>` on the venue's wifi, `/t/<code>` or
the same `/order/t/<code>` through the relay. No device and no sign-in.

## Purpose

Tell a guest who has just scanned a sticker that it worked: which table they
are at, and whose. Then one thing to do.

## Layout

One column, at most a phone and a half wide, on parchment.

- **The logo**, centred, and under it the small-caps line "Good food brings
  people together".
- **A card:** "You're at", the table in the serif at the page-title size and
  in green ("Table T4"; a table already named "Window table" is left as it
  is), a hairline, "Welcome to <venue>" in the serif, "The whole menu, at
  your table.", and one script line, "A small place for big moments ♡".
- **See the menu**, the primary button, full width, at the foot of the
  screen where a thumb is.

## Data

- `GET /order/api/t/{code}`, key `["guest", "table", code]`: the venue's name
  and currency and the table's name.
- The menu is asked for as soon as the table is known, so it is there by the
  time the guest taps (`guest-menu.md`).
- The code is the last segment of the path after `/t/`; the screen is the
  hash: none for the landing, `#menu` for the menu. There is no router.

## Interactions

| Action | Does | Then |
|---|---|---|
| See the menu | Sets the hash to `#menu` | The menu, from the top |
| The phone's Back, on the menu | Clears the hash | This screen |

## States

- **Finding the table:** "Finding your table", centred.
- **The code leads nowhere** (404 `table_code_unknown`, or a path with no
  code in it): "This code does not lead to a table. Scan the code on your
  table again, or ask a member of staff." No button: a code is wrong until
  it is scanned again.
- **The venue cannot be reached** (no network, a timeout, the relay's 502,
  503 or 504, or a 429): "Can't reach the kitchen. Check your connection and
  try again. You can always order at the counter." and **Try again**.
- **Anything else:** "Something went wrong. Try again in a moment, or order
  at the counter." and **Try again**.

## Constraints

- Every screen that says something failed names the counter. Nobody leaves
  hungry because a phone could not reach a kitchen.
- One primary button. The script line is garnish and is hidden from a screen
  reader.
- The whole guest app is under 60 KB gzipped (`make build-web` fails over
  it): no router, no icon pack, nothing from `shared/ui`'s index, only the
  files it uses.
- Fonts are `font-display: swap`: the text is there before the faces are.

## Reference

`docs/design/reference/mez/mez Mobile Customer.dc.html`, M01 (lines 36 to
66), and for the states M11d and M11e (lines 560 to 580).

**Left out:**
- The language switch. There is one language.
- The arch-cropped hero photograph. A venue has no hero photograph to show.
- The three claims at the foot (Easy Ordering, Freshly Made, Better Dining).
  They are the venue's to make, not the software's.
- M11d's "Rescan QR" button and "your cart is saved": a page cannot open the
  camera on the sticker, and there is no cart yet.
- M11e's "ask Rohan" and the bell: no call-waiter until M07 is built.

**Changed:**
- "Welcome!" is "Welcome to <venue>": the design shows only mez's name, and a
  guest should see whose table they are at.
- "Start Ordering" is **See the menu**, and "Scan. Order. Relax." is "The
  whole menu, at your table.": until Task 4 a guest can read and not yet
  order. Both go back to the design's words when ordering lands.
- M11d's "Session ended" is "This code does not lead to a table": there are
  no sessions, and that is what happened.

**Kept that the design does not draw:** the "Finding your table" state, and
the plain "Something went wrong".

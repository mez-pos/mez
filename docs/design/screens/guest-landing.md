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

- **The venue's name**, centred, under "Welcome to": the serif at the
  page-title size. It is where mez's logo was: the guest is at the venue's
  table, not mez's (GST-10).
- **A card:** "You're at", the table in the serif at the page-title size and
  in green ("Table T4"; a table already named "Window table" is left as it
  is), a hairline, and "The whole menu, at your table.".
- **Start ordering**, the primary button, full width, at the foot of the
  screen where a thumb is, at least a home bar's inset clear of the edge
  (GST-6). On a screen wider than the column (768 and up: a tablet, a
  laptop) the whole stands in the middle and the button sits under the
  card, not far below it (GST-14).
- **"Ordering by mez"** under the button, small and muted, with the logo at
  16px: the software, said once (GST-10).
- **The tab's title** is the venue's name, once the table is known.

## Data

- `GET /order/api/t/{code}`, key `["guest", "table", code]`: the venue's name
  and currency and the table's name.
- **The phone's session for the sitting** (ADR-262). While the table has no
  ticket open, the landing also answers `session`, a `gst_...` token, unless
  the phone sent one that is still good. The phone keeps it by code
  (`mez.session.<code>`, shape-checked, `session-logic.ts`) and sends it as
  `X-Guest-Session` on every call under the code. At a table with a ticket
  open it gets none: the code alone does not make a phone one of the party,
  and its first order asks staff to let it join (`guest-order.md`). The
  landing and the menu never need a session.
- The menu is asked for as soon as the table is known, so it is there by the
  time the guest taps (`guest-menu.md`).
- The code is the last segment of the path after `/t/`; the screen is the
  hash: none for the landing, `#menu` for the menu. There is no router.

## Interactions

| Action | Does | Then |
|---|---|---|
| Start ordering | Sets the hash to `#menu` | The menu, from the top |
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
- One primary button. No claim is made for the venue: a line like "A small
  place for big moments" is the venue's to say, and it has nowhere to say it
  yet (GST-10).
- The whole guest app is under 60 KB gzipped (`make build-web` fails over
  it): no router, no icon pack, nothing from `shared/ui`'s index, only the
  files it uses.
- Fonts are `font-display: swap`: the text is there before the faces are.
  The guest has its own lighter cuts (`shared/fonts/guest.css`): Fraunces at
  600 alone and DM Sans at one optical size, both preloaded by `index.html`,
  and Archivo, fetched when a count or a bill shows. No script face. They
  count against their own budget, 72 KB, and the build fails over it
  (GST-11, TECH-18, ADR-397).

## Reference

`docs/design/reference/mez/mez Mobile Customer.dc.html`, M01 (lines 36 to
66), and for the states M11d and M11e (lines 560 to 580).

**Left out:**
- The language switch. There is one language.
- The arch-cropped hero photograph. A venue has no hero photograph to show.
- The three claims at the foot (Easy Ordering, Freshly Made, Better Dining).
  They are the venue's to make, not the software's. So are the small-caps
  "Good food brings people together" and the script "A small place for big
  moments ♡", which every venue showed until GST-10 (ADR-397).
- M11d's "Rescan QR" button and "your cart is saved": a page cannot open the
  camera on the sticker, and there is no cart yet.
- M11e's "ask Rohan" and the bell: no call-waiter until M07 is built.

**Changed:**
- "Welcome!" is "Welcome to <venue>": the design shows only mez's name, and a
  guest should see whose table they are at. The venue's name stands where
  the logo was drawn; mez is a line at the foot (GST-10, ADR-397).
- "Scan. Order. Relax." is "The whole menu, at your table." The button was
  "See the menu" until a guest could order (Task 4c), and is the design's
  "Start ordering" since, in sentence case like every button (GST-13).
- M11d's "Session ended" is "This code does not lead to a table": that is
  what happened. A session that has ended (ADR-262) is never a screen: the
  app drops it and reads the landing again, which hands out another where it
  can.

**Kept that the design does not draw:** the "Finding your table" state, and
the plain "Something went wrong".

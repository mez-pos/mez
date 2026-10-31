# Terminal: Open the shift

The first screen of the day for whoever opens: the drawer is counted into a
float and the order screen follows. Rebuilt from D02 in the rebuild's Task 4
(ADR-150, ADR-152). `web/terminal/src/shift/OpenShiftScreen.tsx`,
`ShiftNotOpen.tsx`, `ShiftStage.tsx`, `FloatCount.tsx`, `open-shift-logic.ts`;
carried at `/shift` by `shell/pages.tsx`.

## Purpose

Nobody sells from a drawer nobody counted. Whoever may open counts the float
and starts; whoever may not is told who can.

## Layout

Inside the frame (`terminal-shell.md`), with no shift open every path leads
here. A faint leaf sits behind the screen. Two columns from 1280px, one under
it, the right column dropping below.

**For someone with `shift.open`:**

- **Left, a card:**
  - "Step 1 of 2" in small caps, **Open the shift** in the serif, and "Count
    the cash drawer before the first order. This becomes tonight's opening
    float."
  - When this device's close has just issued a Z-report: a card, "Z-report 4
    issued. It is printing.", with **View the report**.
  - **Denomination count:** a card for each note and coin of the venue's
    currency, largest first (₹500 down to ₹1 for INR), four across from 1280px:
    the face as staff name it ("₹500", not "₹500.00"; a coin worth less than
    the unit keeps its fraction, "$0.25"), a box for how many, and that row's
    value beside it, in full.
  - **The foot**, sticky (ADR-381): the card's last band stays at the bottom
    of the screen while the denominations scroll under it, with a hairline
    above it, and comes to rest at the card's foot. In it:
    - the error line, when the server refuses;
    - **Counted opening float:** the total, as the one total on the screen, in
      a green panel;
    - **Lock**, and **Open shift & start selling** with an open padlock (the
      one primary, 56px).

    The total and the two buttons share one row where they fit (1024 and 1366
    wide); otherwise the buttons wrap under the total, and on a phone each
    takes the full width.
- **Right, 26.25rem:**
  - **Tonight's shift:** Outlet (the venue), Terminal (this device's name),
    and the person signed in against their role.
  - **Before service**, only when something is sold out: "2 items are sold
    out", a sentence naming up to three of them ("Lemon tart, Banana bread and
    1 more are sold out and can't be ordered until back on sale."), and for
    someone with `menu.edit`, **Review the menu**.

**For someone without `shift.open`** (a waiter, by default):

- **Left, a card:** "Shift not open" in small caps, **The shift isn't open
  yet**, "Orders can't be taken until the cash drawer is counted and the shift
  is opened."; **Who can open it**, each person with their initials, name and
  role, or "Ask the owner or a manager." when the server names nobody; "Lock
  this screen and they can sign in."; **Lock**.
- **Right:** Tonight's shift and Before service, as above, without Review the
  menu.

## Data

- `GET /api/v1/shift`, key `["shift"]`: `{shift: null, openers: [...]}` while
  the till is closed. Read every 30 seconds, so a screen left on "not open"
  moves on by itself once someone opens the shift elsewhere.
- `POST /api/v1/shift/open` `{total, denominations}`, with an
  `Idempotency-Key` kept across retries of the same body. Needs `shift.open`.
  Rows with nothing in them are left out; the server adds the rest up again
  and refuses a total that does not match.
- `GET /api/v1/menu`: the currency, and what is sold out now.
- `GET /api/v1/venue`: the outlet's name.

## Interactions

| Action | Does | Then |
|---|---|---|
| Type in a row's box | Sets how many of that note | The row's value and the total redraw. Digits only; anything else typed or pasted is dropped |
| Focus a box | Selects what is in it | Typing replaces it |
| Open shift & start selling | `POST /shift/open` | A toast, "The shift is open with ₹2,000.00.", and the gate goes to the order screen |
| Cancel, Lock | Ends the session | Staff login |
| View the report | Opens `/shift/z/:n` | The Z-report, with Back to here |
| Review the menu | Leaves for the back office's Menu | |

A float of nothing is allowed: some venues start an empty drawer.

## States

- **Loading:** "Checking the shift", from the gate.
- **Error:** the server's refusal in a line over the buttons; what was counted
  stays.
- **Offline:** nothing opens offline. The gate's "Can't reach the mez server
  to check the shift." with Try again, or the refusal line if the server went
  after the screen drew.
- **Opened elsewhere:** the next read of the shift moves this screen on.
- **A currency with no table of notes** (anything but INR, USD, EUR, GBP):
  one box, "Total counted (AED)", and under a figure that does not read as an
  amount, "Type the total as a plain amount, like 1500 or 1500.50."
- **Permission denied:** the second layout above. A cashier opens by default
  (ADR-152); closing stays with `shift.manage`.

## Constraints

- **Blind:** the screen shows what has been counted and nothing else. What
  the last close left in the drawer never reaches it (ADR-094).
- The only arithmetic here is adding up the boxes for display; the server's
  total is the one recorded.
- One primary: Open shift & start selling.
- Every number is tabular and right-aligned; every box and button 48px or
  taller.
- At 768px high the counted float and Open shift stay in view in the sticky
  foot and the denominations scroll under it (ADR-381, which reverses the
  scroll this line accepted, TILL-24). At 1024x768 the card is the screen's
  width and the foot one row; at 1366x768 the card sits beside Tonight's
  shift and the foot is still one row. Under 820px high (`short:`) the
  foot's padding and the green panel's step down; no text changes size.

## Reference

`mez Desktop POS.dc.html`, D02 Shift Open (line 106). Left out because the
server cannot say them: the expected figure from the last close (the count is
blind), a note for the manager, the shift's hours, who else is on the floor,
the low-stock count, and the illustration.

"Step 1 of 2" is the design's: step two is the day itself. The screen for
someone who cannot open is ours; the design draws only the opener's.

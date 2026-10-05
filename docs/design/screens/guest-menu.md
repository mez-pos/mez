# Guest: the menu

Every dish the venue sells, as a guest reads it at the table. Built in phase
11 Task 3b (ADR-207). `web/guest/src/`: `Menu.tsx`, `menu-logic.ts`,
`use-guest.ts`. The hash `#menu` on a table's path (`guest-landing.md`).
Reading only for now: a dish is opened and added in Task 3c.

## Purpose

Let a guest find what they want to eat: by browsing, by category, by a word,
or by what they do not eat.

## Layout

One column, at most a phone and a half wide.

- **The bar,** stuck to the top: the logo, and the table in a terracotta-tint
  pill ("Table T4").
- **A green band:** the script line "Great food, brighter days ♡" and the
  venue's name in small caps.
- **Search,** "Search for dishes".
- **Categories,** a row of pills that scrolls sideways: **All**, then each
  category that has a dish in it. The chosen one is the green fill. Left out
  when the menu has one category.
- **Diets,** a second row: **Veg**, **Vegan**, **Gluten free**, each only if
  some dish is it. They are switches; a chosen one is the green tint.
- **The title** in the serif: the search ('Results for "dosa"') when there
  is one, else the category, else "All dishes"; and the count at the right
  ("18 dishes").
- **The dishes,** two to a row, each a card:
  - its photo, 4 by 3, only when it has one: a dish without a photo has no
    empty frame;
  - **Popular**, a mustard pill, when it is and is not sold out;
  - the diet mark and the name in the serif;
  - the description, two lines at most;
  - "Gluten free" when it is;
  - the price in the serif, in full ("₹60.00"), and **Sold out** beside it
    when it is. A sold-out card is faded.

## Data

- `GET /order/api/t/{code}/menu`, key `["guest", "menu", code]`: categories,
  items (name, price, description, diet, gluten_free, popular, sold_out,
  photo, modifier_groups) and modifier groups. Read again every minute and
  when the phone comes back to the page, since sold out turns on the
  kitchen's clock.
- A photo is `/order/api/photos/{name}`, lazy and decoded off the main
  thread.

## Interactions

| Action | Does | Then |
|---|---|---|
| A category | Narrows to it | The title is its name |
| A diet | Switches it on or off | Dishes that are all of the chosen ones |
| Typing in search | Narrows, 200 ms after the last key | Every word must be in the name or the description, in any order |
| Show everything | Clears the search, the category and the diets | The whole menu |

Sold-out dishes are listed last, in the menu's order otherwise: a guest reads
from the top, and the top should be what they can have. Veg includes vegan. A
dish that does not say what it is is in no diet's list.

## States

- **Loading:** "Loading the menu".
- **Nothing matches:** "Nothing on the menu matches." and **Show
  everything**.
- **No dishes at all:** "The menu is empty for now."
- **Could not be read:** the landing's states, with **Try again**.

## Constraints

- Nothing tapped is under 48px. Chips are pills with a word; a chosen one
  also says so to a screen reader (`aria-pressed`).
- Sold out is a word and a fade, never a fade alone.
- Prices are whole figures as the bill will show them, through `formatMoney`.
  The compact form is for chart axes and is never a price.
- The search field's text is 16px so a phone does not zoom on it.
- No photo is fetched before it is scrolled to.

## Reference

`docs/design/reference/mez/mez Mobile Customer.dc.html`, M02 (lines 71 to
138), and M11c for sold out.

**Left out:**
- **Add** on each card, and the **View Cart** bar: the cart is Task 3c.
- The heart on each photo: favourites (M10) are not built.
- The bottom navigation (Menu, Orders, Offers, Help) and the menu button in
  the bar: those screens do not exist yet. Each arrives with its screen.
- The sort ("Popular") control: the menu's own order is the venue's choice.
- **Spicy:** the menu does not record it.
- The chevron on the table pill: a guest does not change table by tapping.
- Icons on the category pills: a category is the venue's own word and has
  no icon.
- "Fresh ingredients. Great taste." under the title, and "Fresh flavours
  daily" in the band: claims that are the venue's to make. The band shows
  the venue's name there.
- M11c's "back at 8pm" and its suggestions: the guest is told a dish is sold
  out, not until when, which is the kitchen's guess.

**Changed:**
- Prices show their paise ("₹60.00", where the design draws "₹249"): one
  figure on the menu and on the bill.
- A dish with no photo has no frame, where the design draws a photo on every
  card: most venues start with none.
- "GF" is "Gluten free", in words.

**Kept that the design does not draw:** the count beside the title, the
empty and no-match states, and sold-out dishes sinking to the end.

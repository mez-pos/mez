# Back office: Menu

Where the owner keeps what the venue sells: the categories that are the
register's tabs, and the items in each, with price, tax, where each is made
and which course. Built in phase 10 Task 2b over the server of Task 2a
(ADR-106); groups of choices were added in Task 2c (ADR-108). Sold out
and photos are Task 2d.

## Purpose

Change a price, add a dish, retire what is off the menu, and know exactly
what that does to a ticket already open: nothing. The next line rung up
takes the change; every line already on a ticket keeps what it was rung up
at (`docs/02-domain.md`, "Prices and the snapshot rule").

## Layout

Paper, inside the back office shell (`backoffice-shell.md`), section
**Menu**, path `/menu`, `menu.edit`.

- **Header row:** "Menu" in the serif; on the right **Add a
  category** (secondary) and **Add an item** (primary, the one primary).
- **Under it, one line** in `fg-3`: "Changes reach every till at once.
  Tickets already open keep the prices they were rung up at."
- **Two columns** side by side from 1536px wide; below that the categories
  sit above the items, so a laptop keeps the whole table:
  - **Categories**, 280px, a `Card` list in register order. Each row: the
    name, the number of items in the numeral face on the right, and "Retired" (struck,
    muted) when retired. The selected row has the raised fill and a strong
    left rule, as the shell's navigation does. Under the list, for the
    selected category: **Rename**, **Move up**, **Move down**, and
    **Retire** or **Bring back**.
  - **Items** of the selected category, a dense table (`text-sm` rows):
    | Column | Shows |
    |---|---|
    | Item | the name; "Retired" after it, muted, when retired |
    | Price | `Money`, tabular, right-aligned |
    | Tax | the class's name and rate: "Food, 5%" |
    | HSN | tabular; only on a venue whose profile needs one |
    | Made at | the station's name |
    | Course | its number and name: "1 Mains" |
    | | **Edit**, **↑**, **↓** (labelled "Move Latte up" for a screen reader), **Retire** or **Bring back**, 48px |
- A retired category's items still list, under a line: "This category is
  retired: none of its items are on the till."

## The item form (`Modal`, md)

Name; **Category** (a select of active categories); **Price**, typed in
major units ("180" or "180.50"), tabular, the currency's symbol before it;
**Tax class** (select, "Food, 5%"); **HSN or SAC code** (only on a venue
that needs one; hint "4, 6 or 8 digits. Every GST invoice line shows it.");
**Made at** (active stations); **Course**. Save, Cancel.

- **The price notice.** Editing an item and typing a different price shows,
  under the price, in a `Card` on the sunken fill:
  "Tickets already open keep ₹180.00. Lines rung up after you save are
  ₹200.00." It is there before Save, so the owner reads it while deciding,
  not after. A new item has no notice.
- A price the form cannot read ("12.345" in rupees, "abc", below zero) is
  refused under the field before anything is sent: "Type a price like
  180 or 180.50."
- Every other refusal is the server's words under its field.
- A new item starts in the selected category, with the first active
  station, the first course and the first tax class chosen.

## Choices (Task 2c)

A switch under the header, **Items** and **Choices** (`aria-pressed`),
changes what the page shows. On Choices the primary button is **Add a
group of choices**; Add a category and Add an item are hidden.

- **The groups table:** Group (retired struck, with "Retired"), Asks
  ("Pick 1", "Optional, up to 2", "Pick 1 to 3"), Choices (each name, its
  price change in `Money` when not zero, "(to start)" on a default),
  Items (how many offer it, tabular), and **Edit**, **Retire** or **Bring
  back**. No confirm on retire: it is one click back, and the items keep
  offering it when it returns.
- **The group form** (`Modal`, md): Name; **Fewest to pick** (0 makes it
  optional) and **Most to pick**, tabular; **Choices**, one row each: name,
  price change (40, -20, or empty for none), **Picked to start**, arrows,
  **Remove**; **Add a choice**. A refusal about one choice shows on its
  row. On an edit, a line says how many items offer it and that open
  tickets keep what they were rung up with. Removing a choice retires it.
- **The server refuses** a group whose choices cannot meet its bounds:
  more to pick than it offers, more picked to start than may be picked,
  two choices with one name (`internal/domain/menu/group.go`).
- **In the item form**, "Choices offered": the item's groups in choice
  sheet order, each with its rule, arrows and Remove; a retired one stays
  listed, marked; **Offer another** is a select of the active groups not
  yet offered. Saved after the item, as one ordered list.

## Sold out for today (Task 2d1)

- An active item's row has **Sold out** beside Retire; while sold out, its
  name carries "SOLD OUT TODAY" in the warning colour (the time it returns
  on hover) and the button reads **Back on sale**.
- Sold out lasts until the next trading day starts (04:00 in the venue's
  timezone, ADR-109), then the item is on sale again with no edit. Retire
  is the other thing: off the menu until someone brings it back.
- It is a menu edit: every till learns at once through `menu_changed`.

## Photos (Task 2d2)

- **In the item form,** a Photo field: a 96px preview (or "None"),
  **Choose a photo** / **Choose another**, and **Remove**. Nothing is sent
  until Save; the photo is uploaded after the item, like its choices.
- **Made smaller here:** the chosen file is drawn onto a canvas at most
  480px on its longer side (EXIF rotation applied), on white, and sent as
  a JPEG. Under 64px on a side is refused before sending.
- **The server checks again** (ADR-110): JPEG only, at most 512 KB, 64 to
  1024px a side, and it must decode. A refusal shows under the field.
- **In the table,** a 32px thumbnail before each item's name; an empty
  box where there is none, so names line up.
- On a new item, if the photo or the choices fail after the item is
  saved, the form closes and a warning says which, so Save is never
  pressed twice for one item.

## Categories (`Modal`, sm)

Add and Rename: one field, Name. Retire asks first (`Confirm`): "Retire
Breakfast? Its 6 items come off every till. You can bring it back." Retire
on an item needs no confirm: it is one click to bring back.

## Ordering

The arrows (and Move up, Move down for a category) move a row one place and save at once. There is no drag: the
terminal rules forbid drag as the only way to do something. The whole
list is sent; if the menu changed since the screen read it
(`menu_order_stale`), the screen reloads and says "The menu changed on
another screen. It has been reloaded; move it again."

## Data

- `GET /api/v1/menu/editor`, React Query key `["menu-editor"]`,
  invalidated after every edit and on the stream's `menu_changed`, so a
  second back office sees the first's edits.
- The writes of ADR-106. Every one answers only an id; the screen reads the
  editor again.

## States

- **No categories:** "No menu yet. Add a category, then the items in it."
- **A category with no items:** "No items in Coffee yet."
- **Loading / offline / refused:** the words in place of the columns, with
  Try again.
- **Saved:** a toast: "Saved. Tickets already open keep their prices." for
  an item whose price changed; "Saved." otherwise.

## Constraints

- One primary: **Add an item**. Retire is a secondary button, not danger:
  it is undone in one click.
- Every number tabular and right-aligned: prices, counts, HSN, course numbers.
- The price input is 16px (form controls), tabular.

## Reference

Built from `docs/design/components.md` and the paper tokens. The archive
was not consulted.

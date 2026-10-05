# Back office: Menu

Where the owner keeps what the venue sells: the categories, the items in each
with price, tax, where each is made and which course, the groups of choices,
and the stations and courses. Built in phase 10 (ADR-106 to ADR-110, ADR-139)
and rebuilt from D18 in the rebuild's Task 16f (ADR-190).
`web/backoffice/src/menu/`: `MenuScreen.tsx`, `CategoryRail.tsx`,
`ItemList.tsx`, `ItemEditor.tsx`, `ItemGroupsField.tsx`, `ItemPhotoField.tsx`,
`GroupsView.tsx`, `GroupFormModal.tsx`, `SetupView.tsx`, `SetupModals.tsx`,
`browse-logic.ts`, `menu-logic.ts`, `use-menu-editor.ts`. Section **Menu**,
path `/menu`, `menu.edit`, keys `g m`.

## Purpose

Change a price, add a dish, retire what is off the menu, and know exactly
what that does to an order already open: nothing. The next line rung up takes
the change; every line already on an order keeps what it was rung up at
(`docs/02-domain.md`, "Prices and the snapshot rule").

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Menu Management" in the serif, and "Create and manage
  items, categories and their choices. Changes reach every order screen at
  once." At the right, on Items: the search ("Search menu items"), a select
  of what to show (**All items**, Active, Sold out, Retired), and **Add New
  Item** (terracotta).
- **Three views**, as pills under the head: **Items**, **Choices**,
  **Stations and courses**.
- The screen's problem line, for a refusal that belongs to no field.

### Items (D18)

Three cards side by side from 1280px (220px, 300px, the rest); under that
the editor drops below the other two.

- **Categories:** "Categories" in the serif with a round **+**. Then **All
  items**, **Popular**, and each category in order, with its count; a
  retired one says "Retired". Under the list, for the category chosen:
  **Rename**, **Move up**, **Move down**, and **Retire** or **Bring back**.
- **Menu items (68):** the title with its count, and a select for the
  order: **Menu order**, **A–Z**, **Price**. Each row is one button: the
  photo or a plain tile, the name, the price and a chip (**Active**, **Sold
  out**, **Retired**). The open one has the green tint. Its **...** holds
  **Sold out today** or **Back on sale**, **Retire** or **Bring back**, and
  **Move up**, **Move down**. The list scrolls inside itself.
- **The editor**, for the item open: "Edit menu item" or "Add menu item",
  its chip, and beside the title **Sold out today** (or **Back on sale**)
  and **Retire item** in danger (or **Bring back**).
  - **Item name**; **Category**; **Price, INR** ("As the order screen
    charges it, like 180 or 180.50."); **Tax class** ("Food, 5%"); **HSN or
    SAC code** ("4, 6 or 8 digits. Every GST invoice line shows it." on a
    venue that needs one); **Description**, with its length counted.
  - Two toggles: **Choices** (the groups the item offers, in order, with
    arrows and Remove, and **Offer another**) and **Kitchen and tax**
    (**Made at**, a station; **Course**).
  - Beside the fields, or under them: the **photo** (a preview, **Choose a
    photo**, **Remove**); **Tags** (the diet mark on the card, **Gluten
    free**, **Popular**); and **Save changes** or **Add item** (the
    primary).
- **The price notice.** Typing a different price on an item shows, under
  the price, before Save: "Orders already open keep ₹180.00. Lines rung up
  after you save are ₹200.00."

### Choices

- A table of groups: Group, what it asks ("Pick 1", "Optional, up to 2",
  "Pick 1 to 3"), its choices (each name, its price change, "(to start)" on
  a default), how many items offer it, and **Edit**, **Retire** or **Bring
  back**. **Add a group of choices** is in the head.
- **The group form** (a modal): Name; **Fewest to pick** (0 makes it
  optional) and **Most to pick**; one row per choice: name, price change,
  **Picked to start**, arrows, **Remove**; **Add a choice**. On an edit, a
  line says how many items offer it. Removing a choice retires it.
- The server refuses a group whose choices cannot meet its bounds.

### Stations and courses (ADR-139)

- **Stations**, a card: "Where an item is made: each station has its own
  kitchen display, and prints on the printer you set for it under
  Printers." One row per station: its name, its code, how many items are
  made there, then **Rename**, arrows, and **Retire** or **Bring back**.
  **Add a station** asks a Name and a Code ("Short and lowercase, like bar.
  ... It cannot be changed later.").
- **Courses**, the same: "When an item is fired: course 1 first, then the
  table asks for the next." A course's number follows its place.
- **Retire** (a modal): says what it does, and for a row with items asks
  where they go ("Move its 10 items to"). The only row in service cannot be
  retired: "Add another before retiring it." A toast says what moved: "Bar
  retired. 10 items moved to Kitchen. Its printer now serves Kitchen."

## Data

- `GET /api/v1/menu/editor`, key `["menu-editor"]`, read again after every
  edit and on the stream's `menu_changed`, so a second back office sees the
  first's edits.
- The writes of ADR-106, each answering an id; the screen reads the editor
  again. An item's groups and its photo are saved after the item, as a
  second and third request (ADR-108, ADR-110).
- A photo is made smaller here before it is sent: drawn at most 480px on
  its longer side, on white, as a JPEG. The server checks again: JPEG only,
  at most 512 KB, 64 to 1024px a side.
- Sold out lasts until the next trading day starts (ADR-109), then the item
  is on sale again with no edit.

## Interactions

| Action | Does | Then |
|---|---|---|
| A category | Shows its items | The search is a look through every category |
| A row | Opens it in the editor | |
| Add New Item | Opens an empty editor, in the category chosen | The first station, course and tax class are chosen |
| Save changes | Saves the item, then its choices and photo | A toast: "Saved.", or "Saved. Orders already open keep their prices." when the price changed |
| Sold out today, Back on sale | Marks it until the trading day turns | Every order screen learns at once |
| Retire, Bring back (an item) | No confirm: it is one click back | |
| Retire (a category) | Asks first: "Its 6 items come off every order screen. You can bring it back." | |
| Move up, Move down | Moves a row one place and saves at once | Offered on items only when the list is one category, in menu order, unfiltered |

`/menu?item=<id>` opens that item in the editor as soon as the menu is read;
the tax settings link items still without an HSN code this way.

## States

- **Loading:** "Loading the menu".
- **Error:** the server's words, with **Try again**.
- **No items:** "No items yet. Add the first one." in the editor's place;
  with nothing open, "Pick an item to edit it, or add a new one."
- **A retired category:** "This category is retired: none of its items are
  on the order screen."
- **A price the form cannot read** ("12.345", "abc", below zero) is refused
  under the field before anything is sent. Every other refusal is the
  server's words under its field.
- **Part saved:** "Latte is saved, but its photo did not. Set them again and
  save.", so Save is never pressed twice for one item.
- **The menu changed elsewhere** while moving a row: "The menu changed on
  another screen. It has been reloaded; move it again."
- **Reordering is not offered:** "To move an item up or down, show every
  item in menu order."

## Constraints

- One primary: Save in the editor. Add New Item is terracotta. Retire item
  is the danger button in the editor, though it asks nothing: it is undone
  in one click.
- Every number tabular and right-aligned: prices, counts, HSN.
- No drag: the arrows are the way to reorder.
- Opening another item or leaving the screen drops an unsaved edit without
  asking, and an item changed on another screen while it is open is saved
  over.
- The list no longer shows an item's tax class, station, course or HSN:
  they are in the editor.

## Reference

`mez Desktop Management.dc.html`, D18 Menu Management (line 286). Left out,
and why:

- **Cost price:** a dish's cost is its recipe's, from the stock ledger
  (ADR-189).
- **Show in online ordering, and Happy-hour pricing:** neither exists.
- **The Nutrition tab.**
- **Inventory / stock on an item** (track, current stock, low-stock alert):
  stock is by ingredient, on Inventory.
- **Combos:** not in v1.
- **Search with Ctrl + K, and a free-text tag with Add:** the tags are the
  three the order screen draws.

Changed: "Delete item" is **Retire**, since an item with sales is never
deleted. "Modifiers" is **Choices**, and "Advanced" is **Kitchen and tax**.
The design's Status select is the chip with the two buttons beside the
title. "Bestseller" is **Popular**.

Kept, though not drawn (rule 4): the Choices and Stations and courses
views; what can be done to a category, under the list; a row's "...";
Sold out today.

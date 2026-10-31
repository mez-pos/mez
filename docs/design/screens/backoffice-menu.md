# Back office: Menu

Where the owner keeps what the venue sells: the categories, the items in each
with price, tax, where each is made and which course, the groups of choices,
and the stations and courses. Built in phase 10 (ADR-106 to ADR-110, ADR-139)
and rebuilt from D18 in the rebuild's Task 16f (ADR-190).
`web/backoffice/src/menu/`: `MenuScreen.tsx`, `CategoryRail.tsx`,
`ItemList.tsx`, `ItemEditor.tsx`, `ItemGroupsField.tsx`, `ItemPhotoField.tsx`,
`GroupsView.tsx`, `GroupFormModal.tsx`, `SetupView.tsx`, `SetupModals.tsx`,
`SpreadsheetMenu.tsx`, `ImportModal.tsx`, `PricesModal.tsx`, `FileParts.tsx`,
`browse-logic.ts`, `import-logic.ts`, `prices-logic.ts`, `menu-logic.ts`,
`use-menu-editor.ts`. Section **Menu**,
path `/menu`, `menu.edit`, keys `g m`.

## Purpose

Change a price, add a dish, retire what is off the menu, and know exactly
what that does to an order already open: nothing. The next line rung up takes
the change; every line already on an order keeps what it was rung up at
(`docs/02-domain.md`, "Prices and the snapshot rule").

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Menu", the nav's name for it, in the serif (BO-17,
  ADR-404; D18 said "Menu Management"), and "Create and manage
  items, categories and their choices. Changes reach every order screen at
  once." At the right, on Items: the search ("Search menu items"), a select
  of what to show (**All items**, Active, Sold out, Retired), **Spreadsheet**
  (a menu: Export the menu, Import new items, Import choices, Change
  prices), and **Add New
  Item** (terracotta).
- **Three views**, as pills under the head: **Items**, **Choices**,
  **Stations and courses**.
- The screen's problem line, for a refusal that belongs to no field.

### Items (D18)

Three cards side by side from 1280px: the categories at the sidebar width
(`w-side`, 184 to 240px), the items at the back office's side width
(`w-aside`, 288 to 380px), and the editor the rest, so the editor is what
gives as the screen narrows (TECH-17). From 1024px the categories and the
items sit side by side and the editor drops below them; under 1024px all
three stack. At 1366x768 that is 191px, 328px and about 527px for the
editor; at 1024x768, 184px beside the items, and the editor the full width
below.

- **Categories:** "Categories" in the serif. Then **All items**,
  **Popular**, and each category in order, with its count; a retired one
  says "Retired". A long name wraps to a second line rather than losing its
  end. The list's last row is **+ Add a category** in terracotta: at the
  sidebar's 184px the design's round **+** beside the title does not fit.
  Under the list, for the category chosen: **Rename**, **Move up**, **Move
  down**, and **Retire** (danger, asked first with its count of items) or
  **Bring back**. The card's padding is 12px under
  1536px, 16px from there.
- **Menu items (68):** the title with its count, and a select for the
  order: **Menu order**, **A–Z**, **Price**. Each row is one button: the
  photo where there is one (and its room, empty, where other rows have
  one), the name, the price, and a chip only where the item is **Sold
  out** or **Retired** (BO-16, ADR-404). The open one has the green tint. Its **...** holds
  **Sold out today** or **Back on sale**, **Retire** or **Bring back**, and
  **Move up**, **Move down**. The list scrolls inside itself. A retire is
  said in a toast with **Undo**.
- **The editor**, for the item open: "Edit menu item" or "Add menu item",
  its chip, and beside the title **Sold out today** (or **Back on sale**)
  and **Retire item** in danger (or **Bring back**). Retiring is not asked:
  the toast says "Retired Filter coffee. It is off every order screen." with
  **Undo** for 8 seconds (BO-6, ADR-387).
  - **Item name**; **Category**; **Price, INR** ("As the order screen
    charges it, like 180 or 180.50."); **Tax class** ("Food, 5%"); **HSN or
    SAC code** ("4, 6 or 8 digits. Every GST invoice line shows it." on a
    venue that needs one); **Description**, with its length counted.
  - Two toggles: **Choices** (the groups the item offers, in order, with
    arrows and Remove, and **Offer another**) and **Kitchen and tax**
    (**Made at**, a station; **Course**).
  - Beside the fields from 1700px wide, in a column the sidebar width
    (`w-side`), or under them below that: the **photo** (a preview,
    **Choose a photo**, **Remove**) and **Tags** (the diet mark on the card,
    **Gluten free**, **Popular**).
  - **The save bar**, at the editor's foot at every width (ADR-381): a
    card holding "Unsaved changes" (or "Saving") at the left, a live
    region, while the form differs from what is saved, then **Cancel** and
    **Save changes** or **Add item** (the primary). It sticks to the bottom
    of the screen while the editor runs on below it and rests under the
    editor once its end is in view, so at 1366x768 Save is in sight
    whichever field is being typed in, the tags included. "Unsaved changes"
    reads the form as the server would keep it: "180" for "180.00" and
    spaces at either end are no change. On a screen 820px tall or less the
    editor card's padding is 16px and the bar's 8px top and bottom.
- **The price notice.** Typing a different price on an item shows, under
  the price, before Save: "Orders already open keep ₹180.00. Lines rung up
  after you save are ₹200.00."

### Choices

- A table of groups: Group, what it asks ("Pick 1", "Optional, up to 2",
  "Pick 1 to 3"), its choices (each name, its price change, "(to start)" on
  a default), how many items offer it, and **Edit**, **Retire** (danger,
  asked first: "Retire Milk?", "12 items stop offering it. Orders already
  open keep their choices.") or **Bring back**. **Add a group of choices** is in the head.
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
- **A new venue starts with one of each** (ADR-400): a Kitchen (code
  `kitchen`) and course 1, Main, made at setup, so its first items, by hand
  or by a file, need neither chosen. Under No tax or the GST composition
  scheme it starts with a tax class too, "No tax", exempt (ADR-401).
- **Retire** (a modal): says what it does, and for a row with items asks
  where they go ("Move its 10 items to"). The only row in service cannot be
  retired: "Add another before retiring it." A toast says what moved: "Bar
  retired. 10 items moved to Kitchen. Its printer now serves Kitchen."

### The Spreadsheet menu (ADR-297)

One pill in the head, **Spreadsheet**, opens the three things a file does,
each with a line under it saying what it does and does not touch:

- **Export the menu**: "Save every item as a CSV file."
- **Import new items**: "Add items from a file. Nothing already on the menu
  changes."
- **Change prices**: "Give back an export with its prices edited."

It is one pill and not three buttons: with the search, the select and Add
New Item the head had no room, and the three belong together. While a file
is being saved the pill says **Exporting**.

### Export (ADR-293, ADR-294)

**Export the menu** saves `mez-menu.csv`: every item on the menu, retired
ones included, in menu order.

- It is always the whole menu. The search, the select and the category
  chosen do not narrow it: a copy that leaves out what a filter hid is not
  a copy. Its title says so: "Save the whole menu as a CSV file".
- While it is being read the pill says **Exporting** and does not open
  again. There is no toast when it lands: the browser shows the
  download. A failure is a toast, "Couldn't download the file: ...".
- The file has the import's columns and **Status**, so it opens in a
  spreadsheet, and with rows added it can be given to **Import**, which
  adds the new rows and leaves the rest alone.
- It is off only while the menu is loading. A menu with no items exports
  its headings.

### Import (ADR-288 to ADR-291)

A modal from **Import new items**, for a menu that is already written
down somewhere. It adds items; it never changes or removes one.

- **Choose a file.** Three numbered steps, each one line:
  1. "Get the template. It has this venue's own category, tax class,
     station and course names in it." with **Download template**.
  2. "Fill it in, one item on each row, and save it as CSV UTF-8."
  3. "Choose the file." with **Choose a file** (`.csv`).
  A file that is not a `.csv`, or is over 1 MB, is refused here with what
  to do: "In your spreadsheet, use Save As and pick CSV UTF-8."
- **Checking** the file: "Checking banana.csv".
- **Something is wrong** with it: "3 problems in banana.csv. Nothing has
  been added." and a table of **Where** ("Row 5", or "The file"),
  **Column** and **What is wrong**, in the server's words, scrolling
  inside itself. What every item needs and the venue lacks, a tax class, a
  station or a course, is one row of "The file", in its column, with where
  to add it: "This venue has no station yet, and every item needs one. Add
  one under Menu, Stations and courses, then choose the file again." (a tax
  class: Settings, Tax and numbering), never one on every row (ADR-400).
  Then "Fix them in the file and choose it again." and
  **Choose the file again**. There is no way to add the rows that are
  right: a menu half of what its owner wrote is worse than none.
- **Ready:** "42 items to add." Then, each only when it applies: "3 new
  categories: Bakes, Drinks, Tiffin."; "5 are already on the menu and
  are left as they are."; "2 example rows from the template were left
  out." (a row named "Example: ..." is never imported, ADR-305). Under
  them the items to add, scrolling inside itself: the name over its
  category and the groups it will offer ("Snacks · offers Spice,
  Toppings"), and its price. **Add 42 items** is the primary; **Choose
  another file** beside it.
- **Nothing to add:** "Every item in menu.csv is already on the menu.", or
  for a file with none of the venue's own rows (the template as it came)
  "menu.csv has no items to add."; then the same lines saying what was
  left alone or left out.
- **Added:** the modal closes; a toast, "Added 42 items and 3
  categories." The list shows them, each in its category after what was
  there.
- **The menu changed** between the check and the add, so the file no
  longer fits: the file is checked again and shown as it stands now, under
  "The menu changed since this file was checked. This is how it stands
  now."

### Import choices (ADR-302, ADR-303)

A modal from **Import choices** in the Spreadsheet menu, for groups such as
Size, Milk or Extras. It adds groups; it never changes one that is there.
For a new venue it comes before the items, whose file names the groups each
item offers.

- **Choose a file.** Three numbered steps:
  1. "Get the template." with **Download template**
     (`mez-choices-template.csv`).
  2. "Fill it in, one choice on each row, with its group's name beside it.
     On a group's first row say the fewest and the most a guest picks: 1
     and 1 for a size they must choose, 0 and 2 for up to two extras. Price
     is what the choice adds, and may be below zero. Save it as CSV UTF-8."
  3. "Choose the file." with **Choose a file**.
- **Something is wrong** with it: the table of Where, Column and What is
  wrong, under "2 problems in choices.csv. Nothing has been added."
- **Ready:** "3 groups to add." and, each when it applies, "2 rows are for
  groups already on the menu, and are left as they are." and "4 example
  rows from the template were left out." Under them the
  groups, scrolling inside itself: the name with what it asks beside it
  ("Pick 1", "Optional, up to 2", as the Choices view words it), and under
  it its choices in order: "Regular (to start), Large +₹30.00, No cheese
  −₹20.00".
  - Under the list: "No item offers them yet. Offer a group from an item's
    editor, or name it in the Choices column when you import items."
  - **Add 3 groups** is the primary; **Choose another file** beside it.
- **Nothing to add:** "Every group in choices.csv is already on the menu.",
  or "choices.csv has no groups to add." for the template as it came; then
  the same lines.
- **Added:** the modal closes; a toast, "Added 3 groups. Offer them on
  items from each item's editor, or when you import items."

The items import (above) says in its second step: "To offer choices on an
item, import the choices first, then name the groups in the Choices column
with a semicolon between: Size; Milk."

### Change prices (ADR-295 to ADR-297)

A modal from **Change prices**, for many prices at once. It changes prices
and nothing else: a file never renames, moves, retires, adds or removes an
item.

- **Choose a file.** Three numbered steps:
  1. "Export the menu." with **Export the menu**.
  2. "Change the numbers in the Price column and save it as CSV UTF-8.
     Leave the Category and Name as they are: that is how each row finds
     its item."
  3. "Choose the file. You see every price it would change before any
     changes." with **Choose a file**.
- **Something is wrong** with it: as the import shows it, the table of
  Where, Column and What is wrong, under "3 problems in menu.csv. No price
  has been changed."
- **Ready:** "12 prices to change." Then, each only when it applies: "30
  rows keep the price they have."; "2 rows are not on the menu and are left
  out: Cortado, Idli. To add them, use Import new items." (the first five
  names, then "and 3 more"); and, in the attention tint, "1 item would be
  free." for a new price of zero.
  - The table: **Item** (its name over its category), **Now**, **New** and
    **Change** ("+₹10.00", "−₹5.00"), every figure tabular and
    right-aligned, the largest change first whichever way it goes, so a
    slipped digit is at the top. It scrolls inside itself.
  - Under it: "Orders already open keep the prices they were rung up at.
    The next line rung up takes the new one."
  - **Change 12 prices** is the primary; **Choose another file** beside it.
- **Nothing to change:** "No price in menu.csv differs from the menu."
- **Changed:** the modal closes; a toast, "Changed 12 prices. Orders
  already open keep theirs."
- **The menu changed** since the file was checked, by anyone, on any
  screen: the prices are not applied. The file is checked again and shown
  as it stands now, under "The menu changed since this file was checked.
  This is how it stands now.", and the owner presses again.

## Data

- `POST /api/v1/menu/prices/preview` and `POST /api/v1/menu/prices?version=`
  (ADR-296). The apply sends the `version` the preview answered.
- `GET /api/v1/menu/export` (ADR-293), saved as the reports' CSV files are.
- `GET /api/v1/menu/import/template`, `POST /api/v1/menu/import/preview`
  and `POST /api/v1/menu/import` (ADR-289, ADR-290). The file is sent as
  it is, as `text/csv`, whatever type the browser gave it: Windows calls a
  `.csv` an Excel file.
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
| Spreadsheet, Export the menu | Saves the whole menu as `mez-menu.csv` | The filter does not narrow it |
| Spreadsheet, Import new items | Opens the import (above) | Nothing is added until the file is checked and **Add** is pressed |
| Spreadsheet, Change prices | Opens the price changes (above) | No price changes until the file is checked and **Change** is pressed |
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
- With an unsaved edit, opening another item, adding one, another view, the
  sidebar, a `g` chord or Sign out asks "Leave without saving?", "Your
  changes to Filter coffee are not saved.", **Keep editing** and **Leave**
  (BO-2, ADR-387); the save bar says "Unsaved changes" while there is one.
  Cancel drops the edit: it is the way to. An item changed on
  another screen while it is open is saved over.
- The list no longer shows an item's tax class, station, course or HSN:
  they are in the editor.

## On a mirror

Read and not changed (ADR-330). The line under the title is "The items,
categories and their choices, as the venue last backed them up.", and "This
is a copy to read. Changes are made at the venue." is over the three views.

- **Items:** search, Show and Order stay: they change nothing. There is no
  Add New Item. The Spreadsheet pill offers Export the menu alone. The
  categories card has no Add a category and the chosen category no Rename,
  arrows or Retire. An item's row opens it and has no "...". The form's
  title is "Menu item"; every field, the photo and the tags are disabled;
  Choices and Kitchen and tax still switch, since each shows half the item;
  the sentence is under the tags; and there is no save bar. With nothing
  open: "Pick an item to read it."
- **Choices:** the table without its last column.
- **Stations and courses:** each card without Add, and each row without
  its buttons.

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

# Back office: Inventory

Where a manager keeps the stores: what there is, what came in, what was thrown
away and what a count found, and what each dish takes. Built in phase 10
(ADR-111 to ADR-114, ADR-124) and rebuilt from D19 in the rebuild's Task 16g
(ADR-191). `web/backoffice/src/stock/`: `StockScreen.tsx`, `StockLevels.tsx`,
`WastageCard.tsx`, `RecipesView.tsx`, `MovementModal.tsx`,
`StockItemModal.tsx`, `HistoryModal.tsx`, `levels-logic.ts`, `stock-logic.ts`,
`use-stock.ts`. Section **Inventory**, path `/stock`, `stock.manage`, keys
`g k`.

## Purpose

Answer "how much milk have we got, and where did the rest go?" from the
ledger, and keep the recipes that make sales take stock.

## Layout

Inside the frame (`backoffice-shell.md`).

- **The head:** "Inventory" in the serif, and under it a line of counts:
  "55 tracked ingredients · 6 low · 1 out of stock". At the right, **Stock
  count**, **Log wastage**, **Import**, and **Add ingredient** (terracotta).
  Import is of whatever the view shows: ingredients on Stock levels,
  recipes on Recipes.
- **Two views**, as pills: **Stock levels** and **Recipes**.

### Stock levels (D19)

The table in a card, and beside it the week's wastage.

| Column | Shows |
|---|---|
| Ingredient | the name; "Counted at the close" under it when it is |
| In stock | tabular, in its unit: "4,750 g", "12 each"; below zero in danger |
| Low at | its level; "Not set" muted when it has none |
| Unit cost | what a kilogram, a litre or one of it costs: `Money` then "/ kg", "/ l" or "/ each"; "Not set" muted |
| Stock level | a bar for how full it is, against three times its low level |
| Status | a chip: **Healthy**, **Low** (at or under its level), **Out** (none left); "Retired" muted for a retired one |
| | **...**: Receive, Log wastage, Count, History, Edit; a retired one has History and Edit only |

- Active ingredients first, by name; "Show the 2 retired" under the table
  adds the retired ones.
- **Wastage this week** (`reports.view`), with **View report**: what was
  written off in the last seven trading days, by ingredient, as bars at the
  cost each had then, with the total against the seven days before. "To
  write stock off, use Log wastage."

**The forms** (modals):

- **Stock count** and **Log wastage** from the head ask first which
  ingredient ("Count which ingredient?").
- **Receive:** "How much came in, in g".
- **Waste:** "How much was thrown away", and **Why** (Expired, Spoiled,
  Spilled, Dropped, Other); a note, required for Other.
- **Count:** "How much is there now". Under it, before Save: "The ledger
  says 4,750 g. This records −50 g."
- **Correct** (from Edit): the change, signed ("500" or "-500"), and a
  note saying why, required.
- **Add ingredient, Edit:** Name; **Counted in** (g, ml, each); **Low at**
  ("Shown as low at or under this. Empty for never."); **Cost of a litre,
  in INR** ("What you pay for it. It values a count that comes up short or
  over. Empty for not known."); **Count at close** ("Counted, blind, on the
  terminal as each shift closes."); and on an existing one **Correct**, and
  **Retire** or **Bring back**.
- **History:** the ingredient's movements, newest first: when, what
  (Received, Wasted: spilled, Counted 4,700 g, Sold, Corrected), the change
  with its sign, who, and the note. **Show older** loads the next page.

Amounts are typed in the unit with up to three decimals ("1.5", "250") and
sent in thousandths.

### Import ingredients (ADR-298, ADR-299)

A modal from **Import** on Stock levels, for a store that is already
written down somewhere. It is the menu's import (`backoffice-menu.md`,
"Import") for ingredients: it adds them, and never changes or removes one.

- **Choose a file.** Three numbered steps:
  1. "Get the template." with **Download template**
     (`mez-ingredients-template.csv`).
  2. "Fill it in, one ingredient on each row, and save it as CSV UTF-8.
     Counted in is g, ml or each. Low at is in that unit. Cost is what you
     pay for a kilogram, a litre or one."
  3. "Choose the file." with **Choose a file**.
- **Something is wrong** with it: the table of Where, Column and What is
  wrong, under "3 problems in store.csv. Nothing has been added."
- **Ready:** "12 ingredients to add." Then, each only when it applies: "2
  are already in the store and are left as they are."; and, in the
  attention tint, the line about the template's examples left in the file.
  Under them the ingredients to add, scrolling inside itself: the name
  over "Counted in ml · low at 2,000 ml · counted at the close", and its
  cost ("₹60.00 / l", or "Not costed").
  - Under the list: "They are added with nothing in stock. Use Receive
    when a delivery comes in."
  - **Add 12 ingredients** is the primary; **Choose another file** beside
    it.
- **Nothing to add:** "Every ingredient in store.csv is already in the
  store."
- **Added:** the modal closes; a toast, "Added 12 ingredients. Receive
  stock to put it on hand."

### Import recipes (ADR-300, ADR-301)

A modal from **Import** on the Recipes view. It gives a recipe to a dish
that has none; it never changes one that is there, and it is for dishes,
not for choices.

- **Choose a file.** Three numbered steps:
  1. "Get the template. It lists every dish that has no recipe yet." with
     **Download template** (`mez-recipes-template.csv`).
  2. "Beside each dish, type an ingredient by its name on Stock levels and
     how much one of the dish takes, in that ingredient's unit. For a
     second ingredient, add a row with the dish's name again. Leave a dish
     as it is to skip it. Save it as CSV UTF-8."
  3. "Choose the file." with **Choose a file**.
- **Something is wrong** with it: the table of Where, Column and What is
  wrong, under "2 problems in recipes.csv. Nothing has been added."
- **Ready:** "6 recipes to add." Then, each only when it applies: "3 rows
  are for dishes that already have a recipe, and are left as they are.";
  "12 dishes were left blank." Under them the recipes, scrolling inside
  itself: the dish over what one takes ("Coffee beans 18 g, Milk 200 ml").
  - Under the list: "Sales from now on take this stock. Sales already
    made are not changed."
  - **Add 6 recipes** is the primary; **Choose another file** beside it.
- **Nothing to add:** "No dish in recipes.csv is given a recipe.", with
  the same two lines saying why.
- **Added:** the modal closes; a toast, "Added 6 recipes. Sales from now
  on take this stock."

### Recipes

What one of each dish, and each choice, takes from stock. The dishes are the
order screen's menu, read from `GET /api/v1/menu`, so a manager without
`menu.edit` can keep recipes (ADR-112).

- "A dish takes its own recipe plus each chosen choice's. Put milk on the
  milk choices, not on the latte."
- A row per menu item: its name, what it takes ("Beans 18 g, Milk 200 ml")
  or "Nothing" muted, and **Edit**. Then each group of choices, a row per
  choice.
- **The recipe form**, "What one Latte takes": a row per ingredient (a
  select, the amount in its unit, Remove), **Add a stock item**, **Save**.

## Data

- `GET /api/v1/stock`, `GET /api/v1/stock/recipes`,
  `GET /api/v1/stock/items/{id}/movements?before=`, the menu through the
  shared `useMenu`.
- `GET /api/v1/reports/wastage?from=&to=`, twice, for the week and the week
  before (`reports.view`).
- The writes of ADR-111 and ADR-112; each reads the stock again, the moved
  ingredient's history, and the recipes on a recipe save.
- A sale takes stock through the recipes; a sold movement keeps its dish,
  so a dish can be costed (ADR-189).

## Interactions

| Action | Does | Then |
|---|---|---|
| Receive, Save | Adds a movement in | A toast: "Received 5,000 g of Coffee beans." |
| Log wastage, Save | Adds a write-off with its reason | "Recorded 200 ml of Milk thrown away." |
| Count, Save | Records the difference between the ledger and the count | "Counted. That recorded −50 g." |
| Correct, Save | Adds a signed correction | "Corrected Milk by +500 ml." |
| Add ingredient, Edit | Saves the ingredient | "Added Milk.", "Saved." |
| Retire, Bring back | No confirm: one click back, and the history stays | |
| A recipe, Save | Replaces what the dish takes | "Saved what Latte takes." |

## States

- **Loading:** "Reading the stock" under the title; "Loading the stock".
- **No ingredients:** "No ingredients yet. Add what you keep: beans, milk,
  batter."; with only retired ones, "Every ingredient is retired. Tick the
  box below to show them."
- **Error:** the server's words, with **Try again**.
- **Nothing written off:** "Nothing was written off in the last seven
  days."
- **A value the form cannot read** is refused under its field before
  anything is sent. Changing the unit of an ingredient with history is
  refused by the server, under Counted in.
- **No history:** "Nothing recorded yet."
- **Without `reports.view`:** no wastage card.

## Constraints

- No green primary on Stock levels: Add ingredient is terracotta, and
  nothing here moves money. Each modal's Save is its one primary.
- Every amount tabular and right-aligned, its unit written after it.
- A status is a chip with a word; the bar only repeats it. A full bar is
  three times the low level, a rule of ours, not a figure.
- The ledger is append-only: a count or a correction adds a movement; none
  is ever edited.
- A new cost is for stock moved from then on: what is already in the
  history keeps the cost it had (ADR-124).

## Reference

`mez Desktop Management.dc.html`, D19 Inventory (line 428). Left out, and
why:

- **Purchase orders and Suppliers** (the tabs, the Supplier column, "Open
  purchase orders", "New purchase order"): purchasing is out of v1. **Add
  ingredient** stands where New purchase order is drawn.
- **The Wastage log tab:** the report is on Sales reports, under Items and
  inventory, and the card links to it.
- **"Wastage is down 8% week on week":** the card gives the total and the
  change against the week before, in words, when there is a week before.

Changed: "Reorder at" is **Low at**, since nothing is ordered from here.

Kept, though not drawn (rule 4): the Recipes view; Receive, Count, History
and Edit under a row's "..."; Stock count and Log wastage beside the title;
Count at close.

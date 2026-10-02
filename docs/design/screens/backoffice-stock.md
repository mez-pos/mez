# Back office: Stock

Where a manager keeps the stores: what there is, what came in, what was
thrown away and what a count found, and what each dish takes. Built in
phase 10 Task 3c over the server of Tasks 3a and 3b (ADR-111, ADR-112).
Low stock on the dashboard and the closing count were added in Task 3d
(ADR-114).

## Purpose

Answer "how much milk have we got, and where did the rest go?" from the
ledger, and keep the recipes that make sales take stock.

## Layout

Paper, inside the back office shell, section **Stock**, path `/stock`,
`stock.manage`.

- **Header row:** "Stock" in the serif; **Add a stock item**
  (primary) on the Items view.
- **Under it, one line** in `fg-3`: "On hand is what was received, less
  what was sold, wasted and corrected. A count records the difference."
- **A switch, Items and Recipes** (`aria-pressed`), as the Menu page has.

### Items

A dense table, by name:

| Column | Shows |
|---|---|
| Item | the name; "Retired" (struck, muted) when retired; "LOW" in the warning colour when low |
| On hand | tabular, right-aligned, in its unit: "4,750 g", "12 each", "1.5 each"; below zero in the danger colour |
| Low at | tabular, right-aligned; "None" when 0 |
| Cost | what a kilogram, a litre or one of it costs, by its unit: `Money` then "/ kg", "/ l" or "/ each"; "Not set" muted when not known |
| | **Receive**, **Waste**, **Count**, **History**, **Edit** (48px); a retired item has History and Edit only |

- **The movement form** (`Modal`, sm), opened by Receive, Waste or Count,
  or by **Correct** inside Edit:
  - Receive: "How much came in", in the unit.
  - Waste: how much, and **Why** (Expired, Spoiled, Spilled, Dropped,
    Other); a note, required for Other.
  - Count: "How much is there now". Under it, live: "The ledger says
    4,750 g. This records −50 g." before Save.
  - Correct: the change, signed ("-500" or "500"), and a note, required.
  - Amounts are typed in the unit with up to three decimals ("1.5",
    "250") and sent in thousandths. A value the form cannot read is
    refused under the field before anything is sent.
- **Edit** (`Modal`, sm): name, unit (g, ml, each), low-stock level;
  **Correct**, and **Retire** or **Bring back**. Changing the unit of an
  item with history is refused by the server, with its reason under Unit.
- **History** (`Modal`, md): the item's movements, newest first: when,
  what (Received, Wasted: spilled, Counted 4,700 g, Sold, Corrected), the
  change in the numeral face with its sign, who, and the note. **Show older** loads
  the next page (cursor).

### Recipes

What one of each dish, and each choice, takes from stock. The dishes and
choices are the till's menu (on sale ones), read from `GET /api/v1/menu`
so a manager without `menu.edit` can keep recipes.

- A line: "A dish takes its own recipe plus each chosen choice's. Put
  milk on the milk choices, not on the latte."
- One row per menu item: its name, what it takes ("Beans 18 g, Milk
  200 ml") or "Nothing" muted, and **Edit**.
- Then each group of choices, one row per choice, the same way.
- **The recipe form** (`Modal`, md): one row per stock item it takes: a
  select of active stock items, the amount in that item's unit, Remove;
  **Add a stock item**; Save. The server's refusal shows on its row.

## Low stock and the closing count (Task 3d)

- **Edit** has **Cost** (phase 10 Task 5f1, ADR-124): what the venue
  pays for a kilogram of an item counted in grams, a litre of one in
  millilitres, or one of an item counted by the piece, in the venue's
  currency; the label says which ("Cost of a litre, in INR"). Empty for
  not known. A new cost is for stock moved from now on: what is already
  in the history keeps the cost it had.
- **Edit** has **Count at close**: the item is counted, blind, on the till
  as each shift closes (`terminal-close.md`). The Items table marks such
  an item "AT CLOSE".
- **The dashboard** shows a **Low stock** card to whoever has
  `stock.manage`: each low item's on hand "of" its level, or "Nothing is
  low.", and Go to stock.

## Data

- `GET /api/v1/stock` (`["stock"]`), `GET /api/v1/stock/recipes`
  (`["stock-recipes"]`), `GET /api/v1/stock/items/{id}/movements?before=`
  (`["stock-history", id]`), the menu through the shared `useMenu`.
- Writes of ADR-111 and ADR-112; each invalidates `["stock"]`, the moved
  item's history, and the recipes on a recipe save.

## States

- **No stock items:** "No stock yet. Add what you keep: beans, milk,
  batter."
- **Loading / offline / refused:** the words in place of the table, with
  Try again.
- **Saved:** a toast: "Received 5,000 g of Coffee beans." / "Counted. That
  recorded −50 g." / "Saved."

## Constraints

- One primary per view: **Add a stock item** on Items; none on Recipes.
- Every amount tabular and right-aligned, its unit written after it.
- Retire has no confirm: it is one click back, and the history stays.

## Reference

Built from `docs/design/components.md` and the paper tokens. The archive
was not consulted.

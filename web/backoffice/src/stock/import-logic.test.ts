import { describe, expect, it } from "vitest";
import type { StockImportRow, StockItemsImport } from "../../../shared/api-types";
import { addIngredientsLabel, addedIngredientsText, ingredientLine, ingredientsSummary, ingredientsVerdict, itemExamples } from "./import-logic";

const row = (patch: Partial<StockImportRow> = {}): StockImportRow => ({
  row: 2,
  name: "Milk",
  unit: "ml",
  low_level: 2_000_000,
  cost: { minor: 6000, currency: "INR" },
  count_at_close: true,
  ...patch,
});

const plan = (patch: Partial<StockItemsImport> = {}): StockItemsImport => ({ to_add: [], skipped: [], problems: [], imported: false, ...patch });

describe("ingredientsVerdict", () => {
  it("is ready with rows to add and nothing wrong", () => {
    expect(ingredientsVerdict(plan({ to_add: [row()] }))).toBe("ready");
  });

  it("is problems as soon as there is one", () => {
    expect(ingredientsVerdict(plan({ to_add: [row()], problems: [{ row: 3, column: "Cost", message: "x" }] }))).toBe("problems");
  });

  it("is nothing when every row is already there", () => {
    expect(ingredientsVerdict(plan({ skipped: [{ row: 2, name: "Milk" }] }))).toBe("nothing");
  });
});

describe("ingredientsSummary", () => {
  it("counts what is to add, and agrees in number", () => {
    expect(ingredientsSummary(plan({ to_add: [row()] }))).toEqual(["1 ingredient to add."]);
    expect(ingredientsSummary(plan({ to_add: [row(), row({ row: 3, name: "Eggs" })] }))).toEqual(["2 ingredients to add."]);
  });

  it("counts what is left alone", () => {
    expect(ingredientsSummary(plan({ to_add: [row()], skipped: [{ row: 3, name: "Beans" }] }))).toEqual([
      "1 ingredient to add.",
      "1 is already in the store and is left as it is.",
    ]);
    expect(
      ingredientsSummary(
        plan({
          to_add: [row()],
          skipped: [
            { row: 3, name: "Beans" },
            { row: 4, name: "Sugar" },
          ],
        }),
      ),
    ).toEqual(["1 ingredient to add.", "2 are already in the store and are left as they are."]);
  });
});

describe("ingredientLine", () => {
  it("says what it is counted in, where it is low and whether it is counted at the close", () => {
    expect(ingredientLine(row())).toBe("Counted in ml · low at 2,000 ml · counted at the close");
  });

  it("leaves out what was not given", () => {
    expect(ingredientLine(row({ unit: "each", low_level: 0, count_at_close: false }))).toBe("Counted each");
    expect(ingredientLine(row({ unit: "g", low_level: 500_500, count_at_close: false }))).toBe("Counted in g · low at 500.5 g");
  });
});

describe("itemExamples", () => {
  it("counts the template's examples left in the file", () => {
    expect(itemExamples(plan({ to_add: [row({ name: "Example: Milk" }), row({ name: "Milk" })] }))).toBe(1);
  });
});

describe("the words on the button and the toast", () => {
  it("counts on the button", () => {
    expect(addIngredientsLabel(plan({ to_add: [row()] }))).toBe("Add 1 ingredient");
    expect(addIngredientsLabel(plan({ to_add: [row(), row({ row: 3 })] }))).toBe("Add 2 ingredients");
  });

  it("says what was added, and that none of it is in stock yet", () => {
    expect(addedIngredientsText(plan({ to_add: [row()] }))).toBe("Added 1 ingredient. Receive stock to put it on hand.");
    expect(addedIngredientsText(plan({ to_add: [row(), row({ row: 3 })] }))).toBe("Added 2 ingredients. Receive stock to put it on hand.");
  });
});

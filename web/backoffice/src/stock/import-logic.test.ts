import { describe, expect, it } from "vitest";
import type { RecipeImportDish, RecipesImport, StockImportRow, StockItemsImport } from "../../../shared/api-types";
import {
  addIngredientsLabel,
  addRecipesLabel,
  addedIngredientsText,
  addedRecipesText,
  ingredientLine,
  ingredientsSummary,
  ingredientsVerdict,
  ingredientsLeftOut,
  ingredientsNothingText,
  recipeLine,
  recipeProblemsText,
  recipesLeftOut,
  recipesSummary,
  recipesVerdict,
} from "./import-logic";

const row = (patch: Partial<StockImportRow> = {}): StockImportRow => ({
  row: 2,
  name: "Milk",
  unit: "ml",
  low_level: 2_000_000,
  cost: { minor: 6000, currency: "INR" },
  count_at_close: true,
  ...patch,
});

const plan = (patch: Partial<StockItemsImport> = {}): StockItemsImport => ({ to_add: [], skipped: [], problems: [], examples: 0, imported: false, ...patch });

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

describe("the template's examples, left in a file", () => {
  it("are said to be left out, under what the file adds", () => {
    expect(ingredientsSummary(plan({ to_add: [row()], examples: 3 }))).toEqual(["1 ingredient to add.", "3 example rows from the template were left out."]);
    expect(ingredientsLeftOut(plan({ examples: 1 }))).toEqual(["1 example row from the template was left out."]);
  });

  it("make a file of only them one with nothing to add, not one already imported", () => {
    expect(ingredientsNothingText(plan({ examples: 3 }), "t.csv")).toBe("t.csv has no ingredients to add.");
    expect(ingredientsNothingText(plan({ skipped: [{ row: 2, name: "Milk" }] }), "t.csv")).toBe("Every ingredient in t.csv is already in the store.");
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

const dish = (name: string, parts: RecipeImportDish["parts"]): RecipeImportDish => ({ item_id: 1, dish: name, parts });
const beans = { row: 2, stock_item_id: 10, ingredient: "Coffee beans", unit: "g" as const, qty: 18_000 };
const milk = { row: 3, stock_item_id: 11, ingredient: "Milk", unit: "ml" as const, qty: 200_500 };

const recipes = (patch: Partial<RecipesImport> = {}): RecipesImport => ({ to_add: [], skipped: [], blank: 0, problems: [], imported: false, ...patch });

describe("recipesVerdict", () => {
  it("is ready with a recipe to add and nothing wrong", () => {
    expect(recipesVerdict(recipes({ to_add: [dish("Espresso", [beans])] }))).toBe("ready");
  });

  it("is problems as soon as there is one", () => {
    expect(recipesVerdict(recipes({ to_add: [dish("Espresso", [beans])], problems: [{ row: 4, column: "Amount", message: "x" }] }))).toBe("problems");
  });

  it("is nothing when every row was left blank or is for a dish that has one", () => {
    expect(recipesVerdict(recipes({ blank: 12, skipped: [{ row: 2, dish: "Latte" }] }))).toBe("nothing");
  });
});

describe("recipesSummary", () => {
  it("counts the recipes, and leaves out what does not apply", () => {
    expect(recipesSummary(recipes({ to_add: [dish("Espresso", [beans])] }))).toEqual(["1 recipe to add."]);
  });

  it("says why other rows gave nothing, agreeing in number", () => {
    expect(
      recipesSummary(recipes({ to_add: [dish("Espresso", [beans]), dish("Latte", [beans, milk])], skipped: [{ row: 5, dish: "Mocha" }], blank: 1 })),
    ).toEqual(["2 recipes to add.", "1 row is for a dish that already has a recipe, and is left as it is.", "1 dish was left blank."]);
    expect(
      recipesLeftOut(
        recipes({
          skipped: [
            { row: 5, dish: "Mocha" },
            { row: 6, dish: "Mocha" },
          ],
          blank: 12,
        }),
      ),
    ).toEqual(["2 rows are for dishes that already have a recipe, and are left as they are.", "12 dishes were left blank."]);
  });
});

describe("recipeLine", () => {
  it("is what one of the dish takes, each in its own unit", () => {
    expect(recipeLine(dish("Latte", [beans, milk]))).toBe("Coffee beans 18 g, Milk 200.5 ml");
  });
});

describe("the words for recipes on the button and the toast", () => {
  it("counts", () => {
    expect(addRecipesLabel(recipes({ to_add: [dish("Espresso", [beans])] }))).toBe("Add 1 recipe");
    expect(addedRecipesText(recipes({ to_add: [dish("Espresso", [beans]), dish("Latte", [milk])] }))).toBe(
      "Added 2 recipes. Sales from now on take this stock.",
    );
    expect(recipeProblemsText(recipes({ problems: [{ row: 4, column: "Amount", message: "x" }] }), "r.csv")).toBe(
      "1 problem in r.csv. Nothing has been added.",
    );
  });
});

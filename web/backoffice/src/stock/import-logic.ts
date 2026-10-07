// The words of the two imports on Inventory (backoffice-stock.md, "Import
// ingredients" and "Import recipes"). The server reads the file and says what it would add; this
// turns its answer into what the modal shows.

import type { RecipeImportDish, RecipesImport, StockImportRow, StockItemsImport } from "../../../shared/api-types";
import { formatQty } from "../../../shared/stock-qty";
import { count, examplesAmong, type Verdict } from "../files/file-logic";

export const itemsTemplatePath = "/api/v1/stock/import/template";

export function ingredientsVerdict(plan: StockItemsImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

/** What a file ready to add would do, a line for each thing that applies. */
export function ingredientsSummary(plan: StockItemsImport): string[] {
  const lines = [`${count(plan.to_add.length, "ingredient", "ingredients")} to add.`];
  const there = plan.skipped.length;
  if (there === 1) lines.push("1 is already in the store and is left as it is.");
  if (there > 1) lines.push(`${there} are already in the store and are left as they are.`);
  return lines;
}

/** An ingredient to add, in a line: what it is counted in, where it is low, whether it is counted at the close. */
export function ingredientLine(r: StockImportRow): string {
  const parts = [r.unit === "each" ? "Counted each" : `Counted in ${r.unit}`];
  if (r.low_level > 0) parts.push(`low at ${formatQty(r.low_level, r.unit)}`);
  if (r.count_at_close) parts.push("counted at the close");
  return parts.join(" · ");
}

/** How many of the rows to add are the template's examples, left in the file. */
export function itemExamples(plan: StockItemsImport): number {
  return examplesAmong(plan.to_add.map((r) => r.name));
}

export function itemExamplesText(n: number): string {
  return n === 1
    ? "1 of these is one of the template's examples. Take that row out of the file unless you want it in the store."
    : `${n} of these are the template's examples. Take those rows out of the file unless you want them in the store.`;
}

export function addIngredientsLabel(plan: StockItemsImport): string {
  return `Add ${count(plan.to_add.length, "ingredient", "ingredients")}`;
}

export function addedIngredientsText(plan: StockItemsImport): string {
  return `Added ${count(plan.to_add.length, "ingredient", "ingredients")}. Receive stock to put it on hand.`;
}

export function ingredientProblemsText(plan: StockItemsImport, filename: string): string {
  return `${count(plan.problems.length, "problem", "problems")} in ${filename}. Nothing has been added.`;
}

// Recipes from a file (backoffice-stock.md, "Import recipes").

export const recipesTemplatePath = "/api/v1/stock/recipes/import/template";

export function recipesVerdict(plan: RecipesImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

/** Why some rows gave nothing: their dish has a recipe, or they were left as the template had them. */
export function recipesLeftOut(plan: RecipesImport): string[] {
  const lines: string[] = [];
  const there = plan.skipped.length;
  if (there === 1) lines.push("1 row is for a dish that already has a recipe, and is left as it is.");
  if (there > 1) lines.push(`${there} rows are for dishes that already have a recipe, and are left as they are.`);
  if (plan.blank === 1) lines.push("1 dish was left blank.");
  if (plan.blank > 1) lines.push(`${plan.blank} dishes were left blank.`);
  return lines;
}

/** What a file ready to add would do, a line for each thing that applies. */
export function recipesSummary(plan: RecipesImport): string[] {
  return [`${count(plan.to_add.length, "recipe", "recipes")} to add.`, ...recipesLeftOut(plan)];
}

/** What one of a dish takes, in a line: "Coffee beans 18 g, Milk 200 ml". */
export function recipeLine(dish: RecipeImportDish): string {
  return dish.parts.map((p) => `${p.ingredient} ${formatQty(p.qty, p.unit)}`).join(", ");
}

export function addRecipesLabel(plan: RecipesImport): string {
  return `Add ${count(plan.to_add.length, "recipe", "recipes")}`;
}

export function addedRecipesText(plan: RecipesImport): string {
  return `Added ${count(plan.to_add.length, "recipe", "recipes")}. Sales from now on take this stock.`;
}

export function recipeProblemsText(plan: RecipesImport, filename: string): string {
  return `${count(plan.problems.length, "problem", "problems")} in ${filename}. Nothing has been added.`;
}

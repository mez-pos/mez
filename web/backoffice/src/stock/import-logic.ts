// The words of the two imports on Inventory (backoffice-stock.md, "Import
// ingredients" and "Import recipes"). The server reads the file and says what it would add; this
// turns its answer into what the modal shows.

import type { RecipeImportDish, RecipesImport, StockImportRow, StockItemsImport } from "../../../shared/api-types";
import { formatQty } from "../../../shared/stock-qty";
import { count, examplesLeftOut, type Verdict } from "../files/file-logic";

export const itemsTemplatePath = "/api/v1/stock/import/template";

export function ingredientsVerdict(plan: StockItemsImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

/** What a file ready to add would do, a line for each thing that applies. */
export function ingredientsSummary(plan: StockItemsImport): string[] {
  return [`${count(plan.to_add.length, "ingredient", "ingredients")} to add.`, ...ingredientsLeftOut(plan)];
}

/** Why some rows add nothing: their ingredient is already there, or they are the template's examples. */
export function ingredientsLeftOut(plan: StockItemsImport): string[] {
  const lines: string[] = [];
  const there = plan.skipped.length;
  if (there === 1) lines.push("1 is already in the store and is left as it is.");
  if (there > 1) lines.push(`${there} are already in the store and are left as they are.`);
  const examples = examplesLeftOut(plan.examples);
  if (examples) lines.push(examples);
  return lines;
}

/** What is said of a file of ingredients that adds nothing. */
export function ingredientsNothingText(plan: StockItemsImport, filename: string): string {
  return plan.skipped.length > 0 ? `Every ingredient in ${filename} is already in the store.` : `${filename} has no ingredients to add.`;
}

/** An ingredient to add, in a line: what it is counted in, where it is low, whether it is counted at the close. */
export function ingredientLine(r: StockImportRow): string {
  const parts = [r.unit === "each" ? "Counted each" : `Counted in ${r.unit}`];
  if (r.low_level > 0) parts.push(`low at ${formatQty(r.low_level, r.unit)}`);
  if (r.count_at_close) parts.push("counted at the close");
  return parts.join(" · ");
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

// The words and the checks of the menu import (backoffice-menu.md, "Import").
// The server reads the file and says what it would add; this turns its
// answer into what the modal shows.

import type { ChoicesImport, ChoicesImportChoice, MenuImport, MenuImportItem, Money } from "../../../shared/api-types";
import { count, examplesAmong, type Verdict } from "../files/file-logic";

// The checks of any file are in files/file-logic.ts. They are passed on from
// here so the menu's dialogs read from one place.
export { asCsv, fileProblem, maxImportBytes, whereOf, type Verdict } from "../files/file-logic";

export const templatePath = "/api/v1/menu/import/template";
/** The whole menu as a file the import takes back (ADR-293). */
export const exportPath = "/api/v1/menu/export";

export function verdictOf(plan: MenuImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

/** What a file ready to add would do, a line for each thing that applies. */
export function summaryLines(plan: MenuImport): string[] {
  const lines = [`${count(plan.to_add.length, "item", "items")} to add.`];
  if (plan.new_categories.length > 0) {
    lines.push(`${count(plan.new_categories.length, "new category", "new categories")}: ${plan.new_categories.join(", ")}.`);
  }
  const there = plan.skipped.length;
  if (there === 1) lines.push("1 is already on the menu and is left as it is.");
  if (there > 1) lines.push(`${there} are already on the menu and are left as they are.`);
  return lines;
}

/** An item to add, in a line under its name: its category, and the groups of choices it offers. */
export function itemLine(it: MenuImportItem): string {
  return it.choices.length === 0 ? it.category : `${it.category} · offers ${it.choices.join(", ")}`;
}

/** How many of the rows to add are the template's examples, left in the file. */
export function examplesIn(plan: MenuImport): number {
  return examplesAmong(plan.to_add.map((it) => it.name));
}

export function examplesText(n: number): string {
  return n === 1
    ? "1 of these is one of the template's examples. Take that row out of the file unless you want it on the menu."
    : `${n} of these are the template's examples. Take those rows out of the file unless you want them on the menu.`;
}

export function addLabel(plan: MenuImport): string {
  return `Add ${count(plan.to_add.length, "item", "items")}`;
}

export function addedText(plan: MenuImport): string {
  const items = count(plan.to_add.length, "item", "items");
  const made = plan.new_categories.length;
  return made === 0 ? `Added ${items}.` : `Added ${items} and ${count(made, "category", "categories")}.`;
}

export function problemsText(plan: MenuImport, filename: string): string {
  return `${count(plan.problems.length, "problem", "problems")} in ${filename}. Nothing has been added.`;
}

// Groups of choices from a file (backoffice-menu.md, "Import choices").

export const choicesTemplatePath = "/api/v1/menu/choices/import/template";

export function choicesVerdict(plan: ChoicesImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

/** What a file of choices ready to add would do, a line for each thing that applies. */
export function choicesSummary(plan: ChoicesImport): string[] {
  const lines = [`${count(plan.to_add.length, "group", "groups")} to add.`];
  const there = plan.skipped.length;
  if (there === 1) lines.push("1 row is for a group already on the menu, and is left as it is.");
  if (there > 1) lines.push(`${there} rows are for groups already on the menu, and are left as they are.`);
  return lines;
}

/** How a choice's price reads beside its name: nothing, "+" and an amount, or "−" and an amount. */
export interface ChoicePrice {
  sign: "+" | "−";
  amount: Money;
}

export function choicePrice(c: ChoicesImportChoice): ChoicePrice | null {
  if (c.price.minor === 0) return null;
  return { sign: c.price.minor > 0 ? "+" : "−", amount: { minor: Math.abs(c.price.minor), currency: c.price.currency } };
}

export function addGroupsLabel(plan: ChoicesImport): string {
  return `Add ${count(plan.to_add.length, "group", "groups")}`;
}

export function addedGroupsText(plan: ChoicesImport): string {
  return `Added ${count(plan.to_add.length, "group", "groups")}. Offer them on items from each item's editor, or when you import items.`;
}

export function choicesProblemsText(plan: ChoicesImport, filename: string): string {
  return `${count(plan.problems.length, "problem", "problems")} in ${filename}. Nothing has been added.`;
}

// The words and the checks of the menu import (backoffice-menu.md, "Import").
// The server reads the file and says what it would add; this turns its
// answer into what the modal shows.

import type { MenuImport, MenuImportProblem } from "../../../shared/api-types";

export const templatePath = "/api/v1/menu/import/template";

/** The largest file the server reads (menus.MaxImportBytes). */
export const maxImportBytes = 1 << 20;

/** What the template's example rows are named with (menu.ImportTemplate). */
const examplePrefix = "example:";

const saveAsCsv = "In your spreadsheet, use Save As and pick CSV UTF-8.";

/** Why a chosen file cannot be sent, or null. The server checks what is in it. */
export function fileProblem(file: { name: string; size: number }): string | null {
  if (!file.name.toLowerCase().endsWith(".csv")) return `Choose a CSV file. ${saveAsCsv}`;
  if (file.size === 0) return "That file is empty.";
  if (file.size > maxImportBytes) return "That file is over 1 MB. Import it in parts.";
  return null;
}

/**
 * The file as the server takes it. The browser's own type is dropped:
 * Windows calls a .csv an Excel file, and some browsers give it none.
 */
export function asCsv(file: Blob): Blob {
  return new Blob([file], { type: "text/csv" });
}

/** What a checked file comes to: it has problems, it adds nothing, or it is ready to add. */
export type Verdict = "problems" | "nothing" | "ready";

export function verdictOf(plan: MenuImport): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.to_add.length > 0 ? "ready" : "nothing";
}

function count(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
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

/** How many of the rows to add are the template's examples, left in the file. */
export function examplesIn(plan: MenuImport): number {
  return plan.to_add.filter((it) => it.name.toLowerCase().startsWith(examplePrefix)).length;
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

/** Where a problem is: its row as the spreadsheet numbers it, or the file as a whole. */
export function whereOf(p: MenuImportProblem): string {
  return p.row === 0 ? "The file" : `Row ${p.row}`;
}

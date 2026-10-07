// What every dialog that takes a CSV file checks and says, whatever the
// file is of: a menu, its prices, a store's ingredients, its recipes.

/** The largest file the server reads (menus.MaxImportBytes, stock.MaxImportBytes). */
export const maxImportBytes = 1 << 20;

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

/** What a checked file comes to: it has problems, it does nothing, or it is ready. */
export type Verdict = "problems" | "nothing" | "ready";

/** "1 item", "3 items". */
export function count(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}

/** A problem as the server sends it, whichever import it is from. */
export interface FileProblem {
  row: number;
  column: string;
  message: string;
}

/** Where a problem is: its row as the spreadsheet numbers it, or the file as a whole. */
export function whereOf(p: FileProblem): string {
  return p.row === 0 ? "The file" : `Row ${p.row}`;
}

/**
 * The line for a template's example rows left in a file. The server leaves
 * them out of every import and says how many (ADR-305); null for none.
 */
export function examplesLeftOut(n: number): string | null {
  if (n === 0) return null;
  return n === 1 ? "1 example row from the template was left out." : `${n} example rows from the template were left out.`;
}

// The codes on the tables, without React (backoffice-floor.md, ADR-243,
// ADR-244). A code is what a table's sticker says and a guest's only
// credential, so the words here are careful about what stops working.

import { ApiError } from "../../../shared/api";
import type { TableCodeResponse, TableCodesResponse } from "../../../shared/api-types";

/** How many tables guests cannot order from: no code on them. */
export function missingCount(list: Pick<TableCodesResponse, "tables">): number {
  return list.tables.filter((t) => t.code === "").length;
}

/** "All 12 tables have a code.", "3 of 12 tables have no code.", under the heading. */
export function codesSummary(list: Pick<TableCodesResponse, "tables">): string {
  const total = list.tables.length;
  const missing = missingCount(list);
  if (total === 0) return "No tables on the floor yet.";
  if (missing === 0) return total === 1 ? "The table has a code." : `All ${total} tables have a code.`;
  if (missing === total) return total === 1 ? "The table has no code." : `None of the ${total} tables has a code.`;
  return `${missing} of ${total} tables ${missing === 1 ? "has" : "have"} no code.`;
}

/** Whether the venue has said where guests reach it: without it nothing can be printed. */
export function canPrint(list: Pick<TableCodesResponse, "address">): boolean {
  return list.address !== "";
}

/** What the card says when the venue has no public address. */
export const noAddressWords =
  "The venue has not said where guests reach it, so there is no address to print on a table. Set public_url in the venue's configuration and restart mez.";

/**
 * A code as it is read out or checked against a sticker: in three groups of
 * four. The code itself has no spaces; this is only how it is shown.
 */
export function codeText(code: string): string {
  return code.replace(/(.{4})(?=.)/g, "$1 ");
}

/** What a row can do. A table with no code can only be given one. */
export type CodeAction = "show" | "renew" | "revoke" | "give";

export function actionsFor(table: Pick<TableCodeResponse, "code">, printable: boolean): CodeAction[] {
  if (table.code === "") return ["give"];
  return printable ? ["show", "renew", "revoke"] : ["renew", "revoke"];
}

export const actionLabels: Readonly<Record<CodeAction, string>> = {
  show: "Show the QR code",
  renew: "Renew the code",
  revoke: "Turn the code off",
  give: "Give it a code",
};

export interface ConfirmWords {
  readonly title: string;
  readonly body: string;
  readonly confirm: string;
  readonly done: string;
}

/**
 * What renewing or turning off asks before it is done. Both stop a sticker
 * that may be on a table now, and whatever a guest at that table has open.
 */
export function confirmWords(action: "renew" | "revoke", tableName: string): ConfirmWords {
  if (action === "renew") {
    return {
      title: `Renew the code for ${tableName}?`,
      body: "The code on the table stops working at once, and so does any photo of it. A guest ordering from that table now loses their place. Print the new code and put it on the table.",
      confirm: "Renew the code",
      done: `${tableName} has a new code. Print it and replace the one on the table.`,
    };
  }
  return {
    title: `Turn off the code for ${tableName}?`,
    body: "Nobody can order from this table by phone until you give it a code again. The code on the table stops working at once, and a guest ordering from it now loses their place.",
    confirm: "Turn the code off",
    done: `${tableName} takes no orders by phone.`,
  };
}

/** Why something to do with the codes failed, in words to act on. */
export function codesProblem(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "no_public_url") return noAddressWords;
    if (error.code === "table_not_found") return "That table is no longer on the floor. The list has been read again.";
    if (error.code === "forbidden" || error.status === 403) return "Your role cannot change the tables' codes.";
    if (error.unreachable) return "Can't reach the server. Check the connection and try again.";
  }
  return "That did not work. Try again.";
}

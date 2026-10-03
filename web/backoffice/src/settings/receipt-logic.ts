// The receipt template as a form (docs/design/screens/backoffice-settings.md,
// "Receipt"): a few lines under the venue's name, a few at the bottom, and
// three parts of a receipt to print or leave off. What the printer can print
// (ASCII, accents spelled plainly) is the server's to say, and the live
// preview says it as the owner types; what is checked here is only what
// costs nothing. No React in this file.

import type { ReceiptPreviewRequest, ReceiptRequest, ReceiptResponse } from "../../../shared/api-types";
import { problemWith, type Problem } from "./problem";

/** The paper widths a receipt printer takes. */
export type PaperMM = 80 | 58;

export const PAPERS: readonly { value: PaperMM; label: string }[] = [
  { value: 80, label: "80mm paper" },
  { value: 58, label: "58mm paper" },
];

export interface ReceiptForm {
  /** One line per row, as typed. */
  header: string;
  footer: string;
  /** What to print: the opposite of what the server stores. */
  barcode: boolean;
  orderRef: boolean;
  modifiers: boolean;
}

/** A block of text as the lines the server keeps: trimmed, blank ones dropped. */
export function linesOf(text: string): string[] {
  return text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter((l) => l !== "");
}

export function receiptFormFrom(r: ReceiptResponse): ReceiptForm {
  return {
    header: r.header.join("\n"),
    footer: r.footer.join("\n"),
    barcode: !r.hide_barcode,
    orderRef: !r.hide_order_ref,
    modifiers: !r.hide_modifiers,
  };
}

export function toReceiptInput(f: ReceiptForm): ReceiptRequest {
  return {
    header: linesOf(f.header),
    footer: linesOf(f.footer),
    hide_barcode: !f.barcode,
    hide_order_ref: !f.orderRef,
    hide_modifiers: !f.modifiers,
  };
}

export const toPreviewInput = (f: ReceiptForm, paper: PaperMM): ReceiptPreviewRequest => ({ ...toReceiptInput(f), paper_mm: paper });

/** Whether the draft differs from what the server has: blank lines and spaces do not count. */
export function receiptChanged(draft: ReceiptForm, saved: ReceiptForm): boolean {
  const a = toReceiptInput(draft);
  const b = toReceiptInput(saved);
  return JSON.stringify(a) !== JSON.stringify(b);
}

export type ReceiptField = "header" | "footer";
const receiptFields: readonly ReceiptField[] = ["header", "footer"];

export const receiptProblemOf = (err: unknown): Problem<ReceiptField> => problemWith(err, receiptFields);

/** What can be refused without asking the server: too many lines, a line too long. */
export function receiptProblem(f: ReceiptForm, limits: { maxLines: number; maxChars: number }): Problem<ReceiptField> | null {
  for (const [field, text] of [
    ["header", f.header],
    ["footer", f.footer],
  ] as const) {
    const lines = linesOf(text);
    if (lines.length > limits.maxLines) return { field, message: `Use at most ${limits.maxLines} lines.` };
    const long = lines.findIndex((l) => [...l].length > limits.maxChars);
    if (long >= 0) return { field, message: `Keep line ${long + 1} to ${limits.maxChars} characters.` };
  }
  return null;
}

/** The key a preview is asked for: the same draft at the same width is never asked twice running. */
export const receiptPreviewKey = (f: ReceiptForm, paper: PaperMM): string => JSON.stringify(toPreviewInput(f, paper));

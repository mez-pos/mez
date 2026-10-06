// The receipt, without React (docs/design/screens/guest-receipt.md,
// ADR-240, ADR-241). A settled bill as the counter's invoice has it. The
// phone remembers one thing, which bill was this table's while it sat here,
// so it can ask for that bill's receipt once the table's code has moved on
// to the next. Every figure is the server's.

import { ApiError } from "../../shared/api";
import type { GuestReceiptResponse, PaymentMethod } from "../../shared/api-types";
import { clockText } from "../../shared/venue-text";
import { rateText } from "../../shared/zreport/zreport-logic";
import type { BillRow, BillState } from "./bill-logic";

/** How long the phone remembers a bill: as long as the venue gives its receipt (ADR-240). */
export const BILL_REMEMBER_MS = 45 * 24 * 60 * 60 * 1000;

export interface KeptBill {
  readonly id: string;
  /** When the phone last saw it open, by its own clock. */
  readonly at: number;
}

export function billStorageKey(code: string): string {
  return `mez.bill.${code}`;
}

/** A bill's id as the server draws one. What is in storage is not trusted. */
const BILL_ID = /^ord_[0-9A-Z]{4,32}$/;

export function billIdOf(id: string | undefined): string | null {
  return id !== undefined && BILL_ID.test(id) ? id : null;
}

export function parseKeptBill(text: string | null, now: number): KeptBill | null {
  if (!text) return null;
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof raw !== "object" || raw === null) return null;
  const { id, at } = raw as { id?: unknown; at?: unknown };
  if (typeof id !== "string" || !BILL_ID.test(id) || typeof at !== "number" || !Number.isFinite(at)) return null;
  if (now - at > BILL_REMEMBER_MS || at - now > BILL_REMEMBER_MS) return null;
  return { id, at };
}

/**
 * Whether the bill's screen offers the receipt: the phone knows a bill, and
 * the table's bill is paid or gone. While something is owed there is no
 * receipt to offer, whichever bill the phone remembers.
 */
export function offersReceipt(state: BillState, kept: KeptBill | null): boolean {
  return kept !== null && (state === "paid" || state === "none");
}

/** "Sat, 19 Sep 2026 · 10:24 AM", in the venue's own time and not the phone's. */
export function issuedText(receipt: Pick<GuestReceiptResponse, "issued_at" | "time_zone">): string {
  const at = new Date(receipt.issued_at);
  return Number.isNaN(at.getTime()) ? "" : clockText(at, receipt.time_zone || null);
}

/** The rows between the lines and the total, as the bill has them. */
export function receiptRows(receipt: GuestReceiptResponse): BillRow[] {
  const count = receipt.lines.reduce((sum, l) => sum + l.qty, 0);
  const rows: BillRow[] = [{ label: `Items (${count})`, amount: receipt.items }];
  if (receipt.discounts.minor !== 0) rows.push({ label: "Discount", amount: receipt.discounts, minus: true });
  for (const t of receipt.taxes) {
    const name = `${t.name} (${rateText(t.rate)})`;
    rows.push(receipt.tax_included ? { label: `Includes ${name}`, amount: t.amount, within: true } : { label: name, amount: t.amount });
  }
  return rows;
}

const paidBy: Readonly<Record<PaymentMethod, string>> = {
  cash: "Paid by cash",
  card: "Paid by card",
  upi: "Paid by UPI",
  other: "Paid another way",
};

/** How it was paid and what has come back since, under the total. */
export function paidRows(receipt: GuestReceiptResponse): BillRow[] {
  const rows: BillRow[] = receipt.payments.map((p) => ({ label: paidBy[p.method], amount: p.amount }));
  if (receipt.credited.minor > 0) rows.push({ label: "Taken off since, by credit note", amount: receipt.credited, minus: true });
  if (receipt.refunded.minor > 0) rows.push({ label: "Refunded", amount: receipt.refunded, minus: true });
  return rows;
}

export type ReceiptProblem = "none_yet" | "unreachable" | "failed";

/** Why a receipt could not be read. */
export function receiptProblem(error: unknown): ReceiptProblem {
  if (error instanceof ApiError) {
    if (error.code === "receipt_not_found" || error.code === "table_code_unknown") return "none_yet";
    if (error.unreachable || (error.status !== null && error.status >= 502 && error.status <= 504)) return "unreachable";
  }
  return "failed";
}

/** What the screen says for each, in words a guest can act on. */
export function problemWords(problem: ReceiptProblem): string {
  switch (problem) {
    case "none_yet":
      return "There is no receipt for this bill here. A bill has one once it is paid in full. Ask a member of staff for a copy.";
    case "unreachable":
      return "Can't reach the kitchen to read your receipt. Check your connection and try again.";
    case "failed":
      return "Your receipt could not be read. Try again, or ask a member of staff for a copy.";
  }
}

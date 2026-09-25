// What the past bills screens decide for themselves (terminal-past-bills.md).
// Every money figure is the server's: the credit note preview, the order's
// totals. These only count units, choose, and word.

import type { CreditSelection, LineResponse, OrderResponse, PastBillResponse, TenderResponse } from "../../../shared/api-types";
import type { Reason } from "../register/cart-logic";

export const refundReasons: readonly Reason[] = [
  { code: "wrong_item", label: "Dish was wrong" },
  { code: "cold_or_late", label: "Cold or late" },
  { code: "guest_changed_mind", label: "Guest changed their mind" },
  { code: "overcharged", label: "Overcharged" },
  { code: "goodwill", label: "Goodwill" },
];

export const paymentVoidReasons: readonly Reason[] = [
  { code: "declined_after", label: "Card declined after" },
  { code: "charged_twice", label: "Charged twice" },
  { code: "upi_reversed", label: "UPI reversed" },
  { code: "wrong_method", label: "Wrong method recorded" },
];

/** A refund reason as the till offered it; a code it no longer offers, in words. */
export function refundReasonText(code: string): string {
  const offered = refundReasons.find((r) => r.code === code);
  if (offered) return offered.label;
  const words = code.replace(/_/g, " ").trim();
  return words ? words[0]!.toUpperCase() + words.slice(1) : "";
}

/** Units of a line not yet credited: what its stepper goes up to. */
export function unitsLeft(line: Pick<LineResponse, "qty" | "credited_qty" | "status">): number {
  return line.status === "active" ? Math.max(0, line.qty - line.credited_qty) : 0;
}

/** Whether anything on the bill can still be credited. */
export function anythingLeft(order: OrderResponse): boolean {
  return order.lines.some((l) => l.status === "active" && l.credited.minor < l.due.minor);
}

/** What the refund sheet asks the server to plan, or null while it asks for nothing. */
export type RefundDraft = { kind: "items"; units: Readonly<Record<number, number>> } | { kind: "amount"; minor: number };

export function selectionOf(draft: RefundDraft, currency: string): CreditSelection | null {
  if (draft.kind === "amount") {
    return draft.minor > 0 ? { amount: { minor: draft.minor, currency } } : null;
  }
  const lines = Object.entries(draft.units)
    .filter(([, qty]) => qty > 0)
    .map(([id, qty]) => ({ line_id: Number(id), qty }))
    .sort((a, b) => a.line_id - b.line_id);
  return lines.length > 0 ? { lines } : null;
}

/** Payments money can go back to: taken, and not all refunded already. */
export function refundable(tenders: readonly TenderResponse[]): TenderResponse[] {
  return tenders.filter((t) => t.status === "captured" && t.refunded.minor < t.amount.minor);
}

/** A payment a person may void: taken, and nothing refunded against it. */
export function voidable(t: TenderResponse): boolean {
  return t.status === "captured" && t.refunded.minor === 0;
}

/** Where a past bill was: "Table 4" or "Counter". */
export function whereOf(b: Pick<PastBillResponse, "table_id">): string {
  return b.table_id ? `Table ${b.table_id}` : "Counter";
}

/** When it closed, in the device's zone: "13:05" today, else "25/09 13:05". */
export function closedText(iso: string, now: Date): string {
  const at = new Date(iso);
  const hm = at.toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", hourCycle: "h23" });
  if (at.toDateString() === now.toDateString()) return hm;
  return `${at.toLocaleDateString("en-GB", { day: "2-digit", month: "2-digit" })} ${hm}`;
}

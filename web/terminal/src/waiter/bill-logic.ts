// What the waiter's bill (T05) says about a split and its shares. The shares
// are the server's (POST /orders/{id}/split) and so is every figure: nothing
// here divides or adds money. This names the shares, says who has paid and
// how, picks whose share is collected next, and says why a table cannot be
// closed yet.

import type { Money, OrderResponse, PaymentMethod, SplitKind, SplitResponse } from "../../../shared/api-types";
import { payerLabel, type TillSplit } from "../tender/split-logic";
import { methodNames } from "../tender/tender-logic";

/** The three ways to split, in the design's order and words. */
export const billModes: readonly { kind: TillSplit; label: string }[] = [
  { kind: "evenly", label: "Split equally" },
  { kind: "item", label: "By item" },
  { kind: "amount", label: "Custom" },
];

/** How a share is paid, in the design's order. Its Wallet is no payment method here. */
export const billMethods: readonly PaymentMethod[] = ["upi", "card", "cash"];

/** The shares paid on this tablet, and how each was. */
export type PaidShares = ReadonlyMap<number, PaymentMethod>;

/** What a share is called. A bill nobody shares has one, and it is the bill. */
export function shareName(kind: SplitKind, payer: number, amountsKeyed: number, count: number): string {
  if (count === 1) return "Whole bill";
  return payerLabel(kind, payer, amountsKeyed);
}

/** What each guest pays, when they all pay the same; null when there is no one figure to give. */
export function eachShare(split: SplitResponse): number | null {
  const [first, ...rest] = split.shares;
  if (!first || rest.length === 0) return null;
  return rest.every((s) => s.amount.minor === first.amount.minor) ? first.amount.minor : null;
}

export interface ShareRow {
  payer: number;
  name: string;
  amount: Money;
  /** Paid, the one being collected, or still to come. */
  state: "paid" | "collecting" | "waiting";
  paidBy: PaymentMethod | null;
}

export function shareRows(split: SplitResponse, amountsKeyed: number, paid: PaidShares, collecting: number | null): ShareRow[] {
  return split.shares.map((s) => {
    const paidBy = paid.get(s.payer) ?? null;
    return {
      payer: s.payer,
      name: shareName(split.kind, s.payer, amountsKeyed, split.shares.length),
      amount: s.amount,
      state: paidBy ? "paid" : s.payer === collecting ? "collecting" : "waiting",
      paidBy,
    };
  });
}

/** The line under a share's name: a word, never a colour alone (tokens.md). */
export function shareStatus(row: ShareRow): string {
  if (row.paidBy) return `Paid · ${methodNames[row.paidBy]}`;
  return row.state === "collecting" ? "Collecting now" : "Awaiting payment";
}

/** Whose share the next payment is: the one picked while it is unpaid, else the first unpaid. */
export function tablePays(split: SplitResponse, paid: PaidShares, picked: number | null): number | null {
  const unpaid = split.shares.filter((s) => !paid.has(s.payer));
  if (picked !== null && unpaid.some((s) => s.payer === picked)) return picked;
  return unpaid[0]?.payer ?? null;
}

/** Why Close table is not offered yet; null when it is. */
export function closeBlock(order: OrderResponse): string | null {
  if (order.status === "closed") return "This order is closed.";
  const due = order.totals.balance_due.minor;
  if (due > 0) return "Collect the whole bill to close the table.";
  if (due < 0) return "The guest is owed money back. A manager settles that on the Orders screen.";
  return null;
}

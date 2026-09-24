// What the tender screen works out for itself (terminal-tender.md). Every
// other figure on it is the server's. These are the only sums: the change to
// hand back, the part of a cash payment that settles the bill, and the
// round amounts a guest is likely to hand over. All in integer minor units.

import type { OrderResponse, PaymentMethod, TenderPayment } from "../../../shared/api-types";
import { exponentOf } from "../../../shared/money";

/**
 * Quick-tender steps in major units: the next ₹10, ₹50, ₹100 and ₹500 above
 * the balance. Written for rupees; they read sensibly for any currency with
 * notes of those sizes, and move to the venue's settings when it has any.
 */
const quickSteps = [10, 50, 100, 500];

/** Exact, then the next multiple of each step strictly above the balance, without repeats. */
export function quickTenders(due: number, currency: string): number[] {
  if (due <= 0) return [];
  const unit = 10 ** exponentOf(currency);
  const above = quickSteps.map((step) => {
    const size = step * unit;
    return (Math.floor(due / size) + 1) * size;
  });
  return [...new Set([due, ...above])];
}

export interface TenderInput {
  method: PaymentMethod;
  /** Cash: what the guest handed over. Card and UPI: the amount to take. */
  entered: number;
  tip: number;
  /** The ticket's balance, from the server, or the share of it being paid. */
  due: number;
  /**
   * A split share is paid whole, in one tender, so a split never has a
   * share part paid to keep track of. Card and UPI take exactly the share.
   */
  whole?: boolean;
}

export interface TenderPlan {
  /** What the server records against the bill; never more than the balance. */
  amount: number;
  /** Cash to hand back. Zero for card and UPI. */
  change: number;
  /** What the guest pays in all: for a card, what the card machine is keyed with. */
  charge: number;
  /** Why this cannot be sent yet, in the till's words; null when it can. */
  problem: string | null;
}

const refused = (problem: string): TenderPlan => ({ amount: 0, change: 0, charge: 0, problem });

export function planTender({ method, entered, tip, due, whole = false }: TenderInput): TenderPlan {
  if (due <= 0) return refused("Nothing is owed on this ticket.");
  if (method === "cash") {
    // The tip is left out of the cash handed over; the rest pays the bill,
    // and what the bill does not need is change.
    if (tip > entered) return refused("The tip is more than the cash handed over.");
    const available = entered - tip;
    const amount = Math.min(available, due);
    if (amount <= 0) return refused("Enter the cash handed over.");
    if (whole && amount < due) return refused("A share is paid in one go: take at least the share.");
    return { amount, change: available - amount, charge: amount, problem: null };
  }
  const amount = whole ? due : Math.min(entered, due);
  if (amount <= 0) return refused("Enter the amount.");
  return { amount, change: 0, charge: amount + tip, problem: null };
}

export interface Attempt {
  key: string;
  body: TenderPayment;
}

/**
 * The idempotency key for a tender. A retry of the tender that last failed
 * reuses its key, so an answer lost on the way back cannot charge twice; any
 * change to the tender makes it a new one, which the server would otherwise
 * refuse as a reused key.
 */
export function attemptKey(last: Attempt | null, body: TenderPayment, newKey: () => string): Attempt {
  if (last && sameTender(last.body, body)) return last;
  return { key: newKey(), body };
}

function sameTender(a: TenderPayment, b: TenderPayment): boolean {
  return (
    a.method === b.method &&
    a.amount.minor === b.amount.minor &&
    a.amount.currency === b.amount.currency &&
    a.tip.minor === b.tip.minor &&
    (a.reference ?? "") === (b.reference ?? "")
  );
}

/** Why Charge is disabled on the register, or null when it is not. */
export function chargeBlock(order: OrderResponse | undefined, pending: number): string | null {
  if (!order || !order.lines.some((l) => l.status === "active")) return "Nothing to charge yet.";
  if (order.status === "closed") return "This ticket is closed.";
  if (order.status === "held") return "Resume this ticket to charge it.";
  if (pending > 0) return "Wait for every item to be added.";
  if (order.totals.balance_due.minor <= 0) return "Nothing is owed on this ticket.";
  return null;
}

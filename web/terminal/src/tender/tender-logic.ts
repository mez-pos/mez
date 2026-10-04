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

export const methodNames: Readonly<Record<PaymentMethod, string>> = {
  cash: "Cash",
  card: "Card",
  upi: "UPI",
  other: "Other",
};

/** The tips the payment screen offers in one tap (D07). */
export const tipPercents: readonly number[] = [5, 10, 15];

/**
 * A tip of so many percent of the bill, to the nearest whole unit of the
 * currency: 5% of ₹1,053.00 is ₹53.00, not ₹52.65. Nobody tips in paise.
 */
export function tipFor(bill: number, percent: number, currency: string): number {
  if (bill <= 0 || percent <= 0) return 0;
  const unit = 10 ** exponentOf(currency);
  return Math.round((bill * percent) / (100 * unit)) * unit;
}

/** Where a payment is, for the three steps along the top (D07). */
export type PayStep = "payment" | "split" | "complete";

export function payStep(paid: boolean, splitting: boolean): PayStep {
  if (paid) return "complete";
  return splitting ? "split" : "payment";
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

/**
 * Why Pay is disabled on the order screen, or null when it is not. A paid
 * order still open is not blocked: Pay then finishes it (chargeLabel).
 */
export function chargeBlock(order: OrderResponse | undefined, pending: number): string | null {
  if (!order || !order.lines.some((l) => l.status === "active")) return "Nothing to pay yet.";
  if (order.status === "closed") return "This order is closed.";
  if (order.status === "held") return "Resume this order to pay it.";
  if (pending > 0) return "Wait for every item to be added.";
  if (order.totals.balance_due.minor < 0) return "The guest is owed money back. Refunds come with Task 6.";
  return null;
}

/** Pay while money is owed; once paid, the same button finishes the order. */
export function chargeLabel(order: OrderResponse | undefined): string {
  if (!order || order.totals.balance_due.minor !== 0 || order.status === "closed") return "Pay";
  return order.invoiced ? "Close order" : "Invoice and close";
}

/**
 * What the guest hands over in all for this payment: the part of the bill it
 * settles and the tip. Before an amount is keyed it is the whole of what is
 * being paid, so the button names the sum to ask for.
 */
export function collectOf(plan: TenderPlan, target: number, tip: number): number {
  return (plan.problem === null ? plan.amount : Math.max(target, 0)) + tip;
}

/**
 * What the payment step shows (D08b). Cash is keyed and taken in one go. A
 * card or UPI payment is taken on the venue's own machine or code, so the
 * till waits for the cashier to say it went through; a refusal is the
 * server's, there. Money taken that leaves some owed is confirmed before the
 * next payment. A bill paid in full needs none of these: the paid panel says it.
 */
export type PayView = "entry" | "waiting" | "refused" | "received";

export interface PayViewInput {
  paid: boolean;
  /** A payment was just taken and has not been acknowledged. */
  received: boolean;
  /** A card or UPI payment is being taken away from the till. */
  waiting: boolean;
  failed: boolean;
}

export function payView({ paid, received, waiting, failed }: PayViewInput): PayView {
  if (paid) return "entry";
  if (received) return "received";
  if (waiting) return failed ? "refused" : "waiting";
  return "entry";
}

/** How long a payment has been waited for, as the design writes it: 0:24. */
export function elapsedClock(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

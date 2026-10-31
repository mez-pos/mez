// What the tender screen works out for itself (terminal-tender.md). Every
// other figure on it is the server's. These are the only sums: the change to
// hand back, the part of a cash payment that settles the bill, the round
// amounts a guest is likely to hand over, and the tip ceiling, which the
// server checks again. All in integer minor units.

import { apiErrorOf } from "../../../shared/api";
import type { OrderResponse, PaymentMethod, TenderRequest } from "../../../shared/api-types";
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

/**
 * The quick tenders a row of `room` chips shows. When there is one too many,
 * the exact amount and the largest stay, and the smallest step above exact
 * goes: someone paying a 200.00 bill hands over a 500 note far more often
 * than 210.00 (ADR-342).
 */
export function quickShown(quick: readonly number[], room: number): number[] {
  if (quick.length <= room) return [...quick];
  return [...quick.slice(0, 1), ...quick.slice(quick.length - (room - 1))];
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

/**
 * The largest tip a payment of `amount` may carry without a manager, at the
 * venue's ceiling of `percent` of the payment (ADR-261): rounded up to the
 * next minor unit, as the server rounds, so a tip at the ceiling is never
 * sent to a manager for a paisa the division lost. Integers only.
 */
export function tipCeiling(amount: number, percent: number): number {
  if (amount <= 0 || percent <= 0) return 0;
  return Math.floor((amount * percent + 99) / 100);
}

/**
 * Whether a tip needs a manager before it is sent. Null is a ceiling the till
 * has not read, and nothing keyed yet has no ceiling to judge by: in both the
 * server decides, and its refusal asks for the manager instead.
 */
export function tipNeedsApproval(tip: number, amount: number, percent: number | null): boolean {
  if (percent === null || tip <= 0 || amount <= 0) return false;
  return tip > tipCeiling(amount, percent);
}

/** What the tip card says of a tip over the ceiling, or null. */
export function tipNote(tip: number, amount: number, percent: number | null): string | null {
  if (!tipNeedsApproval(tip, amount, percent)) return null;
  return percent === 0 ? "Any tip needs a manager" : `Over ${percent}%: needs a manager`;
}

/** Whether a refusal is the server asking for a manager to approve the tip. */
export function tipApprovalAsked(err: unknown): boolean {
  return apiErrorOf(err)?.code === "tip_needs_approval";
}

/**
 * Whether a payment sent with a manager's approval was refused for the
 * approval itself: a wrong PIN, someone who may not approve, or too many
 * tries. The approval sheet then asks again; any other refusal is the
 * payment's, and the screen says it as it says one without an approval.
 */
export function approvalRefused(err: unknown): boolean {
  const e = apiErrorOf(err);
  if (!e) return false;
  return e.code === "invalid_pin" || e.code === "locked_out" || (e.code === "forbidden" && e.field === "authorised_by");
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
  if (due <= 0) return refused("Nothing is owed on this order.");
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
  body: TenderRequest;
}

/**
 * The idempotency key for a tender. A retry of the tender that last failed
 * reuses its key, so an answer lost on the way back cannot charge twice; any
 * change to the tender makes it a new one, which the server would otherwise
 * refuse as a reused key.
 */
export function attemptKey(last: Attempt | null, body: TenderRequest, newKey: () => string): Attempt {
  if (last && sameTender(last.body, body)) return last;
  return { key: newKey(), body };
}

function sameTender(a: TenderRequest, b: TenderRequest): boolean {
  return (
    a.method === b.method &&
    a.amount.minor === b.amount.minor &&
    a.amount.currency === b.amount.currency &&
    a.tip.minor === b.tip.minor &&
    (a.reference ?? "") === (b.reference ?? "")
  );
}

/**
 * How a tender that failed stands. A definite refusal is the server saying no
 * (a 4xx): nothing was recorded. Anything else, the network down, no answer
 * in time, a gateway giving up or the server failing part way, may have been
 * recorded with only the answer lost.
 */
export type TenderFailure = "refused" | "uncertain";

export function tenderFailure(err: unknown): TenderFailure {
  const status = apiErrorOf(err)?.status ?? null;
  return status !== null && status >= 400 && status < 500 ? "refused" : "uncertain";
}

/**
 * The attempt to keep after a failure. One that may have been recorded keeps
 * its key, method and amount until the order says it is not there, so a
 * second try can only replay it; a refusal lets it go.
 */
export function afterFailure(attempt: Attempt, err: unknown): Attempt | null {
  return tenderFailure(err) === "refused" ? null : attempt;
}

/**
 * Whether a tender whose answer was lost reached the order: a tender that
 * was not on it when this one was sent (`before`, its ids then), with the
 * same method, amount and tip. The server keeps no key on a tender, so these
 * are what identify it.
 */
export function tenderOutcome(before: readonly number[], body: TenderRequest, order: OrderResponse): "recorded" | "absent" {
  const known = new Set(before);
  const found = order.tenders.some(
    (t) =>
      !known.has(t.id) &&
      t.method === body.method &&
      t.amount.minor === body.amount.minor &&
      t.amount.currency === body.amount.currency &&
      t.tip.minor === body.tip.minor,
  );
  return found ? "recorded" : "absent";
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
  if (order.totals.balance_due.minor < 0) return "The guest is owed money back. A manager settles that on the Orders screen.";
  return null;
}

/**
 * Pay while money is owed; once paid, the same button finishes the order. An
 * order with nothing on it owes nothing either, but it is not paid: it reads
 * Pay, shut, never "Invoice and close" (TILL-12).
 */
export function chargeLabel(order: OrderResponse | undefined): string {
  if (!order || !order.lines.some((l) => l.status === "active")) return "Pay";
  if (order.totals.balance_due.minor !== 0 || order.status === "closed") return "Pay";
  return order.invoiced ? "Close order" : "Invoice and close";
}

/**
 * Whether the payment that settles an order also sends what is waiting and
 * closes it: a sale rung up at the counter with no table (ADR-383). The
 * server decides; the screen only says so, and must say what it does.
 */
export function closesWhenPaid(order: Pick<OrderResponse, "channel" | "table_id">): boolean {
  return order.channel === "counter" && !order.table_id;
}

/**
 * What the payment screen says of lines the kitchen has not seen (TILL-1):
 * on a counter sale they go with the payment; on a table's ticket they have
 * not gone, and Send now is beside the words.
 */
export function unsentWords(waiting: number, sentWithPayment: boolean): string | null {
  if (waiting <= 0) return null;
  const one = waiting === 1;
  const items = one ? "1 item" : `${waiting} items`;
  if (sentWithPayment) return `${items} ${one ? "goes" : "go"} to the kitchen with the payment.`;
  return `${items} ${one ? "has" : "have"} not gone to the kitchen.`;
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
 * next payment. A payment whose answer was lost holds the screen until the
 * order says whether it was recorded. A bill paid in full needs none of
 * these: the paid panel says it.
 */
export type PayView = "entry" | "waiting" | "refused" | "received" | "uncertain";

export interface PayViewInput {
  paid: boolean;
  /** A payment was just taken and has not been acknowledged. */
  received: boolean;
  /** A card or UPI payment is being taken away from the till. */
  waiting: boolean;
  failed: boolean;
  /** A payment was sent and its answer lost: it may have been recorded. */
  uncertain: boolean;
}

export function payView({ paid, received, waiting, failed, uncertain }: PayViewInput): PayView {
  if (paid) return "entry";
  if (uncertain) return "uncertain";
  if (received) return "received";
  if (waiting) return failed ? "refused" : "waiting";
  return "entry";
}

/** How long a payment has been waited for, as the design writes it: 0:24. */
export function elapsedClock(ms: number): string {
  const seconds = Math.max(0, Math.floor(ms / 1000));
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, "0")}`;
}

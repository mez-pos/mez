// The cart's rules without React (terminal-register.md, Task 4): what a line
// still allows, the reasons a manager picks from, and what a discount entry
// means before it is sent.

import type { DiscountRequest, LineResponse, Money, Override } from "../../../shared/api-types";

export interface Reason {
  readonly code: string;
  readonly label: string;
}

// Reason codes are stored on the event and read back by reports and the
// timeline (humanise turns wrong_item into "Wrong item"), so a code is never
// renamed once used. The back office will own these lists (phase 10).
export const voidReasons: readonly Reason[] = [
  { code: "wrong_item", label: "Wrong item" },
  { code: "entered_twice", label: "Entered twice" },
  { code: "guest_changed_mind", label: "Guest changed their mind" },
  { code: "kitchen_error", label: "Kitchen error" },
  { code: "walked_out", label: "Guest walked out" },
];

export const compReasons: readonly Reason[] = [
  { code: "long_wait", label: "Long wait" },
  { code: "quality", label: "Not right" },
  { code: "regular", label: "Regular" },
  { code: "staff_meal", label: "Staff meal" },
  { code: "manager_call", label: "Manager's call" },
];

export const discountReasons: readonly Reason[] = [
  { code: "regular", label: "Regular" },
  { code: "staff", label: "Staff" },
  { code: "complaint", label: "Complaint" },
  { code: "promotion", label: "Promotion" },
  { code: "manager_call", label: "Manager's call" },
];

/** What a line still allows. */
export interface LineActions {
  /** Quantity and modifiers: only before the kitchen has it (ADR-071, ADR-072). */
  readonly change: boolean;
  readonly note: boolean;
  /** Taken off with no manager: rung up by mistake, nothing done yet (ADR-073). */
  readonly remove: boolean;
  readonly void: boolean;
  readonly comp: boolean;
  readonly discount: boolean;
}

/** `paid` is whether the order has a payment that is not voided. */
export function lineActions(line: LineResponse, paid: boolean): LineActions {
  const active = line.status === "active";
  const unseen = active && !line.sent && !line.course_fired;
  return { change: unseen, note: true, remove: unseen && !paid, void: active, comp: active, discount: active };
}

/** A discount as typed: basis points (1250 is 12.5%) or an amount in minor units. */
export type DiscountEntry =
  | { readonly kind: "pct"; readonly basisPoints: number }
  | { readonly kind: "amount"; readonly minor: number };

/** 1250 as "12.5%", 1000 as "10%". */
export function formatPercent(basisPoints: number): string {
  const whole = Math.trunc(basisPoints / 100);
  const frac = basisPoints % 100;
  if (frac === 0) return `${whole}%`;
  return `${whole}.${String(frac).padStart(2, "0").replace(/0$/, "")}%`;
}

/**
 * Why a discount cannot be sent yet, in the till's words, or null. `worth`
 * is what it would come off: the line's value after its own discounts, or
 * the order's before order discounts. The server checks again.
 */
export function discountProblem(entry: DiscountEntry, worth: Money): string | null {
  if (entry.kind === "pct") {
    if (entry.basisPoints <= 0) return "Enter a percentage.";
    if (entry.basisPoints > 10000) return "A discount can't be more than 100%.";
    return null;
  }
  if (entry.minor <= 0) return "Enter an amount.";
  if (entry.minor > worth.minor) return "That's more than it's worth.";
  return null;
}

/** The request body for a discount, with the approval. */
export function discountRequest(
  scope: { readonly lineId: number } | "order",
  entry: DiscountEntry,
  currency: string,
  reason: string,
  override: Override,
): DiscountRequest {
  return {
    ...override,
    scope: scope === "order" ? "order" : "line",
    ...(scope === "order" ? {} : { line_id: scope.lineId }),
    kind: entry.kind,
    ...(entry.kind === "pct" ? { basis_points: entry.basisPoints } : { amount: { minor: entry.minor, currency } }),
    reason,
  };
}

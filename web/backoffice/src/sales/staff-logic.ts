// What the Staff reports screen works out (D17): one row a person from the
// report's bills by who opened them and its tips by who took the payment.
// Every figure is the server's (ADR-118, ADR-188); this joins two lists.

import type { Money, SalesReport, StaffTally } from "../../../shared/api-types";
import { averageBill } from "./sales-logic";

export interface StaffLine {
  /** The staff member's id, as the report keys them. */
  id: string;
  name: string;
  /** "" for someone no longer on record, and for a guest. */
  role: string;
  /** A bill no staff member opened: a guest's own order. */
  guest: boolean;
  /** The bills they opened. */
  bills: number;
  sales: Money;
  /** Their sales over their bills; null with no bills. */
  average: Money | null;
  /** The tips on the payments they took. */
  tips: Money;
}

/** The key of the bills no staff member opened. */
const guestKey = "0";

type Totals = Pick<SalesReport["totals"], "by_staff" | "tips_by" | "currency">;

/**
 * Everyone who opened a bill or took a tip, the largest sales first, then the
 * largest tips. Someone who only took payments has no bills and no sales of
 * their own: a bill counts for whoever opened it.
 */
export function staffLines(totals: Totals, names: Readonly<Record<string, string>>, roles: Readonly<Record<string, string>>): StaffLine[] {
  const zero: Money = { minor: 0, currency: totals.currency };
  const tips = new Map(totals.tips_by.map((t) => [String(t.staff_id), t.amount]));
  const opened = totals.by_staff.map((g) => ({
    id: g.key,
    // The report leaves a guest's own bills unnamed, under no staff member.
    name: g.name || names[g.key] || "A guest",
    role: roles[g.key] ?? "",
    guest: g.key === guestKey,
    bills: g.qty,
    sales: g.total,
    average: averageBill(g.total, g.qty),
    tips: tips.get(g.key) ?? zero,
  }));
  const seen = new Set(opened.map((l) => l.id));
  const tookOnly = totals.tips_by
    .map((t) => ({ t, id: String(t.staff_id) }))
    .filter(({ id }) => !seen.has(id))
    .map(({ t, id }) => ({
      id,
      name: names[id] ?? "Someone no longer on record",
      role: roles[id] ?? "",
      guest: false,
      bills: 0,
      sales: zero,
      average: null,
      tips: t.amount,
    }));
  return [...opened, ...tookOnly].sort((a, b) => b.sales.minor - a.sales.minor || b.tips.minor - a.tips.minor || a.name.localeCompare(b.name));
}

export interface RemovedLine {
  id: string;
  name: string;
  count: number;
  amount: Money;
}

/** Lines rung up in error, by who took them off, the most first. */
export function removedLines(removed: readonly StaffTally[], names: Readonly<Record<string, string>>): RemovedLine[] {
  return removed
    .map((t) => ({ id: String(t.staff_id), name: names[String(t.staff_id)] ?? "Someone no longer on record", count: t.count, amount: t.amount }))
    .sort((a, b) => b.amount.minor - a.amount.minor || b.count - a.count || a.name.localeCompare(b.name));
}

/**
 * How many people took tips. A row at or under nothing is someone whose tipped
 * payment from before these days was voided in them: they took none here.
 */
export function tipTakers(tips: readonly StaffTally[]): number {
  return tips.filter((t) => t.count > 0 && t.amount.minor > 0).length;
}

/** "3 lines", "1 line". */
export function linesText(n: number): string {
  return n === 1 ? "1 line" : `${n} lines`;
}

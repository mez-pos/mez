// The words and the order of a file of prices (backoffice-menu.md, "Change
// prices"). The server reads the file and says what it would change; this
// turns its answer into what the modal shows.

import type { MenuPriceChange, MenuPrices, Money } from "../../../shared/api-types";
import { count, type Verdict } from "../files/file-logic";

/** How many rows that are not on the menu are named; the rest are counted. */
const unknownNamed = 5;

/** What a checked file comes to: it has problems, it changes nothing, or it is ready to apply. */
export function priceVerdict(plan: MenuPrices): Verdict {
  if (plan.problems.length > 0) return "problems";
  return plan.changes.length > 0 ? "ready" : "nothing";
}

/** How far a price moves, as an amount to draw, and which way. */
export interface Difference {
  direction: "up" | "down";
  amount: Money;
}

export function differenceOf(c: MenuPriceChange): Difference {
  const by = c.new.minor - c.old.minor;
  return { direction: by >= 0 ? "up" : "down", amount: { minor: Math.abs(by), currency: c.new.currency } };
}

/**
 * The changes with the largest move first, whichever way it goes, so a
 * slipped digit is at the top of the table and not on its third screen.
 * Changes of one size keep the file's order.
 */
export function byLargestChange(changes: readonly MenuPriceChange[]): MenuPriceChange[] {
  const size = (c: MenuPriceChange) => Math.abs(c.new.minor - c.old.minor);
  return [...changes].sort((a, b) => size(b) - size(a));
}

/** What a file ready to apply would do, a line for each thing that applies. */
export function priceSummary(plan: MenuPrices): string[] {
  const lines = [`${count(plan.changes.length, "price", "prices")} to change.`];
  if (plan.unchanged === 1) lines.push("1 row keeps the price it has.");
  if (plan.unchanged > 1) lines.push(`${plan.unchanged} rows keep the price they have.`);
  return lines;
}

/** The rows that are no item on the menu, by name, and where they belong; null when there are none. */
export function unknownText(plan: MenuPrices): string | null {
  const n = plan.unknown.length;
  if (n === 0) return null;
  const named = plan.unknown.slice(0, unknownNamed).map((u) => u.name);
  const names = n > unknownNamed ? `${named.join(", ")} and ${n - unknownNamed} more` : named.join(", ");
  return n === 1
    ? `1 row is not on the menu and is left out: ${names}. To add it, use Import new items.`
    : `${n} rows are not on the menu and are left out: ${names}. To add them, use Import new items.`;
}

/** How many items the file would make free. Zero is a price, and usually a slip. */
export function freeCount(plan: MenuPrices): number {
  return plan.changes.filter((c) => c.new.minor === 0).length;
}

export function freeText(n: number): string {
  return n === 1 ? "1 item would be free." : `${n} items would be free.`;
}

export function applyLabel(plan: MenuPrices): string {
  return `Change ${count(plan.changes.length, "price", "prices")}`;
}

export function appliedText(plan: MenuPrices): string {
  return `Changed ${count(plan.changes.length, "price", "prices")}. Orders already open keep theirs.`;
}

export function priceProblemsText(plan: MenuPrices, filename: string): string {
  return `${count(plan.problems.length, "problem", "problems")} in ${filename}. No price has been changed.`;
}

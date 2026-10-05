// What the Inventory screen's table works out (D19): where each ingredient
// stands, how full its bar is, and the line under the title. On hand is the
// server's sum of the ledger; this only reads it.

import type { StockItem } from "../../../shared/api-types";
import type { StockLevel } from "../../../shared/ui/status";

/** Where an active ingredient stands: none left, at or under its low level, or neither. */
export function levelOf(item: Pick<StockItem, "on_hand" | "low">): StockLevel {
  if (item.on_hand <= 0) return "out";
  return item.low ? "low" : "ok";
}

/** How many times its low level a full bar is: an ingredient at three times its level reads as full. */
const fullAt = 3;

/**
 * How full an ingredient's bar is, 0 to 100, against three times its low
 * level; null when it has no low level, so there is nothing to measure it
 * against. A width on screen, not a figure.
 */
export function levelPercent(item: Pick<StockItem, "on_hand" | "low_level">): number | null {
  if (item.low_level <= 0) return null;
  if (item.on_hand <= 0) return 0;
  return Math.min(100, Math.max(2, Math.round((item.on_hand * 100) / (item.low_level * fullAt))));
}

export interface StockSummary {
  tracked: number;
  low: number;
  out: number;
}

/** The active ingredients, and how many are low and how many have run out. */
export function summary(items: readonly StockItem[]): StockSummary {
  const active = items.filter((it) => it.active);
  const levels = active.map(levelOf);
  return { tracked: active.length, low: levels.filter((l) => l === "low").length, out: levels.filter((l) => l === "out").length };
}

/** The line under the title: "55 tracked ingredients · 6 low · 1 out of stock". */
export function summaryText(s: StockSummary): string {
  if (s.tracked === 0) return "No ingredients yet. Add what you keep: beans, milk, batter.";
  const tracked = s.tracked === 1 ? "1 tracked ingredient" : `${s.tracked} tracked ingredients`;
  return `${tracked} · ${s.low} low · ${s.out} out of stock`;
}

/**
 * The table's rows: active ingredients first, in the server's order (by
 * name), then the retired ones.
 */
export function tableRows(items: readonly StockItem[], showRetired: boolean): StockItem[] {
  const active = items.filter((it) => it.active);
  return showRetired ? [...active, ...items.filter((it) => !it.active)] : active;
}

/** How many ingredients are retired, for the switch that shows them. */
export function retiredCount(items: readonly StockItem[]): number {
  return items.filter((it) => !it.active).length;
}

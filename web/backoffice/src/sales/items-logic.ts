// What the Item and inventory reports screen works out (D16): each item
// beside the days before, the write-offs as bars, and how the stock stands.
// Every figure is the server's (ADR-118, ADR-186); these are rows, heights
// and words.

import type { Diet, Group, MenuItemResponse, Money, StockItem, WasteItem, WasteReasonTotal, WasteReport } from "../../../shared/api-types";
import { change, type Change } from "../dashboard/dashboard-logic";
import { reasons } from "../stock/stock-logic";
import type { SalesRange } from "./sales-logic";

export interface ItemRow {
  key: string;
  name: string;
  /** "" for an item the menu no longer has, or one with no mark. */
  diet: Diet;
  qty: number;
  total: Money;
  /**
   * How many sold beside the days before; null when it did not sell then,
   * and undefined while those days are not read.
   */
  trend: Change | null | undefined;
}

/**
 * The items that took most, each with its mark from the menu as it stands
 * and how many sold beside the same number of days before.
 */
export function itemRows(
  items: readonly Group[],
  before: readonly Group[] | null,
  menu: readonly Pick<MenuItemResponse, "id" | "diet">[],
  limit: number,
): ItemRow[] {
  const diets = new Map(menu.map((m) => [String(m.id), m.diet]));
  const then = before ? new Map(before.map((g) => [g.key, g.qty])) : null;
  return [...items]
    .filter((g) => g.qty > 0)
    .sort((a, b) => b.total.minor - a.total.minor || b.qty - a.qty || a.name.localeCompare(b.name))
    .slice(0, limit)
    .map((g) => ({
      key: g.key,
      name: g.name,
      diet: diets.get(g.key) ?? "",
      qty: g.qty,
      total: g.total,
      trend: then ? change(g.qty, then.get(g.key) ?? 0) : undefined,
    }));
}

export interface WasteBar {
  item: WasteItem;
  /** Its bar against the largest loss, 0 to 100; 0 for an item with no cost. */
  percent: number;
  /** None of its entries had a cost: there is no figure to show. */
  notCosted: boolean;
}

/** The largest write-offs as bars, and how many items there was no room for. */
export function wasteBars(items: readonly WasteItem[], limit: number): { bars: WasteBar[]; more: number } {
  const top = Math.max(0, ...items.map((it) => it.value.minor));
  const bars = items.slice(0, limit).map((item) => ({
    item,
    percent: top > 0 && item.value.minor > 0 ? Math.max(2, Math.round((item.value.minor * 100) / top)) : 0,
    notCosted: item.uncosted >= item.entries,
  }));
  return { bars, more: items.length - bars.length };
}

/** A reason's word: "Spoiled". One this screen does not know is shown as the server spelt it. */
export function reasonName(reason: string): string {
  return reasons.find((r) => r.value === reason)?.label ?? reason;
}

/** The reason that cost most, or null when nothing was costed. */
export function worstReason(totals: readonly WasteReasonTotal[]): WasteReasonTotal | null {
  const first = totals[0];
  return first && first.value.minor > 0 ? first : null;
}

/** "3 entries", "1 entry". */
export function entriesText(n: number): string {
  return n === 1 ? "1 entry" : `${n} entries`;
}

/** What the total leaves out, or "" when every entry had a cost. */
export function wasteLeftOut(report: Pick<WasteReport, "uncosted">): string {
  if (report.uncosted <= 0) return "";
  return report.uncosted === 1 ? "1 entry had no cost and is not in the total." : `${report.uncosted} entries had no cost and are not in the total.`;
}

export interface StockHealth {
  healthy: number;
  low: number;
  out: number;
  /** What has run out, by name, for the line under the count. */
  outNames: string[];
}

/**
 * How the active stock stands: out is nothing on hand, whether or not it has
 * a low level set; low is at or under its level with some left.
 */
export function stockHealth(items: readonly StockItem[]): StockHealth {
  const active = items.filter((it) => it.active);
  const out = active.filter((it) => it.on_hand <= 0);
  const low = active.filter((it) => it.low && it.on_hand > 0);
  return { healthy: active.length - low.length - out.length, low: low.length, out: out.length, outNames: out.map((it) => it.name) };
}

/** "48 items", "1 item". */
export function itemsText(n: number): string {
  return n === 1 ? "1 item" : `${n} items`;
}

/** Up to three names, then how many more: "Milk, Tomato and 2 more". */
export function namesText(names: readonly string[], room = 3): string {
  if (names.length <= room) return names.join(", ");
  return `${names.slice(0, room - 1).join(", ")} and ${names.length - (room - 1)} more`;
}

/** The path the range's write-offs are read from. */
export function wastePath(range: SalesRange): string {
  return `/api/v1/reports/wastage?${new URLSearchParams({ from: range.from, to: range.to }).toString()}`;
}

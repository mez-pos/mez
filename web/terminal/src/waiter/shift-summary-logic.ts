// What the shift summary (T08) says besides the server's figures, which come
// whole from GET /api/v1/shift/mine: the hours of the shift, how long the
// person has been at it, and the tips by table as bars. No money is added here.

import type { Money, MyShiftResponse, TableTipsResponse } from "../../../shared/api-types";
import { clockText } from "../../../shared/venue-text";
import { elapsed } from "../register/ticket-time";

/** "Sat, 19 Sep 2026 · 4:00 PM – 11:30 PM": the shift's day, and from its opening until now. */
export function shiftSpan(openedAt: string, now: number, timezone: string | null): string {
  const [, until = ""] = clockText(new Date(now), timezone).split(" · ");
  return `${clockText(new Date(openedAt), timezone)} – ${until}`;
}

/**
 * "On shift · 7h 26m". Nobody clocks in (no timesheets), so it runs from the
 * first order they touched, or from the shift's opening before they touch one.
 */
export function onShiftText(mine: MyShiftResponse, now: number): string {
  return `On shift · ${elapsed(mine.first_at ?? mine.opened_at, now, 0).text}`;
}

export interface TipBar {
  tableId: number;
  /** "T4", or "No table" for orders that had none. */
  label: string;
  tips: Money;
  /** The bar's length against the largest, 0 to 100. */
  percent: number;
}

export function tipBars(byTable: readonly TableTipsResponse[], names: ReadonlyMap<number, string>): TipBar[] {
  const most = Math.max(0, ...byTable.map((t) => t.tips.minor));
  return byTable.map((t) => ({
    tableId: t.table_id,
    label: t.table_id === 0 ? "No table" : (names.get(t.table_id) ?? `Table ${t.table_id}`),
    tips: t.tips,
    percent: most > 0 && t.tips.minor > 0 ? Math.round((t.tips.minor * 100) / most) : 0,
  }));
}

// What the Sales reports screen works out for itself (backoffice-reports.md).
// Every figure is the server's (ADR-118); this builds the queries, picks how
// a range is cut, and turns figures into a bar's height or a share.

import type { Money, ReportEvery, ReportSheet, SalesPoint } from "../../../shared/api-types";
import { dayText, lastDays, tradingDay } from "../audit/audit-logic";

export interface SalesRange {
  from: string;
  to: string;
}

export type QuickRange = "today" | "week" | "month" | "this_month";

export const quickRanges: readonly { id: QuickRange; label: string }[] = [
  { id: "today", label: "Today" },
  { id: "week", label: "7 days" },
  { id: "month", label: "30 days" },
  { id: "this_month", label: "This month" },
];

/** A quick range as trading days, today included. */
export function quickRange(id: QuickRange, now: Date): SalesRange {
  switch (id) {
    case "today":
      return lastDays(now, 1);
    case "week":
      return lastDays(now, 7);
    case "month":
      return lastDays(now, 30);
    case "this_month": {
      const to = tradingDay(now);
      return { from: `${to.slice(0, 8)}01`, to };
    }
  }
}

/** The quick range these dates are, or null for a range typed by hand. */
export function quickRangeOf(range: SalesRange, now: Date): QuickRange | null {
  for (const q of quickRanges) {
    const r = quickRange(q.id, now);
    if (r.from === range.from && r.to === range.to) return q.id;
  }
  return null;
}

const dayMs = 24 * 60 * 60 * 1000;
const utc = (day: string): number => {
  const [y, m, d] = day.split("-").map(Number) as [number, number, number];
  return Date.UTC(y, m - 1, d);
};

/** How many trading days a range covers, both ends included; 0 when it is not a range. */
export function daysIn(range: SalesRange): number {
  const n = Math.round((utc(range.to) - utc(range.from)) / dayMs) + 1;
  return Number.isFinite(n) && n > 0 ? n : 0;
}

/** How a range is cut when nobody has chosen: a day up to 31 days, a week up to 26 weeks, a month beyond. */
export function everyFor(range: SalesRange): ReportEvery {
  const days = daysIn(range);
  if (days <= 31) return "day";
  return days <= 26 * 7 ? "week" : "month";
}

export const everyWords: Readonly<Record<ReportEvery, string>> = { day: "Day", week: "Week", month: "Month" };

const query = (range: SalesRange, every: ReportEvery): URLSearchParams => new URLSearchParams({ from: range.from, to: range.to, every });

/** The path the range's reports are read from. */
export function salesPath(range: SalesRange, every: ReportEvery): string {
  return `/api/v1/reports/sales?${query(range, every).toString()}`;
}

/** The path one of the range's reports is downloaded from as CSV. */
export function csvPath(range: SalesRange, every: ReportEvery, sheet: ReportSheet): string {
  const p = query(range, every);
  p.set("report", sheet);
  return `/api/v1/reports/sales/csv?${p.toString()}`;
}

/** A period in words: "1 Oct" for a day, "28 Sept to 4 Oct" for longer. */
export function periodText(p: Pick<SalesPoint, "start" | "end">): string {
  return p.start === p.end ? dayText(p.start) : `${dayText(p.start)} to ${dayText(p.end)}`;
}

/**
 * Each period's bar as a percentage of the tallest, 0 to 100. A height on
 * screen, not money: nothing is added up or rounded into a figure here.
 */
export function barHeights(series: readonly SalesPoint[]): number[] {
  const top = Math.max(0, ...series.map((p) => p.sales.total.minor));
  return series.map((p) => (top > 0 && p.sales.total.minor > 0 ? Math.max(1, Math.round((p.sales.total.minor * 100) / top)) : 0));
}

/**
 * Each row's share of the rows' sum as a whole percentage, or null when
 * there is nothing sensible to divide: no sum, or a negative row (a method
 * that refunded more than it took).
 */
export function shares(minors: readonly number[]): number[] | null {
  const sum = minors.reduce((a, b) => a + b, 0);
  if (sum <= 0 || minors.some((m) => m < 0)) return null;
  return minors.map((m) => Math.round((m * 100) / sum));
}

/** The average bill: the total over the bills, to the nearest minor unit; null with no bills. */
export function averageBill(total: Money, bills: number): Money | null {
  if (bills <= 0) return null;
  return { ...total, minor: Math.round(total.minor / bills) };
}

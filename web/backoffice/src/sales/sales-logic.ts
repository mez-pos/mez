// What the Sales reports screen works out for itself (backoffice-reports.md).
// Every figure is the server's (ADR-118); this builds the queries, picks how
// a range is cut, and turns figures into a bar's height or a share.

import type { AuditAnalysisSheet, Group, ReportEvery, ReportSheet, SalesHour, SalesPoint } from "../../../shared/api-types";
import { averageMoney, formatMoney } from "../../../shared/money";
import type { Sort } from "../../../shared/ui/table-state";
import { dayShort, lastDays, tradingDay, type VenueClock } from "../../../shared/venue-clock";

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

/** A quick range as the venue's trading days, today included (ADR-385). */
export function quickRange(id: QuickRange, now: Date, clock: VenueClock): SalesRange {
  switch (id) {
    case "today":
      return lastDays(now, 1, clock);
    case "week":
      return lastDays(now, 7, clock);
    case "month":
      return lastDays(now, 30, clock);
    case "this_month": {
      const to = tradingDay(now, clock);
      return { from: `${to.slice(0, 8)}01`, to };
    }
  }
}

/** The quick range these dates are, or null for a range typed by hand. */
export function quickRangeOf(range: SalesRange, now: Date, clock: VenueClock): QuickRange | null {
  for (const q of quickRanges) {
    const r = quickRange(q.id, now, clock);
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

/** A period in words: "1 Oct" for a day, "28 Sep to 4 Oct" for longer. */
export function periodText(p: Pick<SalesPoint, "start" | "end">): string {
  return p.start === p.end ? dayShort(p.start) : `${dayShort(p.start)} to ${dayShort(p.end)}`;
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
export const averageBill = averageMoney;

/** The columns a table of groups sorts by. */
export type GroupSortColumn = "name" | "qty" | "total";

/** A table of groups as it opens: by total, largest first. */
export const defaultGroupSort: Sort = { column: "total", direction: "desc" };

/**
 * The server's rows in another order; no figure changes. Rows that tie keep
 * the order the server gave them.
 */
export function sortGroups(groups: readonly Group[], sort: Sort): Group[] {
  const by: Record<GroupSortColumn, (a: Group, b: Group) => number> = {
    name: (a, b) => a.name.localeCompare(b.name),
    qty: (a, b) => a.qty - b.qty,
    total: (a, b) => a.total.minor - b.total.minor,
  };
  const compare = by[sort.column as GroupSortColumn] ?? by.total;
  const turn = sort.direction === "asc" ? 1 : -1;
  return [...groups].sort((a, b) => turn * compare(a, b));
}

/**
 * Where an hour of the clock falls in the trading day that starts in
 * startHour (the venue's, dayStartHour): from 04:00, 0 for 04:00 and 23 for 03:00.
 */
export const slotOf = (hour: number, startHour: number): number => (hour - startHour + 24) % 24;

const weekdayNames = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"] as const;

/** A weekday as the server numbers it, 1 for Monday to 7 for Sunday. */
export function weekdayName(weekday: number): string {
  return weekdayNames[weekday - 1] ?? `Day ${weekday}`;
}

/** An hour of the clock: "13:00". */
export function hourText(hour: number): string {
  return `${String(hour).padStart(2, "0")}:00`;
}

export interface HeatRow {
  weekday: number;
  /** One per column of the grid; null where there was no bill. */
  cells: (SalesHour | null)[];
}

export interface HeatGrid {
  /** The hours of the clock across the top, in the order the trading day runs them. */
  hours: number[];
  /** All seven weekdays, Monday first. */
  rows: HeatRow[];
  /** The cell with the largest total, the earliest when two tie; null with no bills. */
  busiest: SalesHour | null;
}

/**
 * The heatmap's cells laid out: every weekday, and each hour from the first
 * with a bill to the last, counted from the start of the trading day so
 * 01:00 comes after the evening it belongs to.
 */
export function heatGrid(cells: readonly SalesHour[], startHour: number): HeatGrid {
  const live = cells.filter((c) => c.bills > 0);
  if (live.length === 0) return { hours: [], rows: [], busiest: null };
  const slots = live.map((c) => slotOf(c.hour, startHour));
  const first = Math.min(...slots);
  const last = Math.max(...slots);
  const hours = Array.from({ length: last - first + 1 }, (_, i) => (first + i + startHour) % 24);
  const rows = weekdayNames.map((_, i) => ({
    weekday: i + 1,
    cells: hours.map((h) => live.find((c) => c.weekday === i + 1 && c.hour === h) ?? null),
  }));
  let busiest: SalesHour | null = null;
  for (const row of rows) {
    for (const c of row.cells) {
      if (c && (!busiest || c.total.minor > busiest.total.minor)) busiest = c;
    }
  }
  return { hours, rows, busiest };
}

/**
 * How strongly a cell is tinted, in four steps: 0.25, 0.5, 0.75 or 1 of the
 * busiest cell's total. A tint, not a figure.
 */
export function tint(total: number, top: number): number {
  if (top <= 0 || total <= 0) return 0.25;
  return Math.min(4, Math.max(1, Math.ceil((total * 4) / top))) / 4;
}

/** A cell in words: "Friday 13:00, 3 bills, ₹630.00". */
export function cellText(c: SalesHour): string {
  return `${weekdayName(c.weekday)} ${hourText(c.hour)}, ${c.bills} ${c.bills === 1 ? "bill" : "bills"}, ${formatMoney(c.total)}`;
}

/** The path the range's corrections are read from, added up (ADR-122). */
export function analysisPath(range: SalesRange): string {
  return `/api/v1/audit/analysis?${new URLSearchParams({ from: range.from, to: range.to }).toString()}`;
}

/** The path one grouping of the range's corrections is downloaded from as CSV. */
export function analysisCsvPath(range: SalesRange, by: AuditAnalysisSheet): string {
  return `/api/v1/audit/analysis/csv?${new URLSearchParams({ from: range.from, to: range.to, by }).toString()}`;
}

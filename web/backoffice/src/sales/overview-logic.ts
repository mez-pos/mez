// What the top of the Sales reports screen works out (D15): the range before
// the one shown, a round axis for the trend and each period's bar cut by how
// it was served, and the slices of a ring. Every figure is the server's
// (ADR-118); these are heights, shares and words.

import type { Group, Money, SalesPoint, ServiceType } from "../../../shared/api-types";
import { axisTop } from "../dashboard/dashboard-logic";
import { dayText } from "../audit/audit-logic";
import { daysIn, type SalesRange } from "./sales-logic";

const pad = (n: number) => String(n).padStart(2, "0");
const parts = (day: string) => day.split("-").map(Number) as [number, number, number];
const dayOf = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/** The first day a range may start: a year half typed ("0002-10-05") is not a range to ask the server for. */
const earliest = "2000-01-01";

/** Whether these are days worth asking for: a first and a last day, in order, in this century. */
export function usable(range: SalesRange): boolean {
  return daysIn(range) > 0 && range.from >= earliest;
}

/** The same number of days, ending the day before this range starts; null for a range that is not one. */
export function previousRange(range: SalesRange): SalesRange | null {
  if (!usable(range)) return null;
  const days = daysIn(range);
  const [y, m, d] = parts(range.from);
  return { from: dayOf(new Date(y, m - 1, d - days)), to: dayOf(new Date(y, m - 1, d - 1)) };
}

/** A range as its button says it: "1 Sept – 19 Sept 2026", or "5 Oct 2026" for one day. */
export function rangeText(range: SalesRange): string {
  const year = range.to.slice(0, 4);
  if (range.from === range.to) return `${dayText(range.to)} ${year}`;
  const fromYear = range.from.slice(0, 4);
  return `${dayText(range.from)}${fromYear === year ? "" : ` ${fromYear}`} – ${dayText(range.to)} ${year}`;
}

/** The line under the title: "Insight into 19 days of trading". */
export function insightText(range: SalesRange): string {
  const days = daysIn(range);
  if (!usable(range)) return "Pick a first and a last day";
  return days === 1 ? "Insight into one day of trading" : `Insight into ${days} days of trading`;
}

/** What a figure is compared with: "the 7 days before". */
export function beforeText(range: SalesRange): string {
  const days = daysIn(range);
  return days === 1 ? "the day before" : `the ${days} days before`;
}

/** How an order is served, in the order the charts stack and list them. */
export const services: readonly { id: ServiceType; name: string }[] = [
  { id: "dine_in", name: "Dine-in" },
  { id: "takeaway", name: "Takeaway" },
  { id: "delivery", name: "Delivery" },
];

/** A service type's name; one this screen does not know is shown as the server spelt it. */
export function serviceName(key: string): string {
  return services.find((s) => s.id === key)?.name ?? key;
}

export interface TrendBar {
  start: string;
  end: string;
  total: Money;
  /** One per service, in `services` order: its part of the period and its height against the axis, 0 to 100. */
  parts: { id: ServiceType; total: Money | null; percent: number }[];
}

export interface Trend {
  bars: TrendBar[];
  /** The top of the axis, in minor units. */
  top: number;
}

/**
 * Each period's bar, stacked by how its sales were served, against a round
 * axis. Heights on screen, not money: a part too small to see is given 1%.
 */
export function trend(series: readonly SalesPoint[], currency: string): Trend {
  const top = axisTop(Math.max(0, ...series.map((p) => p.sales.total.minor)), currency);
  const bars = series.map((p) => ({
    start: p.start,
    end: p.end,
    total: p.sales.total,
    parts: services.map(({ id }) => {
      const total = p.by_service.find((g) => g.key === id)?.total ?? null;
      const minor = total?.minor ?? 0;
      return { id, total, percent: minor > 0 ? Math.max(1, Math.round((minor * 100) / top)) : 0 };
    }),
  }));
  return { bars, top };
}

/**
 * Which bars are named under the chart: about seven, evenly spaced, the first
 * always. Every bar is named when there are few.
 */
export function namedBars(count: number, room = 7): boolean[] {
  const step = Math.max(1, Math.ceil(count / room));
  return Array.from({ length: count }, (_, i) => i % step === 0);
}

export interface Slice {
  key: string;
  name: string;
  /** The row's own figure: minor units, or bills. */
  value: number;
  /** A whole percentage; a ring's slices come to 100. */
  percent: number;
}

/** The key of the slice the smaller rows are folded into. */
export const othersKey = "__others";

/**
 * The slices of a ring: the largest rows, and the rest as "Others" once there
 * are more than `limit`. Rows with nothing, or less, are left out. The
 * percentages are whole and come to exactly 100 (largest remainder), so the
 * ring closes and the legend adds up.
 */
export function slices(rows: readonly { key: string; name: string; value: number }[], limit: number): Slice[] {
  const live = rows.filter((r) => r.value > 0).sort((a, b) => b.value - a.value);
  const kept = live.length > limit ? live.slice(0, limit - 1) : live;
  const rest = live.slice(kept.length).reduce((sum, r) => sum + r.value, 0);
  const shown = rest > 0 ? [...kept, { key: othersKey, name: "Others", value: rest }] : kept;
  const whole = shown.reduce((sum, r) => sum + r.value, 0);
  if (whole <= 0) return [];

  const exact = shown.map((r) => (r.value * 100) / whole);
  const floors = exact.map(Math.floor);
  const spare = 100 - floors.reduce((a, b) => a + b, 0);
  // The spare points go to the rows that lost most by rounding down.
  const order = exact.map((e, i) => ({ i, lost: e - Math.floor(e) })).sort((a, b) => b.lost - a.lost || a.i - b.i);
  const extra = new Set(order.slice(0, spare).map((o) => o.i));
  return shown.map((r, i) => ({ ...r, percent: (floors[i] ?? 0) + (extra.has(i) ? 1 : 0) }));
}

/** Sales groups as a ring's rows, by what they took. */
export function byTotal(groups: readonly Group[], name: (g: Group) => string = (g) => g.name) {
  return groups.map((g) => ({ key: g.key, name: name(g), value: g.total.minor }));
}

/** Sales groups as a ring's rows, by how many bills. */
export function byBills(groups: readonly Group[], name: (g: Group) => string = (g) => g.name) {
  return groups.map((g) => ({ key: g.key, name: name(g), value: g.qty }));
}

/**
 * A ring as a CSS conic gradient: each slice from where the last ended, in
 * its colour. `colours` are CSS colours, one a slice; a ring with more slices
 * than colours repeats the last.
 */
export function ringGradient(parts: readonly Pick<Slice, "percent">[], colours: readonly string[]): string {
  let at = 0;
  const stops = parts.map((s, i) => {
    const from = at;
    at += s.percent;
    return `${colours[Math.min(i, colours.length - 1)] ?? "transparent"} ${from}% ${at}%`;
  });
  return `conic-gradient(${stops.join(", ")})`;
}

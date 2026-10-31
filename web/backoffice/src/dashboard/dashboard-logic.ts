// What the dashboard works out for itself (D12). Every figure is the server's:
// the day's sales report (ADR-118), the floor in service and the stock list.
// This compares two days, picks the rows a card has room for, and turns totals
// into the height of a bar.

import type { Group, LiveTableResponse, Money, SalesHour, StockItem, TableStatus } from "../../../shared/api-types";
import type { StockLevel } from "../../../shared/ui/status";
import { exponentOf } from "../../../shared/money";
import { slotOf } from "../sales/sales-logic";

export interface Change {
  direction: "up" | "down" | "same";
  /** A whole percentage of yesterday's figure, never negative. */
  percent: number;
}

/** Today beside yesterday; null when yesterday had nothing to compare with. */
export function change(today: number, yesterday: number): Change | null {
  if (yesterday <= 0) return null;
  const percent = Math.round((Math.abs(today - yesterday) * 100) / yesterday);
  if (percent === 0) return { direction: "same", percent: 0 };
  return { direction: today > yesterday ? "up" : "down", percent };
}

/** What the dashboard compares today so far with: the whole of the day before. */
export const yesterday = "all of yesterday";

/** A change in words: "Up 12% on all of yesterday", "Down 4% on the 7 days before". */
export function changeText(c: Change | null, against: string = yesterday): string {
  if (!c) return "Nothing before to compare";
  if (c.direction === "same") return `Same as ${against}`;
  return `${c.direction === "up" ? "Up" : "Down"} ${c.percent}% on ${against}`;
}

export interface Occupancy {
  active: number;
  total: number;
  /** A whole percentage; 0 for a venue with no tables. */
  percent: number;
}

/** How much of the floor is in use: every table that is not free. */
export function occupancy(tables: readonly Pick<LiveTableResponse, "status">[]): Occupancy {
  const active = tables.filter((t) => t.status !== "free").length;
  const total = tables.length;
  return { active, total, percent: total > 0 ? Math.round((active * 100) / total) : 0 };
}

export interface TableRow {
  id: number;
  name: string;
  covers: number;
  status: TableStatus;
  /** How long the party has been there, or "" when the floor does not say. */
  since: string;
}

/**
 * The tables in use, the one that has sat longest first. A table joined to
 * another is left out: its party is listed at the table it was joined to.
 */
export function activeTables(tables: readonly LiveTableResponse[], now: number, limit: number): { rows: TableRow[]; more: number } {
  const live = tables
    .filter((t) => t.status !== "free" && t.joined_to === undefined)
    .map((t) => ({ t, at: t.since ? Date.parse(t.since) : Number.NaN }))
    // A table with no start time (one left to clear) goes after those that have one.
    .sort((a, b) => (Number.isNaN(a.at) ? 1 : 0) - (Number.isNaN(b.at) ? 1 : 0) || a.at - b.at || a.t.name.localeCompare(b.t.name));
  const rows = live.slice(0, limit).map(({ t, at }) => ({
    id: t.id,
    name: t.name,
    covers: t.covers,
    status: t.status,
    since: Number.isNaN(at) ? "" : sinceText(now - at),
  }));
  return { rows, more: live.length - rows.length };
}

/** A wait in words: "45 min", "1 h 05 min". Under a minute is "Just now". */
export function sinceText(ms: number): string {
  const minutes = Math.floor(Math.max(0, ms) / 60_000);
  if (minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes} min`;
  return `${Math.floor(minutes / 60)} h ${String(minutes % 60).padStart(2, "0")} min`;
}

/** "4 guests", "1 guest"; a party nobody counted is "No guest count". */
export function guestsText(covers: number): string {
  if (covers <= 0) return "No guest count";
  return covers === 1 ? "1 guest" : `${covers} guests`;
}

/** The items that sold most, by how many, then by what they took. The server's rows in another order. */
export function topItems(items: readonly Group[], limit: number): Group[] {
  return [...items]
    .filter((g) => g.qty > 0)
    .sort((a, b) => b.qty - a.qty || b.total.minor - a.total.minor || a.name.localeCompare(b.name))
    .slice(0, limit);
}

export interface StaffRow {
  id: string;
  name: string;
  /** "" for someone no longer on record. */
  role: string;
  bills: number;
  total: Money;
}

/**
 * Who opened the day's bills, the largest sales first (the server's order).
 * The report names each person itself and says what role they hold now;
 * someone no longer on record has no role.
 */
export function staffRows(groups: readonly Group[], roles: Readonly<Record<string, string>>, limit: number): StaffRow[] {
  return groups.slice(0, limit).map((g) => ({ id: g.key, name: g.name, role: roles[g.key] ?? "", bills: g.qty, total: g.total }));
}

/** The hours the chart always shows, so a quiet morning is not one wide bar: 8 AM to 10 PM. */
const chartOpens = 8;
const chartCloses = 22;

export interface HourBar {
  /** The hour of the clock, 0 to 23. */
  hour: number;
  bills: number;
  /** Null for an hour with no bill. */
  total: Money | null;
  /** The bar's height against the axis, 0 to 100. A height on screen, not money. */
  percent: number;
}

export interface HourChart {
  bars: HourBar[];
  /** The top of the axis, in minor units: a round figure at or above the best hour. */
  top: number;
}

/**
 * One trading day's bills by the hour, laid out from 8 AM to 10 PM, or wider
 * when a bill fell outside that. The hours run as the trading day does, so
 * 1 AM comes after the evening it belongs to.
 */
export function hourChart(cells: readonly SalesHour[], currency: string, startHour: number): HourChart {
  const live = cells.filter((c) => c.bills > 0);
  const slots = live.map((c) => slotOf(c.hour, startHour));
  const first = Math.min(slotOf(chartOpens, startHour), ...slots);
  const last = Math.max(slotOf(chartCloses, startHour), ...slots);
  const top = axisTop(Math.max(0, ...live.map((c) => c.total.minor)), currency);
  const bars = Array.from({ length: last - first + 1 }, (_, i) => {
    const hour = (first + i + startHour) % 24;
    // A day's range has one weekday; were there two, an hour's cells are shown as the first.
    const cell = live.find((c) => c.hour === hour);
    const minor = cell?.total.minor ?? 0;
    return {
      hour,
      bills: cell?.bills ?? 0,
      total: cell?.total ?? null,
      percent: minor > 0 ? Math.max(1, Math.round((minor * 100) / top)) : 0,
    };
  });
  return { bars, top };
}

/**
 * The top of an axis with four even steps: the smallest of 1, 2, 2.5 and 5
 * times a power of ten, in whole major units, whose four steps reach `max`.
 * 3,400.00 gives 4,000.00 (steps of 1,000); nothing gives 4.00.
 */
export function axisTop(maxMinor: number, currency: string): number {
  const unit = 10 ** exponentOf(currency);
  const need = Math.ceil(Math.max(0, maxMinor) / unit);
  for (let power = 1; ; power *= 10) {
    for (const m of [1, 2, 2.5, 5]) {
      const step = m * power;
      if (Number.isInteger(step) && step * 4 >= need) return step * 4 * unit;
    }
  }
}

/** The axis's five ticks, top first, in minor units. */
export function axisTicks(top: number): number[] {
  return [4, 3, 2, 1, 0].map((n) => (top / 4) * n);
}

/** An hour of the clock as the chart labels it: "8 AM", "12 PM", "12 AM". */
export function hourLabel(hour: number): string {
  const h = hour % 12 === 0 ? 12 : hour % 12;
  return `${h} ${hour < 12 ? "AM" : "PM"}`;
}

export interface LowRow {
  item: StockItem;
  level: StockLevel;
}

/**
 * What is at or under its level, or has none left whether or not it has a
 * level (as the Inventory screen counts "out"), anything with none left
 * first; the rest in the server's order.
 */
export function lowRows(items: readonly StockItem[], limit: number): { rows: LowRow[]; more: number } {
  const low = items
    .filter((it) => it.active && (it.low || it.on_hand <= 0))
    .map((item): LowRow => ({ item, level: item.on_hand <= 0 ? "out" : "low" }))
    .sort((a, b) => (a.level === "out" ? 0 : 1) - (b.level === "out" ? 0 : 1));
  const rows = low.slice(0, limit);
  return { rows, more: low.length - rows.length };
}

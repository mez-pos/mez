// What the waiter's home works out (T01): the four groups its tables fall in,
// the count on each pill, the tables a pill and the search leave, each card's
// small print, and the greeting. The tables and their states are the server's
// (ADR-086); the floor's own logic lists and names them (floor-logic.ts).

import type { LiveFloorResponse, LiveTableResponse, TableStatus } from "../../../shared/api-types";
import type { ChipTone } from "../../../shared/ui";
import { tableChip, tablePaid, tableStatuses } from "../../../shared/ui/status";
import { listRows, tileText } from "../floor/floor-logic";

/**
 * A table as a waiter sorts it, in the till's own words, so the tablet and
 * the till say the same of a table (TBS-11, ADR-381, ADR-394). Seated and
 * Ordered are apart: "Occupied" hid a table that had not ordered.
 */
export type TableGroup = "seated" | "ordered" | "paying" | "clear" | "free";

/** A pill of the home: every table, or one group. */
export type TableFilter = "all" | TableGroup;

const groupByStatus: Readonly<Record<TableStatus, TableGroup>> = {
  free: "free",
  seated: "seated",
  ordered: "ordered",
  settling: "paying",
  dirty: "clear",
};

export function groupOf(status: TableStatus): TableGroup {
  return groupByStatus[status];
}

const words = (status: TableStatus) => ({ label: tableStatuses[status].word, tone: tableStatuses[status].tone });

/** A group's words and tone: the till's for the same turn. Reserved is not a state here. */
export const tableGroups: Readonly<Record<TableGroup, { readonly label: string; readonly tone: ChipTone }>> = {
  seated: words("seated"),
  ordered: words("ordered"),
  paying: words("settling"),
  clear: words("dirty"),
  free: words("free"),
};

/**
 * A table's group, by its turn and what it owes: one settling with nothing
 * owed is paid, and the party may still be at it (ADR-372). It is with the
 * tables that have ordered, never Paying, which would have the bill asked
 * for twice (ADR-383); its card says Paid.
 */
export function tableGroupOf(t: Pick<LiveTableResponse, "status" | "balance">): TableGroup {
  return tablePaid(t) ? "ordered" : groupOf(t.status);
}

/** A card's word and tone: the till's chip, Paid for a table paid in full. */
export function cardWords(t: Pick<LiveTableResponse, "status" | "balance">): { readonly label: string; readonly tone: ChipTone } {
  const chip = tableChip(t.status, tablePaid(t));
  return { label: chip.word, tone: chip.tone };
}

/** The groups in the order of their pills: what a waiter does next, then the free tables. */
export const groupOrder: readonly TableGroup[] = ["seated", "ordered", "paying", "clear", "free"];

export function filterCounts(tables: readonly LiveTableResponse[]): Record<TableFilter, number> {
  const count = (group: TableGroup) => tables.filter((t) => tableGroupOf(t) === group).length;
  return {
    all: tables.length,
    seated: count("seated"),
    ordered: count("ordered"),
    paying: count("paying"),
    clear: count("clear"),
    free: count("free"),
  };
}

/** "t 10" finds "T10": a table is asked for by its number, in a hurry. */
const squash = (text: string) => text.toLowerCase().replace(/\s+/g, "");

/** The cards to draw: every area's tables in the floor's order, less what the pill and the search leave out. */
export function tablesShown(floor: LiveFloorResponse, filter: TableFilter, query: string): LiveTableResponse[] {
  const wanted = squash(query);
  return listRows(floor, floor.areas)
    .filter((t) => filter === "all" || tableGroupOf(t) === filter)
    .filter((t) => wanted === "" || squash(t.name).includes(wanted));
}

/** A card's small print, before its balance, which is shown apart as money. */
export function cardMeta(t: LiveTableResponse, names: ReadonlyMap<number, string>): string {
  const { people } = tileText(t, 0, names);
  if (t.status === "free") return `No guests · ${people}`;
  if (t.status === "seated" && !t.joined_to) return `${people} · No order yet`;
  return people;
}

export interface Greeting {
  readonly hello: string;
  /** How the line under it names this service: "tonight", "this morning". */
  readonly when: string;
}

/** The greeting for an hour of the day, 0 to 23. Before 05:00 is still last night's service. */
export function greeting(hour: number): Greeting {
  if (hour >= 5 && hour < 12) return { hello: "Good morning", when: "this morning" };
  if (hour >= 12 && hour < 17) return { hello: "Good afternoon", when: "this afternoon" };
  return { hello: "Good evening", when: "tonight" };
}

/** The hour in the venue's zone, or the device's own when there is none or the browser does not know it. */
export function hourIn(at: Date, timezone: string | null): number {
  if (!timezone) return at.getHours();
  try {
    const text = new Intl.DateTimeFormat("en-US", { hour: "numeric", hourCycle: "h23", timeZone: timezone }).format(at);
    const hour = Number.parseInt(text, 10);
    return Number.isInteger(hour) ? hour % 24 : at.getHours();
  } catch {
    return at.getHours();
  }
}

/** "Kabir Singh" is greeted as "Kabir". */
export function firstName(name: string): string {
  return name.trim().split(/\s+/)[0] ?? "";
}

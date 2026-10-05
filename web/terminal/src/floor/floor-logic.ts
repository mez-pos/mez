// What the tables screen works out (D05): an area as rows of tiles, what the
// panel shows for the table picked, the counts, the table list's rows and each
// tile's small print. The tables and their states are the server's (ADR-086).

import type { AreaResponse, LiveFloorResponse, LiveTableResponse, TableStatus } from "../../../shared/api-types";
import { tableStatuses } from "../../../shared/ui/status";
import { elapsed } from "../register/ticket-time";

/** One cell of an area's grid: a table in service, or nothing. */
export type Tile = LiveTableResponse | null;

/** The area laid out as the back office drew it, row by row. */
export function tilesOf(floor: LiveFloorResponse, areaId: number): Tile[][] {
  const at = new Map<string, LiveTableResponse>();
  for (const t of floor.tables) {
    if (t.area_id === areaId) at.set(`${t.x},${t.y}`, t);
  }
  const rows = Array.from({ length: floor.grid.rows }, (_, y) => Array.from({ length: floor.grid.columns }, (_, x) => at.get(`${x},${y}`) ?? null));
  // Rows and columns with no table in any area of this floor would only
  // push the tiles apart: trim the empty edges, keep the gaps between.
  return trim(rows);
}

function trim(rows: Tile[][]): Tile[][] {
  const used = (row: Tile[]) => row.some((t) => t !== null);
  const first = rows.findIndex(used);
  if (first < 0) return [];
  const last = rows.length - 1 - [...rows].reverse().findIndex(used);
  const kept = rows.slice(first, last + 1);
  const width = kept[0]?.length ?? 0;
  const colUsed = (x: number) => kept.some((row) => row[x] !== null);
  let left = 0;
  while (left < width && !colUsed(left)) left++;
  let right = width - 1;
  while (right > left && !colUsed(right)) right--;
  return kept.map((row) => row.slice(left, right + 1));
}

/**
 * The rows with every empty row and column taken out: each table keeps its
 * place among the others, and the tiles share the room instead of the gaps the
 * layout grid left between them (D05).
 */
export function packed(rows: Tile[][]): Tile[][] {
  const kept = rows.filter((row) => row.some((t) => t !== null));
  const width = kept[0]?.length ?? 0;
  const used = Array.from({ length: width }, (_, x) => kept.some((row) => row[x] !== null));
  return kept.map((row) => row.filter((_, x) => used[x]));
}

/** The area to show: the one chosen if it is still there, else the first. */
export function areaToShow(floor: LiveFloorResponse, chosen: number | null): number | null {
  if (chosen !== null && floor.areas.some((a) => a.id === chosen)) return chosen;
  return floor.areas[0]?.id ?? null;
}

/** The areas to draw: the one chosen, or every one under "All Areas". */
export type AreaChoice = number | "all" | null;

export function areasShown(floor: LiveFloorResponse, choice: AreaChoice): readonly AreaResponse[] {
  if (choice === "all") return floor.areas;
  const id = areaToShow(floor, choice);
  return floor.areas.filter((a) => a.id === id);
}

/** A table's five states, in the order a table goes through them. */
export const turnOrder: readonly TableStatus[] = ["free", "seated", "ordered", "settling", "dirty"];

/** "3 free · 4 seated · 1 paying · 2 to clear": every state there is, in turn order. */
export function countsText(tables: readonly LiveTableResponse[]): string {
  const counts = new Map<TableStatus, number>();
  for (const t of tables) counts.set(t.status, (counts.get(t.status) ?? 0) + 1);
  return turnOrder
    .filter((s) => counts.has(s))
    .map((s) => `${counts.get(s)} ${tableStatuses[s].word.toLowerCase()}`)
    .join(" · ");
}

/**
 * The table a tap picks. A table joined to a party stands for the party's own
 * table: that is where its order and its guests are.
 */
export function partyTable(t: LiveTableResponse, tables: readonly LiveTableResponse[]): LiveTableResponse {
  return (t.joined_to ? tables.find((o) => o.id === t.joined_to) : undefined) ?? t;
}

/** The panel's heading: "Table T4", or "Table T4 + T7" for a party over joined tables. */
export function panelTitle(t: LiveTableResponse, tables: readonly LiveTableResponse[]): string {
  return `Table ${[t, ...tables.filter((o) => o.joined_to === t.id)].map((o) => o.name).join(" + ")}`;
}

/**
 * What the panel shows for the table picked: a free table seats a party, a
 * table to clear is cleared, a table with one order shows it, and one with
 * several asks which, until one is picked.
 */
export type PanelView =
  | { readonly kind: "seat" }
  | { readonly kind: "clear" }
  | { readonly kind: "order"; readonly orderId: string }
  | { readonly kind: "choose"; readonly orders: readonly string[] }
  | { readonly kind: "nothing" };

export function panelFor(t: LiveTableResponse, picked: string | null = null): PanelView {
  if (t.status === "free") return { kind: "seat" };
  if (t.status === "dirty") return { kind: "clear" };
  if (picked !== null && t.orders.includes(picked)) return { kind: "order", orderId: picked };
  const [first, ...rest] = t.orders;
  if (first === undefined) return { kind: "nothing" };
  return rest.length === 0 ? { kind: "order", orderId: first } : { kind: "choose", orders: t.orders };
}

/** How many tables are in a state, and the share of the floor that is, for a bar. */
export interface Availability {
  readonly status: TableStatus;
  readonly count: number;
  /** 0 to 100. */
  readonly share: number;
}

export function availability(tables: readonly LiveTableResponse[]): Availability[] {
  return turnOrder.map((status) => {
    const count = tables.filter((t) => t.status === status).length;
    return { status, count, share: tables.length === 0 ? 0 : Math.round((count / tables.length) * 100) };
  });
}

/**
 * The table list's rows: area by area as the floor lists them, and within an
 * area by name the way people count, "T2" before "T10".
 */
export function listRows(floor: LiveFloorResponse, areas: readonly AreaResponse[]): LiveTableResponse[] {
  const place = new Map(areas.map((a, i) => [a.id, i]));
  const at = (t: LiveTableResponse) => place.get(t.area_id) ?? 0;
  return floor.tables.filter((t) => place.has(t.area_id)).sort((a, b) => at(a) - at(b) || a.name.localeCompare(b.name, undefined, { numeric: true }));
}

/** The guests a free table's panel starts at: the table's seats, within 1 to 12. */
export function startingCovers(seats: number): number {
  return Math.min(Math.max(seats, 1), 12);
}

/** A tile's small print: who is there, since when. The balance is shown apart, as money. */
export interface TileText {
  readonly people: string;
  readonly time: string | null;
}

export function tileText(t: LiveTableResponse, now: number, names: ReadonlyMap<number, string> = new Map()): TileText {
  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
  const time = t.since ? elapsed(t.since, now, 0).text : null;
  switch (t.status) {
    case "free":
      return { people: count(t.seats, "seat", "seats"), time: null };
    case "dirty":
      return { people: "Needs clearing", time };
    default:
      if (t.joined_to) return { people: `With ${names.get(t.joined_to) ?? `table ${t.joined_to}`}`, time };
      return { people: t.covers > 0 ? count(t.covers, "guest", "guests") : "Seated", time };
  }
}

/**
 * The table a link to the floor names ("?table=7"), or null: the waiter's card
 * leads here with its table picked (routes.ts). Anything but a whole number
 * above zero picks nothing.
 */
export function tableInSearch(search: string): number | null {
  const value = new URLSearchParams(search).get("table") ?? "";
  return /^[1-9]\d{0,9}$/.test(value) ? Number(value) : null;
}

/** Table names by id, for ticket labels. */
export function tableNames(floor: LiveFloorResponse | undefined): ReadonlyMap<number, string> {
  return new Map((floor?.tables ?? []).map((t) => [t.id, t.name]));
}

/**
 * Moving and joining on the floor (ADR-088): a mode the panel's More Actions
 * turns on, then taps on tiles. Move takes a party's table, then a free one. Join
 * takes a party's table, then any number of free ones; tapping a table
 * already joined lets it go. Part lets joined tables go, one tap each.
 */
export type Mode =
  | { readonly kind: "idle" }
  | { readonly kind: "move"; readonly from: LiveTableResponse | null }
  | { readonly kind: "join"; readonly to: LiveTableResponse | null }
  | { readonly kind: "part" };

/** What a tap in a mode asks the server to do. */
export type FloorAction =
  | { readonly kind: "move"; readonly orderId: string; readonly from: number; readonly to: number }
  | { readonly kind: "join"; readonly orderId: string; readonly table: number }
  | { readonly kind: "part"; readonly orderId: string; readonly table: number };

export interface Step {
  readonly mode: Mode;
  readonly action?: FloorAction;
  /** Why the tap did nothing: shown until the next one. */
  readonly refused?: string;
}

export const idle: Mode = { kind: "idle" };

/** Why a table cannot be the party's table in a move or join; null when it can. */
function notAParty(t: LiveTableResponse, names: ReadonlyMap<number, string>): string | null {
  if (t.status === "free" || t.status === "dirty") return `${t.name} has nobody at it. Tap a table with guests.`;
  if (t.joined_to) return `${t.name} is joined to ${names.get(t.joined_to) ?? "another table"}. Tap that one.`;
  if (t.orders.length !== 1) return `${t.name} has ${t.orders.length} tickets: the floor can't tell which one to move.`;
  return null;
}

export function stepMode(mode: Mode, t: LiveTableResponse, names: ReadonlyMap<number, string>): Step {
  switch (mode.kind) {
    case "idle":
      return { mode };
    case "move": {
      if (mode.from === null) {
        const refused = notAParty(t, names);
        return refused ? { mode, refused } : { mode: { kind: "move", from: t } };
      }
      if (t.id === mode.from.id) return { mode: { kind: "move", from: null } };
      if (t.status !== "free") return { mode, refused: `${t.name} isn't free. Tap a free table.` };
      const orderId = mode.from.orders[0] ?? "";
      return { mode: idle, action: { kind: "move", orderId, from: mode.from.id, to: t.id } };
    }
    case "join": {
      if (mode.to === null) {
        const refused = notAParty(t, names);
        return refused ? { mode, refused } : { mode: { kind: "join", to: t } };
      }
      const orderId = mode.to.orders[0] ?? "";
      if (t.id === mode.to.id) return { mode };
      if (t.joined_to === mode.to.id) return { mode, action: { kind: "part", orderId, table: t.id } };
      if (t.status !== "free") return { mode, refused: `${t.name} isn't free. Tap a free table to join.` };
      return { mode, action: { kind: "join", orderId, table: t.id } };
    }
    case "part": {
      const orderId = t.orders[0];
      if (!t.joined_to || !orderId) return { mode, refused: `${t.name} isn't joined to another table.` };
      return { mode, action: { kind: "part", orderId, table: t.id } };
    }
  }
}

/**
 * Starts a move or a join from the table picked in the panel: the first tap
 * is already made. Refused, in words, when that table has no one party.
 */
export function beginMode(kind: "move" | "join", t: LiveTableResponse, names: ReadonlyMap<number, string>): Step {
  return stepMode(kind === "move" ? { kind: "move", from: null } : { kind: "join", to: null }, t, names);
}

/** The banner while a mode is on. */
export function modeText(mode: Mode): string | null {
  switch (mode.kind) {
    case "idle":
      return null;
    case "move":
      return mode.from ? `Tap a free table for ${mode.from.name}'s party` : "Move: tap the table the party is at";
    case "join":
      return mode.to ? `Tap free tables to join to ${mode.to.name}. Tap a joined one to let it go.` : "Join: tap the party's table";
    case "part":
      return "Part: tap a joined table to let it go";
  }
}

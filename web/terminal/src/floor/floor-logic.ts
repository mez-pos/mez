// What the floor works out (terminal-floor.md): an area as rows of tiles,
// what a tap on a tile does, the counts in the header, and each tile's
// small print. The tables and their states are the server's (ADR-086).

import type { LiveFloorResponse, LiveTableResponse, TableStatus } from "../../../shared/api-types";
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
  const rows = Array.from({ length: floor.grid.rows }, (_, y) =>
    Array.from({ length: floor.grid.columns }, (_, x) => at.get(`${x},${y}`) ?? null),
  );
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

/** The area to show: the one chosen if it is still there, else the first. */
export function areaToShow(floor: LiveFloorResponse, chosen: number | null): number | null {
  if (chosen !== null && floor.areas.some((a) => a.id === chosen)) return chosen;
  return floor.areas[0]?.id ?? null;
}

const order: readonly TableStatus[] = ["free", "seated", "ordered", "settling", "dirty"];

/** "3 free · 4 seated · 1 paying · 2 to clear": every state there is, in turn order. */
export function countsText(tables: readonly LiveTableResponse[]): string {
  const counts = new Map<TableStatus, number>();
  for (const t of tables) counts.set(t.status, (counts.get(t.status) ?? 0) + 1);
  return order
    .filter((s) => counts.has(s))
    .map((s) => `${counts.get(s)} ${tableStatuses[s].word.toLowerCase()}`)
    .join(" · ");
}

/**
 * What a tap on a table does. A free table seats a party; a table with one
 * ticket being eaten or paid for opens it; anything with a choice in it (a
 * seated party who may leave, a table to clear, two tickets) opens a sheet.
 */
export type Tap = { kind: "seat" } | { kind: "ticket"; orderId: string } | { kind: "sheet" };

export function tapFor(t: LiveTableResponse): Tap {
  if (t.status === "free") return { kind: "seat" };
  const only = t.orders.length === 1 ? t.orders[0] : undefined;
  if ((t.status === "ordered" || t.status === "settling") && only) return { kind: "ticket", orderId: only };
  return { kind: "sheet" };
}

/** The covers a seat sheet starts at: the table's seats, within 1 to 12. */
export function startingCovers(seats: number): number {
  return Math.min(Math.max(seats, 1), 12);
}

/** A tile's small print: who is there, since when. The balance is shown apart, as money. */
export interface TileText {
  readonly people: string;
  readonly time: string | null;
}

export function tileText(t: LiveTableResponse, now: number): TileText {
  const count = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;
  const time = t.since ? elapsed(t.since, now, 0).text : null;
  switch (t.status) {
    case "free":
      return { people: count(t.seats, "seat", "seats"), time: null };
    case "dirty":
      return { people: "Needs clearing", time };
    default:
      return { people: t.covers > 0 ? count(t.covers, "cover", "covers") : "Seated", time };
  }
}

/** Table names by id, for ticket labels. */
export function tableNames(floor: LiveFloorResponse | undefined): ReadonlyMap<number, string> {
  return new Map((floor?.tables ?? []).map((t) => [t.id, t.name]));
}

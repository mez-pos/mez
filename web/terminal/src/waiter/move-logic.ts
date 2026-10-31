// Moving a party and joining tables from the table's screen (T06): the tables
// offered, what a tap picks, what the button says, and what is asked of the
// server. The rules are the floor's (ADR-088): a party moves to a free table;
// free tables are joined to it, and a joined one is let go.

import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { listRows, type FloorAction } from "../floor/floor-logic";
import { cardWords } from "./waiter-logic";

export type MoveMode = "transfer" | "merge";

/** A table offered in the modal: free to take, joined to this party, or with someone else. */
export interface MoveTile {
  readonly table: LiveTableResponse;
  readonly state: "free" | "joined" | "taken";
  /** "Available · 4" (its seats), "Joined", or why it cannot be taken. */
  readonly word: string;
}

/** Every table but the party's own, area by area, "T2" before "T10". */
export function moveTiles(floor: LiveFloorResponse, party: LiveTableResponse): MoveTile[] {
  return listRows(floor, floor.areas)
    .filter((t) => t.id !== party.id)
    .map((t) => {
      if (t.joined_to === party.id) return { table: t, state: "joined", word: "Joined" };
      if (t.status === "free") return { table: t, state: "free", word: `Available · ${t.seats}` };
      return { table: t, state: "taken", word: cardWords(t).label };
    });
}

/** Whether a tile can be picked: a free table always, a joined one only to let it go. */
export function canPick(mode: MoveMode, tile: MoveTile): boolean {
  return tile.state === "free" || (mode === "merge" && tile.state === "joined");
}

/** The picks after a tap: one table for a transfer, any number for a merge; a second tap lets go. */
export function pickTile(mode: MoveMode, picked: readonly number[], tile: MoveTile): number[] {
  if (!canPick(mode, tile)) return [...picked];
  const id = tile.table.id;
  if (picked.includes(id)) return picked.filter((p) => p !== id);
  return mode === "transfer" ? [id] : [...picked, id];
}

const chosen = (tiles: readonly MoveTile[], picked: readonly number[], state: MoveTile["state"]) =>
  tiles.filter((t) => t.state === state && picked.includes(t.table.id)).map((t) => t.table);

/** What the button says it will do, or null while nothing is picked. */
export function moveLabel(mode: MoveMode, tiles: readonly MoveTile[], picked: readonly number[]): string | null {
  const names = (tables: readonly LiveTableResponse[]) => tables.map((t) => t.name).join(", ");
  const free = chosen(tiles, picked, "free");
  if (mode === "transfer") return free[0] ? `Move to ${free[0].name}` : null;
  const joined = chosen(tiles, picked, "joined");
  if (free.length > 0 && joined.length > 0) return `Join ${names(free)} and let go ${names(joined)}`;
  if (free.length > 0) return `Join ${names(free)}`;
  return joined.length > 0 ? `Let go ${names(joined)}` : null;
}

/** What to ask of the server, in turn. */
export function moveActions(mode: MoveMode, orderId: string, party: LiveTableResponse, tiles: readonly MoveTile[], picked: readonly number[]): FloorAction[] {
  const free = chosen(tiles, picked, "free");
  if (mode === "transfer") return free[0] ? [{ kind: "move", orderId, from: party.id, to: free[0].id }] : [];
  return [
    ...free.map((t): FloorAction => ({ kind: "join", orderId, table: t.id })),
    ...chosen(tiles, picked, "joined").map((t): FloorAction => ({ kind: "part", orderId, table: t.id })),
  ];
}

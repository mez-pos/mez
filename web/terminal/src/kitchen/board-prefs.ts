// The choices a kitchen display keeps for itself (the foot of D10): espresso
// night, the chime for a new ticket, and whether a table's tickets are kept
// together. They are the device's, like its station: a display on a wall has
// nobody signed in to keep them for.

import type { KitchenTicketResponse } from "../../../shared/api-types";

export interface BoardPrefs {
  /** Espresso night (D11): the dark ground, for a dim pass. */
  night: boolean;
  /** A chime when a new ticket arrives. */
  sound: boolean;
  /** A table's tickets sit together in each column. */
  groupByTable: boolean;
}

/**
 * A display starts on espresso night, silent, in the order tickets were sent.
 * Night is the kitchen's surface (tokens.md): a bright screen over a hot pass
 * is the wrong tool. Paper stays a choice in the foot (ADR-381, over ADR-177).
 */
export const defaultPrefs: BoardPrefs = { night: true, sound: false, groupByTable: false };

export const prefsKey = "mez.kitchen.prefs";

/** The choices kept on this device; anything unreadable or missing is the default. */
export function readPrefs(storage: Pick<Storage, "getItem"> | null): BoardPrefs {
  try {
    const raw: unknown = JSON.parse(storage?.getItem(prefsKey) ?? "null");
    if (typeof raw !== "object" || raw === null) return defaultPrefs;
    const kept = raw as Record<string, unknown>;
    const flag = (key: keyof BoardPrefs) => (typeof kept[key] === "boolean" ? kept[key] : defaultPrefs[key]);
    return { night: flag("night"), sound: flag("sound"), groupByTable: flag("groupByTable") };
  } catch {
    return defaultPrefs;
  }
}

/** Keeps the choices. A device that blocks storage keeps them until it reloads. */
export function writePrefs(storage: Pick<Storage, "setItem"> | null, prefs: BoardPrefs): void {
  try {
    storage?.setItem(prefsKey, JSON.stringify(prefs));
  } catch {
    // Storage blocked: the choice lasts until the page is reloaded.
  }
}

/** The surface a choice of night puts the page on (tokens.md). */
export function surfaceOf(prefs: BoardPrefs): "ink" | "paper" {
  return prefs.night ? "ink" : "paper";
}

/**
 * A column with each order's tickets together: an order takes the place of
 * its first ticket, and its later ones follow it. A table sent to twice is
 * then plated as one. The order of the column is otherwise kept.
 */
export function groupByTable(tickets: readonly KitchenTicketResponse[]): KitchenTicketResponse[] {
  const firstAt = new Map<string, number>();
  tickets.forEach((t, i) => {
    if (!firstAt.has(t.order_id)) firstAt.set(t.order_id, i);
  });
  return tickets
    .map((t, i) => ({ t, i }))
    .sort((a, b) => (firstAt.get(a.t.order_id) ?? 0) - (firstAt.get(b.t.order_id) ?? 0) || a.i - b.i)
    .map(({ t }) => t);
}

/** A ticket's name on this board: its order and its number on it. */
export const ticketId = (t: KitchenTicketResponse): string => `${t.order_id}-${t.ticket_seq}`;

/**
 * The new tickets that were not on the board at the last look. Null before
 * the first look: a display that has just been turned on does not chime for
 * a board that was already full.
 */
export function arrivals(seen: ReadonlySet<string> | null, fresh: readonly KitchenTicketResponse[]): string[] {
  if (seen === null) return [];
  return fresh.map(ticketId).filter((id) => !seen.has(id));
}

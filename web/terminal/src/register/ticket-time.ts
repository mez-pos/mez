// How long a ticket has been open, as the ticket list shows it. Past the
// venue's threshold it is late, which is a "now" signal: terracotta (Task 5).

export interface Elapsed {
  /** "4m", "1h 05m": minutes are what a floor works in. */
  readonly text: string;
  readonly late: boolean;
}

export function elapsed(openedAt: string, now: number, lateAfterMinutes: number): Elapsed {
  const opened = Date.parse(openedAt);
  const minutes = Number.isNaN(opened) ? 0 : Math.max(0, Math.floor((now - opened) / 60_000));
  const text = minutes < 60 ? `${minutes}m` : `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, "0")}m`;
  return { text, late: lateAfterMinutes > 0 && minutes >= lateAfterMinutes };
}

/**
 * "Table T4 · 2 covers", "Counter". A table goes by its name from the floor
 * (terminal-floor.md); by its number until the floor has been read.
 */
export function ticketLabel(
  t: { readonly table_id?: number; readonly joined?: readonly number[]; readonly covers: number },
  names: ReadonlyMap<number, string> = new Map(),
): string {
  const name = (id: number) => names.get(id) ?? String(id);
  // A party joined over more tables names them all: "Table T4 + T7".
  const tables = t.table_id ? [t.table_id, ...(t.joined ?? [])].map(name).join(" + ") : "";
  const where = tables ? `Table ${tables}` : "Counter";
  if (!t.covers) return where;
  return `${where} · ${t.covers} ${t.covers === 1 ? "cover" : "covers"}`;
}

// How long a ticket has been open, as the ticket list shows it. Past the
// venue's threshold it is late, which is a "now" signal: vermilion (Task 5).

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

/** "Table 4 · 2 covers", "Counter". */
export function ticketLabel(t: { readonly table_id?: number; readonly covers: number }): string {
  const where = t.table_id ? `Table ${t.table_id}` : "Counter";
  if (!t.covers) return where;
  return `${where} · ${t.covers} ${t.covers === 1 ? "cover" : "covers"}`;
}

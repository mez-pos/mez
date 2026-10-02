// What the kitchen display works out (kitchen-display.md): which tickets
// are still to make, how late each is, what each says it is for, the
// all-day counts, and which ticket Recall would bring back. The board is
// the server's (ADR-090); this only reads it.

import type { KitchenBoardResponse, KitchenTicketResponse } from "../../../shared/api-types";

/** A ticket's timer: calm, then waiting from half the late time, then late. */
export type TimerTone = "calm" | "waiting" | "late";

export function timerTone(elapsedMs: number, lateAfterMinutes: number): TimerTone {
  const late = Math.max(lateAfterMinutes, 1) * 60_000;
  if (elapsedMs >= late) return "late";
  if (elapsedMs >= late / 2) return "waiting";
  return "calm";
}

/** The words that go with a tone: colour is never the only signal. */
export const toneWord: Readonly<Record<TimerTone, string | null>> = { calm: null, waiting: "Waiting", late: "Late" };

/** "0:42", "7:05", "1:02:09": minutes and seconds, hours when it comes to that. */
export function timerText(elapsedMs: number): string {
  const total = Math.max(0, Math.floor(elapsedMs / 1000));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = String(total % 60).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${s}` : `${m}:${s}`;
}

/** The tickets still to make, oldest first. */
export function toMake(board: KitchenBoardResponse | undefined): KitchenTicketResponse[] {
  return (board?.tickets ?? [])
    .filter((t) => t.status === "fired")
    .sort((a, b) => Date.parse(a.sent_at) - Date.parse(b.sent_at) || a.ticket_seq - b.ticket_seq);
}

/** Where a ticket goes: "T4", "T4 + T7", or "Counter · NMRN" for a counter order. */
export function whereText(t: KitchenTicketResponse): string {
  if (t.table) return t.table;
  if (t.table_id) return `Table ${t.table_id}`;
  const short = t.order_id.replace(/^ord_/, "").slice(0, 4);
  return `Counter · ${short}`;
}

export interface AllDay {
  readonly name: string;
  readonly qty: number;
}

/** How many of each dish are still to make, most first, then by name. */
export function allDay(tickets: readonly KitchenTicketResponse[]): AllDay[] {
  const counts = new Map<string, number>();
  for (const t of tickets) {
    if (t.status !== "fired") continue;
    for (const l of t.lines) {
      if (l.status === "voided") continue;
      counts.set(l.name, (counts.get(l.name) ?? 0) + l.qty);
    }
  }
  return [...counts]
    .map(([name, qty]) => ({ name, qty }))
    .sort((a, b) => b.qty - a.qty || a.name.localeCompare(b.name));
}

/** The ticket Recall brings back: the one bumped last, still in the window. */
export function lastBumped(board: KitchenBoardResponse | undefined): KitchenTicketResponse | null {
  let last: KitchenTicketResponse | null = null;
  for (const t of board?.tickets ?? []) {
    if (t.status !== "ready" || !t.ready_at) continue;
    if (!last || Date.parse(t.ready_at) > Date.parse(last.ready_at ?? "")) last = t;
  }
  return last;
}

/** Where the station chosen on this device is kept: a display stays on its station. */
export const stationKey = "mez.kitchen.station";

export function readStation(storage: Pick<Storage, "getItem"> | null): string | null {
  try {
    const v = storage?.getItem(stationKey) ?? null;
    return v && /^[a-z0-9_-]{1,40}$/.test(v) ? v : null;
  } catch {
    return null;
  }
}

// What the kitchen display works out (kitchen-display.md): which tickets
// are still to make, how late each is, what each says it is for, the
// all-day counts, and which ticket Recall would bring back. The board is
// the server's (ADR-090); this only reads it.

import type { KitchenBoardResponse, KitchenStatsResponse, KitchenTicketResponse } from "../../../shared/api-types";

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

/** In the kitchen: new on the board, or picked up by a cook (ADR-175). Not yet on the pass. */
export function inKitchen(t: KitchenTicketResponse): boolean {
  return t.status === "fired" || t.status === "in_progress";
}

/** The tickets still to make, oldest first. */
export function toMake(board: KitchenBoardResponse | undefined): KitchenTicketResponse[] {
  return (board?.tickets ?? []).filter(inKitchen).sort((a, b) => Date.parse(a.sent_at) - Date.parse(b.sent_at) || a.ticket_seq - b.ticket_seq);
}

/**
 * The board's three columns (D10, ADR-175): new tickets nobody has started,
 * those a cook is making, and those on the pass. The first two are oldest
 * first, as they should be made; the pass has the latest on top, since that
 * is the one a runner has just been called for.
 */
export interface BoardColumns {
  fresh: KitchenTicketResponse[];
  preparing: KitchenTicketResponse[];
  ready: KitchenTicketResponse[];
}

export function boardColumns(board: KitchenBoardResponse | undefined): BoardColumns {
  const making = toMake(board);
  const ready = (board?.tickets ?? [])
    .filter((t) => t.status === "ready")
    .sort((a, b) => Date.parse(b.ready_at ?? b.sent_at) - Date.parse(a.ready_at ?? a.sent_at) || b.ticket_seq - a.ticket_seq);
  return { fresh: making.filter((t) => t.status === "fired"), preparing: making.filter((t) => t.status === "in_progress"), ready };
}

/** How far a ticket is towards late, 0 to 100: the bar under a ticket being made. */
export function latePercent(elapsedMs: number, lateAfterMinutes: number): number {
  const late = Math.max(lateAfterMinutes, 1) * 60_000;
  return Math.min(100, Math.max(0, Math.round((elapsedMs * 100) / late)));
}

const serviceWords = { dine_in: "Dine-in", takeaway: "Takeaway", delivery: "Delivery" } as const;

/** "4 guests · Dine-in", "Takeaway": who it is for and how it leaves the pass. */
export function metaText(t: KitchenTicketResponse): string {
  const guests = t.covers > 0 ? `${t.covers} ${t.covers === 1 ? "guest" : "guests"}` : null;
  return [guests, serviceWords[t.service]].filter(Boolean).join(" · ");
}

/** "now", "2m", "1h 05m": how long ago, for a ticket on the pass. */
export function agoText(elapsedMs: number): string {
  const minutes = Math.max(0, Math.floor(elapsedMs / 60_000));
  if (minutes === 0) return "now";
  return minutes < 60 ? `${minutes}m` : `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, "0")}m`;
}

/** "11 min", or "under 1 min": a time from sent to ready, to the nearest minute. */
export function minutesText(seconds: number): string {
  if (seconds < 60) return "under 1 min";
  return `${Math.round(seconds / 60)} min`;
}

export interface StatRow {
  label: string;
  /** "28", "11 min", "Biryani · 19 min"; "Not yet" where there is nothing to say. */
  value: string;
  /** The two times sit under a rule, apart from the three counts. */
  timing: boolean;
}

/**
 * Today's stats (D10), in the design's order and words. The figures are the
 * server's for the station's trading day (ADR-178); "In progress" is every
 * ticket not yet on the pass, new or being made. The counts are of tickets,
 * which is what a station is sent: an order sent to twice is two.
 */
export function statRows(stats: KitchenStatsResponse): StatRow[] {
  const slowest = stats.slowest;
  const dish = slowest ? (slowest.more > 0 ? `${slowest.name} +${slowest.more}` : slowest.name) : null;
  return [
    { label: "Total tickets", value: String(stats.sent), timing: false },
    { label: "In progress", value: String(stats.in_kitchen), timing: false },
    { label: "Completed", value: String(stats.ready), timing: false },
    { label: "Avg. prep time", value: stats.ready > 0 ? minutesText(stats.average_seconds) : "Not yet", timing: true },
    { label: "Slowest today", value: slowest && dish ? `${dish} · ${minutesText(slowest.seconds)}` : "Not yet", timing: true },
  ];
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
    if (!inKitchen(t)) continue;
    for (const l of t.lines) {
      if (l.status === "voided") continue;
      counts.set(l.name, (counts.get(l.name) ?? 0) + l.qty);
    }
  }
  return [...counts].map(([name, qty]) => ({ name, qty })).sort((a, b) => b.qty - a.qty || a.name.localeCompare(b.name));
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

// What the terminal says about guests' requests (terminal-requests.md,
// ADR-212). The requests are the server's; this words them, and says which
// are new since the terminal last looked.

import type { GuestRequestResponse, GuestRoundLineResponse } from "../../../shared/api-types";

/** How often the chime repeats while a request waits unanswered. */
export const remindEveryMs = 30_000;

/**
 * The requests that were not there at the last look. Nothing seen yet is
 * nothing known: a terminal opened onto a request already waiting is told of
 * it, because a request nobody notices is a guest waiting on nobody.
 */
export function arrived(seen: ReadonlySet<string> | null, requests: readonly Pick<GuestRequestResponse, "id">[]): string[] {
  return requests.filter((r) => !seen?.has(r.id)).map((r) => r.id);
}

/** The tables with a request waiting, for the floor's marker. */
export function wantingTables(requests: readonly Pick<GuestRequestResponse, "table_id">[]): ReadonlySet<number> {
  return new Set(requests.map((r) => r.table_id));
}

/** The pill's words: "1 table request", "3 table requests". */
export function pillLabel(count: number): string {
  return count === 1 ? "table request" : "table requests";
}

/** How long a request has waited: "Just now", "4 min", "1 h 5 min". */
export function waited(placedAt: string, now: number): string {
  const minutes = Math.floor(Math.max(0, now - Date.parse(placedAt)) / 60_000);
  if (!Number.isFinite(minutes) || minutes < 1) return "Just now";
  if (minutes < 60) return `${minutes} min`;
  const rest = minutes % 60;
  return rest === 0 ? `${Math.floor(minutes / 60)} h` : `${Math.floor(minutes / 60)} h ${rest} min`;
}

/** A line's choices in words, or nothing: "Oat, Extra shot". */
export function choicesOf(line: Pick<GuestRoundLineResponse, "choices">): string {
  return (line.choices ?? []).join(", ");
}

/** How many dishes a request asks for, counting each one. */
export function dishCount(request: Pick<GuestRequestResponse, "lines">): number {
  return request.lines.reduce((sum, l) => sum + l.qty, 0);
}

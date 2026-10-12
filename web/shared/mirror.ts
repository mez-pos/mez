// Whether this page is open on a mirror: the back office over a copy of the
// venue's backup, read when the venue's computer is off (ADR-312, ADR-326).
// Without React; use-mirror.tsx binds it.
//
// A mirror says so on every answer it gives, in a header carrying when its
// copy was last written by the venue (internal/api's HeaderMirror). The copy
// grows newer as the mirror follows its backup (ADR-328), so the time is
// read off every answer and not once: what the bar says is never older than
// what the screen under it is showing.

import type { ApiClient } from "./api";

/** internal/api's HeaderMirror: RFC 3339, UTC, or empty when the copy cannot say how old it is. */
export const headerMirror = "X-Mez-Mirror";

/**
 * A copy older than this, when the mirror last answered, is a venue that has
 * stopped sending: its computer is off, or it has no internet.
 */
export const venueOfflineAfterMs = 2 * 60_000;

/**
 * How often a mirror in view is asked something, so that the time it is "as
 * of" keeps moving and a venue that goes quiet is seen to. A mirror looks at
 * its backup once a minute, and only when it has been asked something since
 * (ADR-328): this is what asks.
 */
export const mirrorAskEveryMs = 20_000;

export interface MirrorMark {
  /** When the venue last wrote something the copy holds, in ms; null when the mirror could not say. */
  readonly asOf: number | null;
  /** When the mirror last answered this page, by this browser's clock. */
  readonly heardAt: number;
}

/** When the copy behind an answer was last written, or null for an answer that is not a mirror's. */
export function mirrorAsOf(response: Pick<Response, "headers">): { asOf: number | null } | null {
  const header = response.headers.get(headerMirror);
  if (header === null) return null;
  const at = Date.parse(header);
  return { asOf: Number.isNaN(at) ? null : at };
}

type Listener = () => void;

/** What the answers so far have said of the mirror. Null until one of them is a mirror's. */
export class MirrorWatch {
  private mark: MirrorMark | null = null;
  private readonly listeners = new Set<Listener>();

  constructor(private readonly now: () => number = () => Date.now()) {}

  current = (): MirrorMark | null => this.mark;

  subscribe = (listener: Listener): (() => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  /**
   * Shown every answer. One that is not a mirror's changes nothing: a page
   * is on a mirror or it is not, and a proxy's own 502 in front of one
   * carries no header.
   */
  heard = (response: Pick<Response, "headers">): void => {
    const said = mirrorAsOf(response);
    if (!said) return;
    // Two answers may land out of order across a change of copy, and the
    // time never goes back: a newer copy is never older.
    const asOf = newest(this.mark?.asOf ?? null, said.asOf);
    this.mark = { asOf, heardAt: this.now() };
    for (const listener of this.listeners) listener();
  };
}

function newest(a: number | null, b: number | null): number | null {
  if (a === null) return b;
  if (b === null) return a;
  return Math.max(a, b);
}

/**
 * Whether the venue had stopped sending when the mirror last answered. It is
 * judged then and not now: a page left alone asks nothing, and its copy
 * growing old on the screen says nothing of the venue.
 */
export function venueOffline(mark: MirrorMark): boolean {
  return mark.asOf === null || mark.heardAt - mark.asOf > venueOfflineAfterMs;
}

/**
 * What the bar says: "Mirror, as of 23:10", in the venue's zone, with the day
 * when it is not today there: "Mirror, as of 11 Oct, 23:10".
 */
export function mirrorText(mark: MirrorMark, now: number, timezone: string | null): string {
  if (mark.asOf === null) return "Mirror";
  const at = stamp(mark.asOf, timezone);
  const today = stamp(now, timezone);
  return at.day === today.day ? `Mirror, as of ${at.time}` : `Mirror, as of ${at.day}, ${at.time}`;
}

function stamp(ms: number, timezone: string | null): { day: string; time: string } {
  const parts = partsOf(new Date(ms), timezone);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
  return { day: `${get("day")} ${get("month")}`, time: `${get("hour")}:${get("minute")}` };
}

function partsOf(at: Date, timezone: string | null): Intl.DateTimeFormatPart[] {
  const options: Intl.DateTimeFormatOptions = { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", hourCycle: "h23" };
  try {
    return new Intl.DateTimeFormat("en-GB", timezone ? { ...options, timeZone: timezone } : options).formatToParts(at);
  } catch {
    // A zone this browser does not know: the device's own clock is better than none.
    return new Intl.DateTimeFormat("en-GB", options).formatToParts(at);
  }
}

/**
 * Asks the mirror something small, for the header on its answer. A mirror
 * that does not answer is left to the next time: the bar keeps the last
 * time it was told, which is still true of what the screens hold.
 */
export async function askMirror(api: Pick<ApiClient, "get">): Promise<void> {
  try {
    await api.get("/api/v1/devices/me", { timeoutMs: 4_000 });
  } catch {
    // Nothing to do with the failure here: a refused device is the auth
    // store's to hear, through the same client, and it has.
  }
}

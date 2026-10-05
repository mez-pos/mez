// The words for the venue's link to the relay (docs/design/screens/
// backoffice-devices.md, "Ordering from phones"). Pure, so each state's
// sentence is tested without a server that is in it.

import type { TunnelResponse, TunnelState } from "../../../shared/api-types";
import { dateText, timeText } from "./devices-logic";

/** How often the state is read again: often while it is changing, seldom once it has settled. */
export function refreshMs(state: TunnelState | undefined): number {
  switch (state) {
    case "connecting":
    case "waiting":
      return 5_000;
    case "off":
      // Set in a file and read at start: it cannot change under an open page.
      return 5 * 60_000;
    default:
      return 30_000;
  }
}

/**
 * What the state means for the venue, in a sentence an owner can act on.
 * Every state short of connected says what guests can and cannot do; the two
 * that waiting will not mend say what will.
 */
export function tunnelLine(t: Pick<TunnelResponse, "state" | "since">, now: Date): string {
  switch (t.state) {
    case "off":
      return "Guests order at the counter. Ordering from a phone needs a relay address and key in this venue's settings file.";
    case "connecting":
      return "Reaching the relay. Guests cannot order from their phones until it answers.";
    case "connected":
      return t.since ? `Guests can order from their phones. Connected since ${sinceText(t.since, now)}.` : "Guests can order from their phones.";
    case "waiting":
      return "The relay cannot be reached, so guests cannot order from their phones. The counter is not affected.";
    case "refused":
      return "The relay does not accept this venue's key, so guests cannot order from their phones. Check the relay key in the settings file, or ask whoever runs the relay for a new one.";
    case "replaced":
      return "Another computer is connected with this venue's key, and guests reach that one. Only one can be: stop the other, or give this one a key of its own.";
    case "stopped":
      return "The link to the relay is stopped. It starts again when mez does.";
  }
}

/** "14:02" for today, "3 Oct 2026, 14:02" for any other day, on the browser's clock. */
export function sinceText(iso: string, now: Date): string {
  const at = new Date(iso);
  return at.toDateString() === now.toDateString() ? timeText(iso) : `${dateText(iso)}, ${timeText(iso)}`;
}

/** "Trying again in 20 s", "in 4 min", or "now"; nothing when no attempt is due. */
export function retryText(t: Pick<TunnelResponse, "state" | "retry_at">, now: Date): string | null {
  if (!t.retry_at || t.state === "connected" || t.state === "off") return null;
  const ms = Date.parse(t.retry_at) - now.getTime();
  if (ms <= 0) return "Trying again now";
  if (ms < 60_000) return `Trying again in ${Math.ceil(ms / 1000)} s`;
  return `Trying again in ${Math.ceil(ms / 60_000)} min`;
}

/** The client's own account of the last failure, for whoever is helping; never while connected. */
export function detailText(t: Pick<TunnelResponse, "state" | "detail">): string | null {
  const detail = t.detail?.trim();
  if (!detail || t.state === "connected" || t.state === "off") return null;
  return detail;
}

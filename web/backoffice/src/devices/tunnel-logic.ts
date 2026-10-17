// The words for the venue's link to the relay (docs/design/screens/
// backoffice-devices.md, "Ordering from phones"). Pure, so each state's
// sentence is tested without a server that is in it.

import type { TunnelCloud, TunnelResponse, TunnelState } from "../../../shared/api-types";
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
export function tunnelLine(t: Pick<TunnelResponse, "state" | "since" | "cloud" | "paired_to">, now: Date): string {
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
      // A venue paired to mez Cloud pairs again: its key was turned off, or
      // the cloud's keys were all changed (ADR-359).
      if (t.paired_to) {
        return "mez Cloud no longer accepts this venue's key, so guests cannot order from their phones and the back office cannot be opened from away. Pair this computer again with a new code from the owner's mez Cloud account. The counter is not affected.";
      }
      return "The relay does not accept this venue's key, so guests cannot order from their phones. Check the relay key in the settings file, or ask whoever runs the relay for a new one.";
    case "not_entitled":
      return `The relay is not carrying this venue, so guests cannot order from their phones. ${whyNotCarried(t.cloud?.reason)} mez asks again every minute and comes back by itself once it is. The counter is not affected.`;
    case "replaced":
      return "Another computer is connected with this venue's key, and guests reach that one. Only one can be: stop the other, or give this one a key of its own.";
    case "stopped":
      return "The link to the relay is stopped. It starts again when mez does.";
  }
}

/**
 * Why the relay is not carrying the venue, from the short code it gave
 * (ADR-313, ADR-316). A code this build does not know says who to ask.
 */
export function whyNotCarried(reason: string | undefined): string {
  switch (reason) {
    case "payment_failed":
    case "payment_stopped":
      return "The last payment for mez Cloud did not go through.";
    case "trial_ended":
      return "The free trial has ended.";
    case "cancelled":
      return "The subscription was cancelled.";
    case "closed":
      return "The account this venue was under is closed.";
    case "expired":
      return "The date it was carried until has passed.";
    case "never":
    case "not_listed":
      return "Whoever runs the relay has not turned it on for this venue.";
    default:
      return "Whoever runs the relay can say why.";
  }
}

const DAY_MS = 24 * 60 * 60 * 1000;

/** "5 days left", "1 day left", "less than a day left"; nothing with no date, or one that has passed. */
export function daysLeft(until: string | undefined, now: Date): string | null {
  if (!until) return null;
  const ms = Date.parse(until) - now.getTime();
  if (Number.isNaN(ms) || ms <= 0) return null;
  if (ms < DAY_MS) return "less than a day left";
  const days = Math.ceil(ms / DAY_MS);
  return days === 1 ? "1 day left" : `${days} days left`;
}

/**
 * Where the venue stands with its relay, in a line: what is paid for and to
 * when, or how long is left once a payment has failed. Nothing when the
 * relay said nothing worth a line: one that carries the venue with no date.
 * A relay that keeps accounts gives a state and is "Cloud"; one that keeps a
 * file gives a date at most.
 */
export function cloudLine(cloud: TunnelCloud | undefined, now: Date): string | null {
  if (!cloud) return null;
  const paid = cloud.paid_until ? dateText(cloud.paid_until) : null;
  const until = cloud.until ? dateText(cloud.until) : null;
  switch (cloud.state) {
    case "trial":
      return paid ? `Cloud: free trial until ${paid}.` : "Cloud: free trial.";
    case "active":
      return paid ? `Cloud: paid until ${paid}.` : "Cloud: paid.";
    case "grace": {
      const what = cloud.reason === "trial_ended" ? "the free trial has ended" : "payment failed";
      const left = daysLeft(cloud.until, now);
      return left ? `Cloud: ${what}, ${left}.` : `Cloud: ${what}.`;
    }
    case "cancelled":
      return until ? `Cloud: cancelled. Carried until ${until}.` : "Cloud: cancelled.";
    case "suspended":
    case "closed":
      return paid ? `Cloud: stopped. It was paid until ${paid}.` : "Cloud: stopped.";
    default:
      return until ? `The relay carries this venue until ${until}.` : null;
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

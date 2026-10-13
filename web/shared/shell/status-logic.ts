// What the status bar says about the connection (rebuild flow map: Online or
// what is wrong). A function of its inputs, so every wording is tested without
// rendering anything. The date and the version are in shared/venue-text.ts.

import { describeConnection, type ConnectionDescription } from "../connection-status";
import { venueOffline, type MirrorMark } from "../mirror";
import type { RealtimeStatus } from "../realtime";

/**
 * The connection as the bar words it: "Online" while the stream is live, and
 * what the indicator already says otherwise ("Reconnecting", "No network").
 * Nothing in the terminal needs the internet, so this is the link to the mez
 * server and nothing wider.
 *
 * On a mirror the server is a copy of the venue, and what is worth a word is
 * the venue: "Nothing new from the venue" once the copy has stopped growing
 * newer (ADR-329). Not "offline": the copy cannot tell a venue that is off
 * from one that has sold nothing for two minutes, and the chip beside this
 * already says when it last heard (ADR-342).
 * Only once the page has settled as away, so a mirror that stops answering
 * still says so in the connection's own words.
 */
export function connectionText(status: RealtimeStatus, now: number, mirror: MirrorMark | null = null): ConnectionDescription {
  const described = describeConnection(status, now);
  if (status.state === "live") return { ...described, word: "Online" };
  if (venueSilent(status, mirror)) return { ...described, tone: "warning", word: "Nothing new from the venue" };
  return described;
}

/** Whether the page is on a mirror whose venue has stopped sending. */
export function venueSilent(status: RealtimeStatus, mirror: MirrorMark | null): boolean {
  return status.state === "away" && mirror !== null && venueOffline(mirror);
}

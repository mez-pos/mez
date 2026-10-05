// What the status bar says about the connection (rebuild flow map: Online or
// what is wrong). A function of its inputs, so every wording is tested without
// rendering anything. The date and the version are in shared/venue-text.ts.

import { describeConnection, type ConnectionDescription } from "../connection-status";
import type { RealtimeStatus } from "../realtime";

/**
 * The connection as the bar words it: "Online" while the stream is live, and
 * what the indicator already says otherwise ("Reconnecting", "No network").
 * Nothing in the terminal needs the internet, so this is the link to the mez
 * server and nothing wider.
 */
export function connectionText(status: RealtimeStatus, now: number): ConnectionDescription {
  const described = describeConnection(status, now);
  return status.state === "live" ? { ...described, word: "Online" } : described;
}

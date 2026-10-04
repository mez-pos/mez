// What the status bar says (rebuild flow map: version, Online or Offline, the
// venue, the date and time). Each is a function of its inputs, so every
// wording is tested without rendering anything.

import { describeConnection, type ConnectionDescription } from "../../../shared/connection-status";
import type { RealtimeStatus } from "../../../shared/realtime";

/** "Sat, 19 Sep 2026 · 10:24 AM", read in the venue's zone, or the device's own when there is none. */
export function clockText(at: Date, timezone: string | null): string {
  const parts = partsOf(at, timezone);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
  const when = `${get("weekday")}, ${get("day")} ${get("month")} ${get("year")}`;
  return `${when} · ${get("hour")}:${get("minute")} ${get("dayPeriod").toUpperCase()}`;
}

function partsOf(at: Date, timezone: string | null): Intl.DateTimeFormatPart[] {
  const options: Intl.DateTimeFormatOptions = {
    weekday: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  };
  try {
    return new Intl.DateTimeFormat("en-US", timezone ? { ...options, timeZone: timezone } : options).formatToParts(at);
  } catch {
    // A zone this browser does not know: the device's own clock is better than none.
    return new Intl.DateTimeFormat("en-US", options).formatToParts(at);
  }
}

/** "mez v1.0.0" for a release, "mez dev-dirty" for a build with no version number. */
export function versionText(version: string | null): string {
  if (!version) return "mez";
  return /^\d/.test(version) ? `mez v${version}` : `mez ${version}`;
}

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

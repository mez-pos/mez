// How a venue is written on a screen: its date and time, read in its own zone,
// and the server's version. Shared by the sign-in screen and the terminal's
// status bar, so both say them alike.

import { dayShort, dayText, tradingDay, venueTime, type VenueClock } from "./venue-clock";

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

/** What a status bar's clock says (BO-3, ADR-385). */
export interface BarClock {
  /** "Sat, 31 Oct 2026 · 10:58 AM"; between midnight and the day start, "Trading day 31 Oct · 1:05 AM". */
  text: string;
  /** The trading day alone, "31 Oct", where there is no room for the rest. */
  short: string;
  /** The whole of it, for a screen reader. */
  label: string;
}

/**
 * The bar's clock. Through the day it is the date and time. Between midnight
 * and the venue's day start, the reports' "today" is still the day before, so
 * the bar says that trading day where the date was: a range ending
 * "yesterday" at 1 AM is not a surprise.
 */
export function barClock(at: Date, clock: VenueClock): BarClock {
  const full = clockText(at, clock.timezone);
  const day = tradingDay(at, clock);
  const calendar = tradingDay(at, { ...clock, dayStart: 0 });
  if (day === calendar || day === "") return { text: full, short: dayShort(day), label: full };
  return {
    text: `Trading day ${dayShort(day)} · ${venueTime(at, clock.timezone)}`,
    short: dayShort(day),
    label: `${full}, still the trading day of ${dayText(day)}`,
  };
}

/** "mez v1.0.0" for a release, "mez dev-dirty" for a build with no version number. */
export function versionText(version: string | null): string {
  if (!version) return "mez";
  return /^\d/.test(version) ? `mez v${version}` : `mez ${version}`;
}

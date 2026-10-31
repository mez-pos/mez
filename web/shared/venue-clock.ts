// The venue's clock (ADR-385): what day and time it is where the venue is,
// whatever the device showing it is set to. A screen's "today" is the
// trading day, which begins at the venue's day start (04:00 unless it says
// otherwise), as the server counts it (shift.TradingDay). And every date or
// time a screen shows is written one way, read in the venue's zone:
// "3 Oct 2026", "11:40 PM", "3 Oct 2026, 11:40 PM".

/** Where and when the venue's day turns over. */
export interface VenueClock {
  /** The venue's zone, "Asia/Kolkata"; null, the device's own, until the venue has been read. */
  timezone: string | null;
  /** Minutes after midnight the trading day begins: 240 for 04:00. */
  dayStart: number;
}

/** The server's default day start, 04:00 (shift.DefaultDayStart). */
export const defaultDayStart = 4 * 60;

const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"] as const;

/** "05:30" as minutes after midnight; the default for none, or for what is not a time of day. */
export function dayStartMinutes(s: string | null | undefined): number {
  const m = /^(\d{2}):(\d{2})$/.exec(s ?? "");
  if (!m) return defaultDayStart;
  const hours = Number(m[1]);
  const minutes = Number(m[2]);
  return hours < 24 && minutes < 60 ? hours * 60 + minutes : defaultDayStart;
}

/** The clock of a venue as the server describes it; until it has, the device's zone and the default. */
export function venueClock(venue: { timezone: string; day_start?: string } | undefined): VenueClock {
  if (!venue) return { timezone: null, dayStart: defaultDayStart };
  return { timezone: venue.timezone || null, dayStart: dayStartMinutes(venue.day_start) };
}

/** The clock hour the trading day starts in: 4 for 04:00, 5 for 05:30. */
export function dayStartHour(clock: VenueClock): number {
  return Math.floor(clock.dayStart / 60);
}

/** "A trading day runs 5:30 AM to 5:30 AM.": the venue's, where a screen once said 04:00 for every venue. */
export function dayRunsText(clock: VenueClock): string {
  const hour = Math.floor(clock.dayStart / 60);
  const minute = String(clock.dayStart % 60).padStart(2, "0");
  const at = clock.dayStart === 0 ? "midnight" : `${hour % 12 || 12}:${minute} ${hour < 12 ? "AM" : "PM"}`;
  return `A trading day runs ${at} to ${at}.`;
}

/** The trading day an instant falls in: the venue's date, or the one before when it is earlier than the day start. */
export function tradingDay(at: Date, clock: VenueClock): string {
  const wall = wallClock(at, clock.timezone);
  if (!wall) return "";
  return wall.minutes < clock.dayStart ? addDays(wall.day, -1) : wall.day;
}

/** A day n days after another ("2026-10-31", 1 is "2026-11-01"); n may be negative. */
export function addDays(day: string, n: number): string {
  const [y, m, d] = day.split("-").map(Number) as [number, number, number];
  return new Date(Date.UTC(y, m - 1, d + n)).toISOString().slice(0, 10);
}

/** The last n trading days, today's included. */
export function lastDays(at: Date, n: number, clock: VenueClock): { from: string; to: string } {
  const to = tradingDay(at, clock);
  return { from: addDays(to, -(n - 1)), to };
}

/** A trading day, "2026-09-24", as "24 Sep 2026". What is not a day is left as it came. */
export function dayText(day: string): string {
  const d = splitDay(day);
  return d ? `${d.day} ${months[d.month - 1]} ${d.year}` : day;
}

/** A trading day without its year, "24 Sep", for a range that says the year once. */
export function dayShort(day: string): string {
  const d = splitDay(day);
  return d ? `${d.day} ${months[d.month - 1]}` : day;
}

/** The venue's date at an instant: "2 Oct 2026". Empty for what is not a time. */
export function venueDate(at: string | Date, timezone: string | null): string {
  const wall = wallClock(toDate(at), timezone);
  return wall ? dayText(wall.day) : "";
}

/** The venue's time at an instant, on the twelve-hour clock: "10:58 AM". Empty for what is not a time. */
export function venueTime(at: string | Date, timezone: string | null): string {
  const wall = wallClock(toDate(at), timezone);
  if (!wall) return "";
  const hour = Math.floor(wall.minutes / 60);
  const minute = String(wall.minutes % 60).padStart(2, "0");
  return `${hour % 12 || 12}:${minute} ${hour < 12 ? "AM" : "PM"}`;
}

/** Both: "31 Oct 2026, 10:58 AM". Empty for what is not a time. */
export function venueDateTime(at: string | Date, timezone: string | null): string {
  const date = venueDate(at, timezone);
  return date && `${date}, ${venueTime(at, timezone)}`;
}

function toDate(at: string | Date): Date {
  return at instanceof Date ? at : new Date(at);
}

function splitDay(day: string): { year: number; month: number; day: number } | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(day);
  if (!m) return null;
  const month = Number(m[2]);
  return month >= 1 && month <= 12 ? { year: Number(m[1]), month, day: Number(m[3]) } : null;
}

const wallOptions: Intl.DateTimeFormatOptions = {
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
};

/** The date and the minutes past midnight on the wall at the venue; null for what is not a time. */
function wallClock(at: Date, timezone: string | null): { day: string; minutes: number } | null {
  if (Number.isNaN(at.getTime())) return null;
  let parts: Intl.DateTimeFormatPart[];
  try {
    parts = new Intl.DateTimeFormat("en-US", timezone ? { ...wallOptions, timeZone: timezone } : wallOptions).formatToParts(at);
  } catch {
    // A zone this browser does not know: the device's own clock is better than none.
    parts = new Intl.DateTimeFormat("en-US", wallOptions).formatToParts(at);
  }
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
  // Some engines write midnight as 24 under h23's older cousin; it is 0.
  const hour = Number(get("hour")) % 24;
  return { day: `${get("year")}-${get("month")}-${get("day")}`, minutes: hour * 60 + Number(get("minute")) };
}

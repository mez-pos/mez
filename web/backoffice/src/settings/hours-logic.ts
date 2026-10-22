// The venue's opening hours as a form (ADR-360; backoffice-settings.md,
// "Shift, tips and receipt"): a row per weekday, ticked when the venue opens
// that day, with an opening and a closing time. The till does nothing with
// them; mez Cloud's alerts read them from the venue's health summary. The
// server checks everything again. No React in this file.

import type { Day } from "../../../shared/api-types";

export type Weekday = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

/** The days, Monday first, as the server keys them. */
export const WEEKDAYS: readonly { key: Weekday; label: string }[] = [
  { key: "mon", label: "Monday" },
  { key: "tue", label: "Tuesday" },
  { key: "wed", label: "Wednesday" },
  { key: "thu", label: "Thursday" },
  { key: "fri", label: "Friday" },
  { key: "sat", label: "Saturday" },
  { key: "sun", label: "Sunday" },
];

export interface DayForm {
  /** Unticked: the venue is closed that day, and the times are not sent. */
  readonly closed: boolean;
  /** "09:00", as a time input holds it. */
  readonly open: string;
  readonly close: string;
}

export type HoursForm = Readonly<Record<Weekday, DayForm>>;

/** What a day newly ticked starts with. */
export const DEFAULT_OPEN = "09:00";
export const DEFAULT_CLOSE = "23:00";

const CLOCK = /^([01]\d|2[0-3]):[0-5]\d$/;

const CLOSED: DayForm = { closed: true, open: "", close: "" };

/** The form for what the server holds: a day it does not name is closed. */
export function hoursFormFrom(hours: Readonly<Record<string, Day>> | undefined): HoursForm {
  return WEEKDAYS.reduce(
    (form, { key }) => {
      const day = hours?.[key];
      return { ...form, [key]: day ? { closed: false, open: day.open, close: day.close } : CLOSED };
    },
    {} as Record<Weekday, DayForm>,
  );
}

/** A day ticked open: it keeps the times it had, or starts with the defaults. */
export function openDay(day: DayForm): DayForm {
  return { closed: false, open: day.open || DEFAULT_OPEN, close: day.close || DEFAULT_CLOSE };
}

/** What Save sends: the days that are open, trimmed. An empty object is "not set". */
export function toHoursInput(f: HoursForm): Record<string, Day> {
  return WEEKDAYS.reduce<Record<string, Day>>((out, { key }) => {
    const day = f[key];
    return day.closed ? out : { ...out, [key]: { open: day.open.trim(), close: day.close.trim() } };
  }, {});
}

/** Whether the day closes after midnight: its close is before its open, as 17:00 to 02:00. */
export function isOvernight(day: DayForm): boolean {
  const open = day.open.trim();
  const close = day.close.trim();
  return CLOCK.test(open) && CLOCK.test(close) && close < open;
}

/** What can be refused without asking the server, the earliest day first. */
export function hoursProblem(f: HoursForm): string | null {
  for (const { key, label } of WEEKDAYS) {
    const day = f[key];
    if (day.closed) continue;
    const open = day.open.trim();
    const close = day.close.trim();
    if (!CLOCK.test(open)) return `Give ${label} an opening time, like 09:00.`;
    if (!CLOCK.test(close)) return `Give ${label} a closing time, like 23:00.`;
    if (open === close) return `${label} opens and closes at ${open}. Close later than you open, or earlier for the next morning.`;
  }
  return null;
}

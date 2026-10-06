// The words and the one conversion behind the tips screen
// (docs/design/screens/tips.md). Shares are the server's; the only sum
// here is hours and minutes into minutes.

import type { PoolMember, StaffResponse, TipPoolResponse, TipRule } from "../../../shared/api-types";
import { ApiError, isUnreachable } from "../../../shared/api";

const rules: Readonly<Record<TipRule, string>> = {
  even: "Split evenly",
  hours: "Split by hours worked",
  role_weight: "Split by role weight",
};

export function ruleWords(r: TipRule): string {
  return rules[r];
}

/** Longest shift the server accepts, in minutes. */
export const maxMinutes = 36 * 60;

/** Hours and minutes typed in, as minutes, or null when either is not a whole number in range. */
export function toMinutes(hours: string, minutes: string): number | null {
  const h = hours.trim() === "" ? 0 : Number(hours);
  const m = minutes.trim() === "" ? 0 : Number(minutes);
  if (!Number.isInteger(h) || !Number.isInteger(m) || h < 0 || m < 0 || m > 59) return null;
  const total = h * 60 + m;
  return total > maxMinutes ? null : total;
}

/** A person on the screen: in the pool or not, and their time as typed. */
export interface Row {
  staffId: string;
  name: string;
  inPool: boolean;
  hours: string;
  minutes: string;
}

/**
 * The rows to start from: the server's suggested members, in, then the
 * rest of the staff, out. A suggestion's time, if it has one, is kept.
 */
export function startingRows(pool: TipPoolResponse, staff: readonly StaffResponse[]): Row[] {
  const rows: Row[] = pool.members.map((m) => ({
    staffId: m.staff_id,
    name: m.name,
    inPool: m.in_pool,
    hours: m.minutes > 0 ? String(Math.trunc(m.minutes / 60)) : "",
    minutes: m.minutes > 0 ? String(m.minutes % 60) : "",
  }));
  for (const s of staff) {
    if (!rows.some((r) => r.staffId === s.id)) rows.push({ staffId: s.id, name: s.name, inPool: false, hours: "", minutes: "" });
  }
  return rows;
}

/** The members to send, or null when someone in has hours that are not a time. */
export function membersOf(rows: readonly Row[]): PoolMember[] | null {
  const out: PoolMember[] = [];
  for (const r of rows) {
    if (!r.inPool) continue;
    const minutes = toMinutes(r.hours, r.minutes);
    if (minutes === null) return null;
    out.push({ staff_id: r.staffId, minutes });
  }
  return out;
}

/** Why the server would not split or pool, in words. */
export function tipProblem(err: unknown, rule: TipRule): string {
  if (err instanceof ApiError) {
    if (isUnreachable(err)) return "Can't reach the mez server to load the tips.";
    switch (err.code) {
      case "forbidden":
        return "You don't have permission to do that. Ask a manager.";
      case "already_pooled":
        return "This shift's tips are already pooled.";
      case "shift_still_open":
        return "The shift is still open. Tips are pooled after it closes.";
      // An empty pool never reaches the server: the screen says so first.
      case "invalid_tip_pool":
        if (rule === "hours") return "Enter hours for everyone in the pool.";
        return "These people can't be pooled. Check who is in.";
    }
  }
  return "The server refused this. Try again.";
}

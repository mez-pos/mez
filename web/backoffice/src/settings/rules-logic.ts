// The shift and tip rules as a form (docs/design/screens/backoffice-settings.md,
// "Shift, tips and receipt"): when the trading day starts, how far a drawer may
// be out before closing needs a reason, how tips are shared and what a ticket's
// clock means. The server checks everything again, and a weight must name a
// role it knows; what is checked here costs nothing. No React in this file.

import type { RulesRequest, RulesResponse } from "../../../shared/api-types";
import { majorText, parseMajor } from "./money-text";
import { problemWith, type Problem } from "./problem";

export type TipRule = "even" | "hours" | "role_weight";

export const TIP_RULES: readonly { value: TipRule; label: string }[] = [
  { value: "even", label: "Shared evenly" },
  { value: "hours", label: "By hours worked" },
  { value: "role_weight", label: "By role weight" },
];

// The server's limits (internal/service/settings/keys.go).
export const MAX_VARIANCE_MINOR = 100_000_000;
export const MIN_LATE_MINUTES = 1;
export const MAX_LATE_MINUTES = 600;
export const MIN_WEIGHT = 1;
export const MAX_WEIGHT = 100;

export interface RulesForm {
  /** "04:00", as a time input holds it. */
  dayStart: string;
  /** In major units, as typed: "50.00". */
  variance: string;
  tipRule: TipRule;
  /** By role name, as typed: empty counts as 1. Only roles the venue has. */
  weights: Record<string, string>;
  lateMinutes: string;
}

const tipRuleIds: readonly string[] = TIP_RULES.map((r) => r.value);

export function rulesFormFrom(r: RulesResponse): RulesForm {
  const weights: Record<string, string> = {};
  // A weight for a role that no longer exists is left behind: the server
  // refuses one, and a rename or a removal is how one comes to be stored.
  for (const role of r.roles) {
    const w = r.tip_role_weights[role];
    weights[role] = w === undefined ? "" : String(w);
  }
  return {
    dayStart: r.day_start,
    variance: majorText(r.variance_threshold, r.minor_per_major),
    tipRule: tipRuleIds.includes(r.tip_rule) ? (r.tip_rule as TipRule) : "even",
    weights,
    lateMinutes: String(r.ticket_late_minutes),
  };
}

/** The weights that were given: a role left empty is not listed, and weighs 1. */
function givenWeights(f: RulesForm): Record<string, number> {
  const out: Record<string, number> = {};
  for (const [role, text] of Object.entries(f.weights)) {
    const t = text.trim();
    if (t !== "") out[role] = /^\d{1,3}$/.test(t) ? Number(t) : NaN;
  }
  return out;
}

export function toRulesInput(f: RulesForm, perMajor: number): RulesRequest {
  return {
    day_start: f.dayStart.trim(),
    variance_threshold: parseMajor(f.variance, perMajor) ?? 0,
    tip_rule: f.tipRule,
    tip_role_weights: givenWeights(f),
    ticket_late_minutes: Number(f.lateMinutes.trim()),
  };
}

/** Whether the draft differs from what the server has, ignoring stray spaces. */
export function rulesChanged(draft: RulesForm, saved: RulesForm, perMajor: number): boolean {
  const norm = (f: RulesForm) =>
    JSON.stringify([f.dayStart.trim(), parseMajor(f.variance, perMajor) ?? f.variance.trim(), f.tipRule, givenWeights(f), f.lateMinutes.trim()]);
  return norm(draft) !== norm(saved);
}

export type RulesField = "day_start" | "variance_threshold" | "tip_rule" | "tip_role_weights" | "ticket_late_minutes";
const rulesFields: readonly RulesField[] = ["day_start", "variance_threshold", "tip_rule", "tip_role_weights", "ticket_late_minutes"];

export const rulesProblemOf = (err: unknown): Problem<RulesField> => problemWith(err, rulesFields);

/** What can be refused without asking the server, in the order the form reads. */
export function rulesProblem(f: RulesForm, perMajor: number): Problem<RulesField> | null {
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(f.dayStart.trim())) {
    return { field: "day_start", message: "Choose when the trading day starts, like 04:00." };
  }
  const variance = parseMajor(f.variance, perMajor);
  if (variance === null || variance > MAX_VARIANCE_MINOR) {
    return { field: "variance_threshold", message: `Enter an amount from 0 to ${majorText(MAX_VARIANCE_MINOR, perMajor)}.` };
  }
  const weights = Object.entries(givenWeights(f));
  const bad = weights.find(([, w]) => !(w >= MIN_WEIGHT && w <= MAX_WEIGHT));
  if (bad) return { field: "tip_role_weights", message: `Give ${bad[0]} a weight from ${MIN_WEIGHT} to ${MAX_WEIGHT}.` };
  if (f.tipRule === "role_weight" && weights.length === 0) {
    return { field: "tip_role_weights", message: "Give at least one role a weight, or choose another way to share tips." };
  }
  const late = f.lateMinutes.trim();
  if (!/^\d{1,3}$/.test(late) || Number(late) < MIN_LATE_MINUTES || Number(late) > MAX_LATE_MINUTES) {
    return { field: "ticket_late_minutes", message: `Enter a whole number of minutes from ${MIN_LATE_MINUTES} to ${MAX_LATE_MINUTES}.` };
  }
  return null;
}

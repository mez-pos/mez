// How the venue runs its shifts, shares its tips and times its tickets
// (backoffice-settings.md, "Shift, tips and receipt").

import type { RulesRequest, RulesResponse } from "../../../shared/api-types";
import { Choice, Field, inputClass } from "../FormFields";
import {
  MAX_LATE_MINUTES,
  MAX_WEIGHT,
  MIN_LATE_MINUTES,
  MIN_WEIGHT,
  TIP_RULES,
  rulesChanged,
  rulesFormFrom,
  rulesProblem,
  rulesProblemOf,
  toRulesInput,
} from "./rules-logic";
import { SectionCard } from "./SectionCard";
import { useSectionDraft } from "./use-section-draft";

export interface RulesCardProps {
  rules: RulesResponse;
  onSave: (input: RulesRequest) => Promise<RulesResponse>;
  onDirty: (dirty: boolean) => void;
}

export function RulesCard({ rules, onSave, onDirty }: RulesCardProps) {
  const per = rules.minor_per_major;
  const d = useSectionDraft({
    saved: rulesFormFrom(rules),
    changed: (draft, saved) => rulesChanged(draft, saved, per),
    check: (f) => rulesProblem(f, per),
    send: (f) => onSave(toRulesInput(f, per)).then(rulesFormFrom),
    problemOf: rulesProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<typeof form>) => d.update((f) => ({ ...f, ...patch }));

  return (
    <SectionCard
      title="Shifts, tips and orders"
      description="When a day starts, what a short drawer needs, how tips are shared and when an order runs late."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Field
        label="The trading day starts at"
        hint={
          rules.open_shift_day
            ? `Sales before this time belong to the day before. The shift open now keeps its day, ${rules.open_shift_day}: a new start applies from the next shift. Reports use the new start for every day.`
            : "Sales before this time belong to the day before. A shift keeps its day when this changes: a new start applies from the next shift. Reports use the new start for every day."
        }
        error={d.on("day_start")}
      >
        {(id) => (
          <input
            id={id}
            type="time"
            className={`${inputClass} max-w-40 font-mono`}
            value={form.dayStart}
            disabled={busy}
            onChange={(e) => set({ dayStart: e.target.value })}
          />
        )}
      </Field>

      <Field
        label={`A drawer may be out by (${rules.currency})`}
        hint="Over or short by more than this, closing the shift needs a reason. 0 asks for a reason on any difference."
        error={d.on("variance_threshold")}
      >
        {(id) => (
          <input
            id={id}
            className={`${inputClass} max-w-40 font-mono`}
            value={form.variance}
            disabled={busy}
            inputMode="decimal"
            autoComplete="off"
            onChange={(e) => set({ variance: e.target.value })}
          />
        )}
      </Field>

      <Choice label="Tips are shared" value={form.tipRule} options={TIP_RULES} onChange={(tipRule) => set({ tipRule })} error={d.on("tip_rule")} />
      {form.tipRule === "hours" && <p className="text-sm text-fg-muted">Each person's share follows the hours entered when the tips are pooled.</p>}
      {form.tipRule === "role_weight" && (
        <fieldset className="flex flex-col gap-2" disabled={busy}>
          <legend className="text-base font-semibold">Weight of each role</legend>
          <ul className="flex flex-col gap-2">
            {Object.keys(form.weights).map((role) => (
              <li key={role} className="flex items-center gap-3">
                <label htmlFor={`weight-${role}`} className="min-w-0 flex-1 text-base">
                  {role}
                </label>
                <input
                  id={`weight-${role}`}
                  className={`${inputClass} max-w-24 font-mono`}
                  value={form.weights[role] ?? ""}
                  placeholder="1"
                  inputMode="numeric"
                  autoComplete="off"
                  onChange={(e) => d.update((f) => ({ ...f, weights: { ...f.weights, [role]: e.target.value } }))}
                />
              </li>
            ))}
          </ul>
          <p className="text-sm text-fg-muted">
            A whole number from {MIN_WEIGHT} to {MAX_WEIGHT}. A role left empty counts as 1: a manager at 2 takes twice a cashier's share.
          </p>
          {d.on("tip_role_weights") && (
            <p role="alert" className="text-sm text-danger">
              {d.on("tip_role_weights")}
            </p>
          )}
        </fieldset>
      )}

      <Field
        label="An order is late after (minutes)"
        hint={`The order screen shows its time as late once it has been open this long. ${MIN_LATE_MINUTES} to ${MAX_LATE_MINUTES}.`}
        error={d.on("ticket_late_minutes")}
      >
        {(id) => (
          <input
            id={id}
            className={`${inputClass} max-w-28 font-mono`}
            value={form.lateMinutes}
            disabled={busy}
            inputMode="numeric"
            autoComplete="off"
            onChange={(e) => set({ lateMinutes: e.target.value })}
          />
        )}
      </Field>
    </SectionCard>
  );
}

// The venue's own details: its name, its legal name, its timezone, and the
// currency it trades in, which is shown and never edited (backoffice-settings.md).

import { useEffect, useMemo, useState } from "react";
import type { SettingsVenueResponse, VenueDetailsRequest } from "../../../shared/api-types";
import { Field, inputClass } from "../FormFields";
import { SectionCard } from "./SectionCard";
import { problemOf, timezoneNames, toVenueInput, venueChanged, venueFormFrom, venueProblem, type Problem, type VenueForm } from "./settings-logic";

export interface VenueSectionProps {
  venue: SettingsVenueResponse;
  /** Saves, and resolves with the venue as the server now holds it. */
  onSave: (input: VenueDetailsRequest) => Promise<SettingsVenueResponse>;
  /** Told whenever the draft starts or stops differing from what is saved. */
  onDirty: (dirty: boolean) => void;
}

const zoneListId = "venue-timezones";

export function VenueSection({ venue, onSave, onDirty }: VenueSectionProps) {
  const saved = venueFormFrom(venue);
  const [form, setForm] = useState<VenueForm>(saved);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const zones = useMemo(() => timezoneNames(), []);

  const dirty = venueChanged(form, saved);
  useEffect(() => {
    onDirty(dirty);
    return () => onDirty(false);
  }, [dirty, onDirty]);

  const set = (patch: Partial<VenueForm>) => {
    setForm((f) => ({ ...f, ...patch }));
    setProblem(null);
    setDone(false);
  };
  const save = () => {
    const refused = venueProblem(form);
    if (refused) {
      setProblem(refused);
      return;
    }
    setBusy(true);
    setProblem(null);
    onSave(toVenueInput(form))
      .then((now) => {
        setForm(venueFormFrom(now));
        setDone(true);
      })
      .catch((err: unknown) => setProblem(problemOf(err)))
      .finally(() => setBusy(false));
  };
  const discard = () => {
    setForm(saved);
    setProblem(null);
    setDone(false);
  };
  const on = (field: NonNullable<Problem["field"]>) => (problem?.field === field ? problem.message : null);

  return (
    <SectionCard
      title="Venue"
      description="Who the venue is, as it appears on the lock screen and on invoices."
      dirty={dirty}
      busy={busy}
      saved={done}
      error={problem && problem.field === null ? problem.message : null}
      onSave={save}
      onDiscard={discard}
    >
      <Field label="Venue name" hint="Shown on the lock screen and on receipts." error={on("name")}>
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.name}
            disabled={busy}
            autoComplete="off"
            onChange={(e) => set({ name: e.target.value })}
          />
        )}
      </Field>
      <Field label="Legal name" hint="Printed on invoices. Leave empty to use the venue's name." error={on("legal_name")}>
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.legalName}
            disabled={busy}
            autoComplete="off"
            onChange={(e) => set({ legalName: e.target.value })}
          />
        )}
      </Field>
      <Field label="Timezone" hint="Like Asia/Kolkata. Decides where a day starts and what time a receipt shows." error={on("timezone")}>
        {(id) => (
          <>
            <input
              id={id}
              className={inputClass}
              list={zoneListId}
              value={form.timezone}
              disabled={busy}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => set({ timezone: e.target.value })}
            />
            <datalist id={zoneListId}>
              {zones.map((z) => (
                <option key={z} value={z} />
              ))}
            </datalist>
          </>
        )}
      </Field>
      <div className="flex flex-col gap-1.5">
        <p className="text-base font-semibold">Currency</p>
        <p className="font-mono text-lg" data-testid="venue-currency">
          {venue.currency}
        </p>
        <p className="text-sm text-fg-muted">Cannot be changed: every amount recorded would change what it means.</p>
      </div>
    </SectionCard>
  );
}

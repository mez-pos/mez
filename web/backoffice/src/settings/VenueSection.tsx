// The venue's own details: its name, its legal name, its timezone, and the
// currency it trades in, which is shown and never edited (backoffice-settings.md).

import { useMemo } from "react";
import type { SettingsVenueResponse, VenueDetailsRequest } from "../../../shared/api-types";
import { Field, inputClass } from "../FormFields";
import { SectionCard } from "./SectionCard";
import { problemOf, timezoneNames, toVenueInput, venueChanged, venueFormFrom, venueProblem } from "./settings-logic";
import { useSectionDraft } from "./use-section-draft";

export interface VenueSectionProps {
  venue: SettingsVenueResponse;
  /** Saves, and resolves with the venue as the server now holds it. */
  onSave: (input: VenueDetailsRequest) => Promise<SettingsVenueResponse>;
  /** Told whenever the draft starts or stops differing from what is saved. */
  onDirty: (dirty: boolean) => void;
}

const zoneListId = "venue-timezones";

export function VenueSection({ venue, onSave, onDirty }: VenueSectionProps) {
  const d = useSectionDraft({
    saved: venueFormFrom(venue),
    changed: venueChanged,
    check: venueProblem,
    send: (form) => onSave(toVenueInput(form)).then(venueFormFrom),
    problemOf,
    onDirty,
  });
  const zones = useMemo(() => timezoneNames(), []);
  const { form, busy } = d;

  return (
    <SectionCard
      title="Venue"
      saves="venue details"
      description="Who the venue is, as it appears on the lock screen and on invoices."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
      columns={2}
    >
      <Field label="Venue name" hint="Shown on the lock screen and on receipts." error={d.on("name")}>
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.name}
            disabled={busy}
            autoComplete="off"
            onChange={(e) => d.update((f) => ({ ...f, name: e.target.value }))}
          />
        )}
      </Field>
      <Field label="Legal name" hint="Printed on invoices. Leave empty to use the venue's name." error={d.on("legal_name")}>
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.legalName}
            disabled={busy}
            autoComplete="off"
            onChange={(e) => d.update((f) => ({ ...f, legalName: e.target.value }))}
          />
        )}
      </Field>
      <Field label="Timezone" hint="Like Asia/Kolkata. Decides where a day starts and what time a receipt shows." error={d.on("timezone")}>
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
              onChange={(e) => d.update((f) => ({ ...f, timezone: e.target.value }))}
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

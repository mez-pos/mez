// The venue's tax profile (backoffice-settings.md, "Tax and numbering"): which
// regime it is under and what that regime needs on an invoice. A profile that
// prints an HSN lists the items still without one, each a link to its form.

import { Link } from "wouter";
import type { TaxSettingsRequest, TaxSettingsResponse } from "../../../shared/api-types";
import { Button } from "../../../shared/ui";
import { Choice, Field, inputClass } from "../FormFields";
import { SectionCard } from "./SectionCard";
import {
  PRICINGS,
  PROFILES,
  asksPricing,
  menuItemPath,
  namedItems,
  registrationLabel,
  showWithoutHSN,
  taxChanged,
  taxFormFrom,
  taxProblem,
  taxProblemOf,
  toTaxInput,
  type JurisdictionRow,
  type TaxForm,
} from "./tax-logic";
import { useSectionDraft } from "./use-section-draft";
import { compositionDeclaration } from "../../../shared/tax-words";

/** Where the registration number is printed, under each profile that has one. */
const registrationHint: Partial<Record<string, string>> = {
  gst_in: "Invoices print it. Capitals and spaces are tidied for you.",
  gst_in_composition: "Every bill shows it. Capitals and spaces are tidied for you.",
};

export interface TaxProfileCardProps {
  tax: TaxSettingsResponse;
  onSave: (input: TaxSettingsRequest) => Promise<TaxSettingsResponse>;
  onDirty: (dirty: boolean) => void;
}

export function TaxProfileCard({ tax, onSave, onDirty }: TaxProfileCardProps) {
  const saved = taxFormFrom(tax);
  const d = useSectionDraft({
    saved,
    changed: taxChanged,
    check: taxProblem,
    send: (form) => onSave(toTaxInput(form)).then(taxFormFrom),
    problemOf: taxProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<TaxForm>) => d.update((f) => ({ ...f, ...patch }));
  const registration = registrationLabel(form.profile);

  return (
    <SectionCard
      title="Tax profile"
      saves="tax profile"
      description="Which tax the venue charges, and what its invoices must show."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Choice label="Tax" value={form.profile} options={PROFILES} onChange={(profile) => set({ profile })} error={d.on("profile")} />
      {form.profile !== saved.profile && (
        <p className="text-sm text-fg-2">
          Changing the profile is refused while any order is open. Settle or void them first. Orders already paid keep the tax they were charged.
        </p>
      )}

      {form.profile === "sales_tax_us" && <p className="text-base text-fg-2">Tax is always added on top of the price under US sales tax.</p>}
      {form.profile === "gst_in_composition" && (
        <p className="text-base text-fg-2">
          No GST is added to a price. Every bill is a bill of supply: it shows the GSTIN and says &ldquo;{compositionDeclaration}&rdquo;.
        </p>
      )}
      {asksPricing(form.profile) && (
        <Choice label="Prices" value={form.pricing} options={PRICINGS} onChange={(pricing) => set({ pricing })} error={d.on("pricing")} />
      )}

      {registration && (
        <Field label={registration} hint={registrationHint[form.profile] ?? "Receipts print it."} error={d.on("registration_id")}>
          {(id) => (
            <input
              id={id}
              className={`${inputClass} font-mono`}
              value={form.registrationId}
              disabled={busy}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => set({ registrationId: e.target.value })}
            />
          )}
        </Field>
      )}
      {form.profile === "gst_in" && (
        <Field label="Place of supply" hint="The two digit state code, like 29. Empty takes the first two digits of the GSTIN." error={d.on("state_code")}>
          {(id) => (
            <input
              id={id}
              className={`${inputClass} max-w-32 font-mono`}
              value={form.stateCode}
              disabled={busy}
              maxLength={2}
              inputMode="numeric"
              autoComplete="off"
              onChange={(e) => set({ stateCode: e.target.value })}
            />
          )}
        </Field>
      )}
      {form.profile === "sales_tax_us" && (
        <Jurisdiction rows={form.jurisdiction} disabled={busy} error={d.on("jurisdiction")} onChange={(jurisdiction) => set({ jurisdiction })} />
      )}

      {showWithoutHSN(tax, form.profile) && <WithoutHSN tax={tax} />}
    </SectionCard>
  );
}

/** The rates a US venue charges: each authority and its percentage. */
function Jurisdiction({
  rows,
  disabled,
  error,
  onChange,
}: {
  rows: JurisdictionRow[];
  disabled: boolean;
  error: string | null;
  onChange: (rows: JurisdictionRow[]) => void;
}) {
  const edit = (i: number, patch: Partial<JurisdictionRow>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  return (
    <div className="flex flex-col gap-2">
      <p className="text-base font-semibold">Rates where the venue is</p>
      <ul className="flex flex-col gap-2">
        {rows.map((r, i) => (
          <li key={i} className="flex items-center gap-2">
            <input
              aria-label={`Name of rate ${i + 1}`}
              className={`${inputClass} min-w-0 flex-1`}
              value={r.name}
              disabled={disabled}
              placeholder="CA"
              maxLength={30}
              onChange={(e) => edit(i, { name: e.target.value })}
            />
            <input
              aria-label={`Percent of rate ${i + 1}`}
              className={`${inputClass} max-w-28 font-mono`}
              value={r.rate}
              disabled={disabled}
              placeholder="7.25"
              inputMode="decimal"
              onChange={(e) => edit(i, { rate: e.target.value })}
            />
            <span aria-hidden="true">%</span>
            <Button aria-label={`Remove rate ${i + 1}`} variant="ghost" disabled={disabled} onClick={() => onChange(rows.filter((_, j) => j !== i))}>
              Remove
            </Button>
          </li>
        ))}
      </ul>
      <div>
        <Button disabled={disabled} onClick={() => onChange([...rows, { name: "", rate: "" }])}>
          Add a rate
        </Button>
      </div>
      <p className="text-sm text-fg-muted">One for the state, one for the county, one for the city: each is added together on the bill.</p>
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

/** What the menu still owes a profile that prints an HSN or SAC code on every line. */
function WithoutHSN({ tax }: { tax: TaxSettingsResponse }) {
  const { shown, more } = namedItems(tax.items_without_hsn);
  const n = tax.items_without_hsn.length;
  return (
    <div role="status" className="rounded border border-line bg-sunken p-4">
      <p className="text-base font-semibold">{n === 1 ? "1 item has" : `${n} items have`} no HSN or SAC code</p>
      <p className="mt-1 text-sm text-fg-2">They still sell, but their invoices fall short until each is edited.</p>
      <ul className="mt-3 flex flex-wrap gap-2">
        {shown.map((it) => (
          <li key={it.id}>
            <Link
              href={menuItemPath(it.id)}
              className="inline-flex min-h-tap items-center rounded-pill border border-line-strong bg-raised px-4 text-base text-attention-text underline hover:bg-attention-tint"
            >
              {it.name}
            </Link>
          </li>
        ))}
        {more > 0 && <li className="inline-flex min-h-tap items-center px-2 text-base text-fg-2">and {more} more in the menu</li>}
      </ul>
    </div>
  );
}

// How invoices and credit notes are numbered, the financial year they restart
// in, and the cash rounding step (backoffice-settings.md, "Invoice numbering").
// The next numbers are shown as the draft is typed, asked of the server.

import type { ReactNode } from "react";
import type { NumberingRequest, NumberingResponse } from "../../../shared/api-types";
import { Field, inputClass } from "../FormFields";
import { useAway } from "../../../shared/use-realtime";
import { SectionCard } from "./SectionCard";
import {
  MONTHS,
  numberingChanged,
  numberingFormFrom,
  numberingProblem,
  numberingProblemOf,
  previewKey,
  stepRange,
  stepText,
  toNumberingInput,
  withSeries,
} from "./numbering-logic";
import { usePreview, type Preview } from "./use-preview";
import { useSectionDraft } from "./use-section-draft";
import type { NumberingApi } from "./use-tax";

export interface NumberingCardProps {
  numbering: NumberingResponse;
  api: Pick<NumberingApi, "save" | "preview">;
  onDirty: (dirty: boolean) => void;
}

export function NumberingCard({ numbering: n, api, onDirty }: NumberingCardProps) {
  const away = useAway();
  const max = n.max_rounding_step;
  const d = useSectionDraft({
    saved: numberingFormFrom(n),
    changed: numberingChanged,
    check: (f) => numberingProblem(f, max),
    send: (f) => api.save(toNumberingInput(f, max)).then(numberingFormFrom),
    problemOf: numberingProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<typeof form>) => d.update((f) => ({ ...f, ...patch }));
  const stepOk = numberingProblem(form, max) === null;
  const input: NumberingRequest | null = stepOk ? toNumberingInput(form, max) : null;
  const preview = usePreview({
    active: d.dirty,
    input,
    missing: "Fix the rounding step to see the next numbers.",
    key: previewKey(form, max),
    ask: api.preview,
  });

  return (
    <SectionCard
      readOnly={away}
      title="Invoice numbering"
      description="How invoices and credit notes are numbered, and when the numbers start again."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Series
        label="Invoices"
        series={form.invoiceSeries}
        prefix={form.invoicePrefix}
        disabled={busy}
        seriesError={d.on("invoice_series")}
        prefixError={d.on("invoice_prefix")}
        onSeries={(v) => d.update((f) => withSeries(f, "invoice", v))}
        onPrefix={(v) => set({ invoicePrefix: v })}
      />
      <Series
        label="Credit notes"
        series={form.creditSeries}
        prefix={form.creditPrefix}
        disabled={busy}
        seriesError={d.on("credit_series")}
        prefixError={d.on("credit_prefix")}
        onSeries={(v) => d.update((f) => withSeries(f, "credit", v))}
        onPrefix={(v) => set({ creditPrefix: v })}
      />
      <Field
        label="Number format"
        hint={`{prefix}, then {fy:short} for 26-27 or {fy} for 2026-27, then {seq:06d} for the number in six digits.${
          n.number_limit > 0 ? ` Under GST a number is at most ${n.number_limit} characters.` : ""
        }`}
        error={d.on("template")}
      >
        {(id) => (
          <input
            id={id}
            className={`${inputClass} font-mono`}
            value={form.template}
            disabled={busy}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => set({ template: e.target.value })}
          />
        )}
      </Field>
      <Field
        label="Financial year starts in"
        hint={
          n.fy_locked
            ? "Numbers have been issued, and numbering restarts on this date. It can no longer change here."
            : "Numbering starts again at 1 on the 1st of this month. Fixed once the first invoice is issued."
        }
        error={d.on("financial_year_start_month")}
      >
        {(id) => (
          <select
            id={id}
            className={`${inputClass} max-w-56`}
            value={form.month}
            disabled={busy || n.fy_locked}
            onChange={(e) => set({ month: Number(e.target.value) })}
          >
            {MONTHS.map((m, i) => (
              <option key={m} value={i + 1}>
                {m}
              </option>
            ))}
          </select>
        )}
      </Field>
      <Field
        label={`Round cash to the nearest (${n.currency})`}
        hint={`${stepRange(max)}. The smallest, ${stepText(1, max)}, rounds nothing. Saved now; payments do not apply it yet.`}
        error={d.on("cash_rounding_step")}
      >
        {(id) => (
          <input
            id={id}
            className={`${inputClass} max-w-32 font-mono`}
            value={form.step}
            disabled={busy}
            inputMode="decimal"
            autoComplete="off"
            onChange={(e) => set({ step: e.target.value })}
          />
        )}
      </Field>

      <NextNumbers dirty={d.dirty} saved={n} preview={preview} />
    </SectionCard>
  );
}

function Series(props: {
  label: string;
  series: string;
  prefix: string;
  disabled: boolean;
  seriesError: string | null;
  prefixError: string | null;
  onSeries: (v: string) => void;
  onPrefix: (v: string) => void;
}) {
  return (
    <div className="flex flex-col gap-3">
      <p className="text-base font-semibold">{props.label}</p>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label="Series code" hint="Capitals and digits, up to 8. Its own counter." error={props.seriesError}>
          {(id) => (
            <input
              id={id}
              className={`${inputClass} font-mono uppercase`}
              value={props.series}
              disabled={props.disabled}
              maxLength={8}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => props.onSeries(e.target.value)}
            />
          )}
        </Field>
        <Field label="Prefix" hint="What {prefix} writes. Follows the code unless you change it." error={props.prefixError}>
          {(id) => (
            <input
              id={id}
              className={`${inputClass} font-mono`}
              value={props.prefix}
              disabled={props.disabled}
              maxLength={8}
              autoComplete="off"
              spellCheck={false}
              onChange={(e) => props.onPrefix(e.target.value)}
            />
          )}
        </Field>
      </div>
    </div>
  );
}

/** The next numbers: as saved when nothing is typed, as the server works them out when it is. */
function NextNumbers({ dirty, saved, preview }: { dirty: boolean; saved: NumberingResponse; preview: Preview<NumberingResponse> }) {
  const heading = dirty ? "If you save, the next numbers are" : "The next numbers are";
  let body: ReactNode;
  if (!dirty) {
    body = <Numbers invoice={saved.next_invoice_no} credit={saved.next_credit_no} />;
  } else if (preview.state === "ok") {
    body = <Numbers invoice={preview.value.next_invoice_no} credit={preview.value.next_credit_no} />;
  } else if (preview.state === "error") {
    body = <p className="text-base text-fg-2">No preview yet: {preview.message}</p>;
  } else {
    body = <p className="text-base text-fg-muted">Working out the next numbers</p>;
  }
  return (
    <div className="rounded border border-line bg-sunken p-4">
      <p className="text-meta font-semibold uppercase text-fg-faint">{heading}</p>
      <div aria-live="polite" className="mt-2">
        {body}
      </div>
    </div>
  );
}

function Numbers({ invoice, credit }: { invoice: string; credit: string }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-base">
      <dt className="text-fg-2">Invoice</dt>
      <dd className="font-mono">{invoice}</dd>
      <dt className="text-fg-2">Credit note</dt>
      <dd className="font-mono">{credit}</dd>
    </dl>
  );
}

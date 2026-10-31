// What the venue adds to its receipts and credit notes, with a sample receipt
// laid out at the printer's width as it is typed (backoffice-settings.md, "Receipt").

import { useState } from "react";
import type { ReceiptRequest, ReceiptResponse } from "../../../shared/api-types";
import { CheckField, Choice, Field, inputClass } from "../FormFields";
import { ReceiptPaper } from "./ReceiptPaper";
import {
  PAPERS,
  receiptChanged,
  receiptFormFrom,
  receiptPreviewKey,
  receiptProblem,
  receiptProblemOf,
  toPreviewInput,
  toReceiptInput,
  type PaperMM,
  type ReceiptForm,
} from "./receipt-logic";
import { SectionCard } from "./SectionCard";
import { useSectionDraft } from "./use-section-draft";
import { usePreview } from "./use-preview";
import type { ReceiptApi } from "./use-shifts";

export interface ReceiptCardProps {
  receipt: ReceiptResponse;
  api: Pick<ReceiptApi, "save" | "preview">;
  onDirty: (dirty: boolean) => void;
}

export function ReceiptCard({ receipt, api, onDirty }: ReceiptCardProps) {
  const limits = { maxLines: receipt.max_lines, maxChars: receipt.max_line_chars };
  const d = useSectionDraft({
    saved: receiptFormFrom(receipt),
    changed: receiptChanged,
    check: (f) => receiptProblem(f, limits),
    send: (f) => api.save(toReceiptInput(f) satisfies ReceiptRequest).then(receiptFormFrom),
    problemOf: receiptProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<ReceiptForm>) => d.update((f) => ({ ...f, ...patch }));
  const [paper, setPaper] = useState<PaperMM>(80);

  const clientProblem = receiptProblem(form, limits);
  const preview = usePreview({
    active: true,
    input: clientProblem ? null : toPreviewInput(form, paper),
    missing: clientProblem?.message ?? "",
    key: receiptPreviewKey(form, paper),
    ask: api.preview,
  });

  return (
    <SectionCard
      title="Receipt"
      saves="receipt"
      description="What the receipt and the credit note say beyond what your tax profile requires. That always prints: the name, registration, number, date, tax and total."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Field
        label="Under the venue's name"
        hint={`One line per row, up to ${limits.maxLines} lines of ${limits.maxChars} characters: an address, a phone number. The printer prints plain letters and digits: accents print without them, and other scripts cannot be printed yet.`}
        error={d.on("header")}
      >
        {(id) => (
          <textarea
            id={id}
            className={`${inputClass} min-h-28 py-3`}
            rows={3}
            value={form.header}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => set({ header: e.target.value })}
          />
        )}
      </Field>
      <Field label="At the bottom" hint="Thanks, the Wi-Fi password, opening hours. Same limits." error={d.on("footer")}>
        {(id) => (
          <textarea
            id={id}
            className={`${inputClass} min-h-28 py-3`}
            rows={3}
            value={form.footer}
            disabled={busy}
            spellCheck={false}
            onChange={(e) => set({ footer: e.target.value })}
          />
        )}
      </Field>

      <div className="flex flex-col">
        <p className="text-base font-semibold">Print</p>
        <CheckField
          label="The invoice number as a barcode"
          hint="So a refund can scan the guest's copy instead of typing the number."
          checked={form.barcode}
          disabled={busy}
          onChange={(barcode) => set({ barcode })}
        />
        <CheckField
          label="The order reference"
          hint="The order's id under the date. The table number always prints."
          checked={form.orderRef}
          disabled={busy}
          onChange={(orderRef) => set({ orderRef })}
        />
        <CheckField
          label="The choices under each item"
          hint="Oat milk, extra hot, and the like."
          checked={form.modifiers}
          disabled={busy}
          onChange={(modifiers) => set({ modifiers })}
        />
      </div>

      <div className="flex flex-col gap-3 rounded border border-line bg-sunken p-4">
        <p className="text-meta font-semibold uppercase text-fg-faint">Preview</p>
        <Choice
          label="Paper"
          value={String(paper)}
          options={PAPERS.map((p) => ({ value: String(p.value), label: p.label }))}
          onChange={(v: string) => setPaper(v === "58" ? 58 : 80)}
          error={null}
        />
        <div aria-live="polite">
          {preview.state === "ok" && <ReceiptPaper preview={preview.value} />}
          {preview.state === "loading" && <p className="text-base text-fg-muted">Laying out a sample receipt</p>}
          {preview.state === "error" && <p className="text-base text-fg-2">No preview yet: {preview.message}</p>}
        </div>
        <p className="text-sm text-fg-muted">
          A sample bill laid out by the same code that prints a real one. The venue's name, tax details and next invoice number are real; the items and amounts
          are made up.
        </p>
      </div>
    </SectionCard>
  );
}

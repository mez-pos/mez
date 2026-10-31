// Where the venue is paid when a guest pays from a phone: its UPI address,
// and the name a guest's app shows; and when a guest who orders from their
// phone pays (backoffice-settings.md, "Paid from a phone"; ADR-215,
// ADR-229).

import type { PaymentSettingsRequest, PaymentSettingsResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { clockText } from "../../../shared/venue-text";
import { Choice, Field, inputClass } from "../FormFields";
import {
  PAY_MODES,
  modeWords,
  onlineWords,
  payeeShown,
  paymentsChanged,
  paymentsFormFrom,
  paymentsProblem,
  paymentsProblemOf,
  paymentsStanding,
  toPaymentsInput,
  type PaymentsForm,
} from "./payments-logic";
import { useAway } from "../../../shared/use-realtime";
import { SectionCard } from "./SectionCard";
import { useSectionDraft } from "./use-section-draft";

export interface PaymentsCardProps {
  payments: PaymentSettingsResponse;
  /** Saves, and resolves with the settings as the server now holds them. */
  onSave: (input: PaymentSettingsRequest) => Promise<PaymentSettingsResponse>;
  onDirty: (dirty: boolean) => void;
}

export function PaymentsCard({ payments, onSave, onDirty }: PaymentsCardProps) {
  const away = useAway();
  const saved = paymentsFormFrom(payments);
  const d = useSectionDraft({
    saved,
    changed: paymentsChanged,
    check: (f) => paymentsProblem(f, payments.max_payee),
    send: (f) => onSave(toPaymentsInput(f)).then(paymentsFormFrom),
    problemOf: paymentsProblemOf,
    onDirty,
  });
  const { form, busy } = d;
  const set = (patch: Partial<PaymentsForm>) => d.update((f) => ({ ...f, ...patch }));
  const mode = modeWords(form.mode, payments.can_charge);
  const online = onlineWords(payments.online, (at) => clockText(at, null));

  return (
    <SectionCard
      readOnly={away}
      title="Paid from a phone"
      saves="phone payments"
      description="A guest reading their bill on their phone can pay it by UPI, straight to your account. They show you the payment, and you record it on the terminal as you would one made at the counter."
      dirty={d.dirty}
      busy={busy}
      saved={d.done}
      error={d.footerError}
      onSave={d.save}
      onDiscard={d.discard}
    >
      <Field
        label="UPI address"
        hint="Like cafe@okbank: the one on your counter's UPI sticker. Leave empty to offer no UPI on a guest's phone."
        error={d.on("upi_address")}
      >
        {(id) => (
          <input
            id={id}
            className={`${inputClass} font-mono`}
            value={form.address}
            disabled={busy}
            autoComplete="off"
            autoCapitalize="none"
            spellCheck={false}
            inputMode="email"
            onChange={(e) => set({ address: e.target.value })}
          />
        )}
      </Field>
      <Field
        label="Name the guest sees"
        hint={`What a guest's UPI app shows before they pay. Leave empty to use the venue's name, ${payments.venue_name}.`}
        error={d.on("upi_payee")}
      >
        {(id) => (
          <input
            id={id}
            className={inputClass}
            value={form.payee}
            placeholder={payments.venue_name}
            maxLength={payments.max_payee}
            disabled={busy}
            autoComplete="off"
            onChange={(e) => set({ payee: e.target.value })}
          />
        )}
      </Field>

      <div className="rounded border border-chip-warning-border bg-chip-warning-bg p-4 text-chip-warning-fg">
        <p className="text-base font-semibold">mez cannot check this address.</p>
        <p className="mt-1 text-base">
          It only checks that it is written like one. A wrong address sends your guests' money to someone else. After saving, open a bill on your own phone and
          pay it one rupee: your UPI app should show {payeeShown(form, payments.venue_name)}, and the rupee should arrive in your account.
        </p>
      </div>

      <p role="status" className="text-base text-fg-2">
        {paymentsStanding(saved, payments.currency)}
      </p>

      <section
        aria-label="Paying online"
        className={cx(
          "rounded border p-4",
          online.tone === "warning" ? "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg" : "border-line bg-sunken text-fg-2",
        )}
      >
        <p className={cx("text-base font-semibold", online.tone === "good" && "text-accent-text", online.tone === "none" && "text-fg")}>{online.title}</p>
        {online.lines.map((line) => (
          <p key={line} className="mt-1 text-base">
            {line}
          </p>
        ))}
      </section>

      <div className="flex flex-col gap-3 border-t border-line-soft pt-5">
        <Choice
          label="A guest who orders from their phone pays"
          value={form.mode}
          options={PAY_MODES}
          onChange={(v) => set({ mode: v })}
          error={d.on("mode")}
        />
        <p
          role="status"
          className={cx("text-base", mode.inert ? "rounded border border-chip-warning-border bg-chip-warning-bg p-4 text-chip-warning-fg" : "text-fg-2")}
        >
          {mode.text}
        </p>
      </div>
    </SectionCard>
  );
}

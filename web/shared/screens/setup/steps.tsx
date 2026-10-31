// The wizard's three steps (setup-wizard.md). Each draws its fields from the
// answers and reports a change; none of them holds anything or asks the server.

import { CardChoice, SelectField, TextField } from "./fields";
import {
  currencies,
  maxOwnerPin,
  minOwnerPin,
  months,
  profileOrder,
  profiles,
  type Answers,
  type Pricing,
  type Problems,
  type TaxProfile,
} from "./setup-logic";

export interface StepProps {
  answers: Answers;
  problems: Problems;
  /** Sets one answer, as typed by the owner. */
  onChange: <K extends keyof Answers>(key: K, value: Answers[K]) => void;
}

export function VenueStep({ answers, problems, onChange, zones }: StepProps & { zones: readonly { value: string; label: string }[] }) {
  // A currency the list does not have is still shown, so it is never silently changed.
  const offered = currencies.some((c) => c.code === answers.currency) ? currencies : [{ code: answers.currency, name: answers.currency }, ...currencies];
  return (
    <div className="grid gap-5 md:grid-cols-2">
      <div className="md:col-span-2">
        <TextField
          label="Venue name"
          value={answers.name}
          onChange={(v) => onChange("name", v)}
          error={problems.name}
          hint="As staff and guests see it, and as it prints on a receipt."
          maxLength={60}
          autoFocus
        />
      </div>
      <div className="md:col-span-2">
        <TextField
          label="Name on invoices"
          optional
          value={answers.legalName}
          onChange={(v) => onChange("legalName", v)}
          error={problems.legal_name}
          hint="The registered name, if it differs from the venue's."
          maxLength={120}
        />
      </div>
      <SelectField
        label="Currency"
        value={answers.currency}
        onChange={(v) => onChange("currency", v)}
        error={problems.currency}
        options={offered.map((c) => ({ value: c.code, label: c.code === c.name ? c.code : `${c.code} · ${c.name}` }))}
      />
      {zones.length > 0 ? (
        // Chosen from a list in words, this computer's own first chosen; never
        // an IANA name to type (SET-8).
        <SelectField
          label="Time zone"
          value={answers.timezone}
          onChange={(v) => onChange("timezone", v)}
          error={problems.timezone}
          hint="The venue's own. The day's sales and every invoice's time follow it."
          options={zones}
        />
      ) : (
        // A browser that cannot list the zones: the name is typed.
        <TextField
          label="Time zone"
          value={answers.timezone}
          onChange={(v) => onChange("timezone", v)}
          error={problems.timezone}
          hint="The venue's own, such as Asia/Kolkata. The day's sales and every invoice's time follow it."
          placeholder="Asia/Kolkata"
        />
      )}
      <div className="md:col-span-2">
        <TextField
          label="What this till is called"
          value={answers.deviceName}
          onChange={(v) => onChange("deviceName", v)}
          error={problems.device_name}
          hint="This computer's name on reports and in the list of devices."
          maxLength={40}
        />
      </div>
    </div>
  );
}

const pricingOptions: readonly { value: Pricing; label: string; blurb: string }[] = [
  { value: "inclusive", label: "Prices include tax", blurb: "The menu price is what the guest pays." },
  { value: "exclusive", label: "Tax is added", blurb: "Tax goes on top of the menu price at the till." },
];

export function TaxesStep({ answers, problems, onChange, onProfile }: StepProps & { onProfile: (profile: TaxProfile) => void }) {
  const facts = profiles[answers.taxProfile];
  return (
    <div className="grid gap-5 md:grid-cols-2">
      <div className="md:col-span-2">
        <CardChoice
          label="Tax"
          value={answers.taxProfile}
          onChange={onProfile}
          options={profileOrder.map((p) => ({ value: p, label: profiles[p].label, blurb: profiles[p].blurb }))}
        />
      </div>
      {facts.registration && (
        <div className="md:col-span-2">
          <TextField
            label={facts.registration.label}
            value={answers.taxRegistration}
            onChange={(v) => onChange("taxRegistration", v)}
            error={problems.tax_registration}
            hint={facts.registration.hint}
            className="font-mono uppercase"
            maxLength={40}
          />
        </div>
      )}
      {facts.pricingChoice && (
        <div className="md:col-span-2">
          <CardChoice label="Menu prices" value={answers.pricing} onChange={(v) => onChange("pricing", v)} options={pricingOptions} min="16rem" />
        </div>
      )}
      <SelectField
        label="Financial year starts in"
        value={String(answers.fyStartMonth)}
        onChange={(v) => onChange("fyStartMonth", Number(v))}
        error={problems.fy_start_month}
        hint="Invoice numbers start again at 1 on the first of this month."
        options={months.map((name, i) => ({ value: String(i + 1), label: name }))}
      />
      <TextField
        label="Invoice prefix"
        value={answers.invoicePrefix}
        onChange={(v) => onChange("invoicePrefix", v)}
        error={problems.invoice_prefix}
        hint="What every invoice number begins with. The rest of the pattern is in Settings."
        className="font-mono uppercase"
        maxLength={12}
      />
    </div>
  );
}

export function YouStep({ answers, problems, onChange }: StepProps) {
  const digits = (v: string) => v.replace(/\D/g, "").slice(0, maxOwnerPin);
  return (
    <div className="grid gap-5 md:grid-cols-2">
      <div className="md:col-span-2">
        <TextField
          label="Your name"
          value={answers.ownerName}
          onChange={(v) => onChange("ownerName", v)}
          error={problems.owner_name}
          hint="You are the owner: you can do everything, and you add everyone else in the back office."
          maxLength={60}
          autoFocus
        />
      </div>
      <TextField
        label="Your PIN"
        type="password"
        inputMode="numeric"
        autoComplete="new-password"
        value={answers.pin}
        onChange={(v) => onChange("pin", digits(v))}
        error={problems.owner_pin}
        hint={`${minOwnerPin} to ${maxOwnerPin} digits. You sign in with it at every till.`}
        className="font-mono tracking-widest"
      />
      <TextField
        label="Your PIN again"
        type="password"
        inputMode="numeric"
        autoComplete="new-password"
        value={answers.pinAgain}
        onChange={(v) => onChange("pinAgain", digits(v))}
        error={problems.pin_again}
        className="font-mono tracking-widest"
      />
    </div>
  );
}

// The back office's form fields: a labelled input with its hint and the
// server's refusal under it, and a row of choices. Used by every settings
// modal, so they all read and refuse alike.

import { useId, type ReactNode } from "react";
import { cx } from "../../shared/ui/cx";
import { focusButton } from "../../shared/ui/focus";
import { radioGroup } from "../../shared/ui/radio-group";

/** The kit's one-line field (TECH-23, ADR-406). */
export { inputClass } from "../../shared/ui/field";

/**
 * What a field's control takes beside its id (TECH-10, ADR-403): the hint and
 * the error it is described by, and whether it is refused, so a screen reader
 * reads them with it. Spread it on the control: `<input id={id} {...aria} />`.
 */
export interface FieldAria {
  "aria-describedby"?: string;
  "aria-invalid"?: true;
}

/** The ids a field's hint and error take, and what its control is described by. */
export function fieldAria(id: string, hint: boolean, error: boolean): FieldAria {
  const by = [hint ? `${id}-hint` : null, error ? `${id}-error` : null].filter((x) => x !== null).join(" ");
  return { ...(by !== "" ? { "aria-describedby": by } : {}), ...(error ? { "aria-invalid": true as const } : {}) };
}

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: string;
  error: string | null;
  children: (id: string, aria: FieldAria) => ReactNode;
}) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-base font-semibold">
        {label}
      </label>
      {children(id, fieldAria(id, Boolean(hint), Boolean(error)))}
      {hint && (
        <p id={`${id}-hint`} className="text-sm text-fg-muted">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

export interface ChoiceProps<T extends string> {
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (v: T) => void;
  error: string | null;
}

export function Choice<T extends string>({ label, value, options, onChange, error }: ChoiceProps<T>) {
  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-base font-semibold">{label}</p>
      <div role="radiogroup" {...radioGroup} aria-label={label} className="flex flex-wrap gap-tap-gap">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={value === o.value}
            onClick={() => onChange(o.value)}
            className={cx(
              "min-h-tap rounded border px-4 text-base",
              focusButton,
              value === o.value ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line-strong bg-raised text-fg hover:bg-raised-high",
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
      {error && (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
    </div>
  );
}

/** A box to tick, with what it does in words. The whole row is the target: 48px. */
export function CheckField({
  label,
  hint,
  checked,
  disabled,
  onChange,
}: {
  label: string;
  hint?: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  const id = useId();
  return (
    <div className="flex flex-col">
      <label htmlFor={id} className="flex min-h-tap cursor-pointer items-center gap-3 text-base">
        <input
          id={id}
          type="checkbox"
          className="size-6 shrink-0 accent-accent"
          checked={checked}
          disabled={disabled}
          onChange={(e) => onChange(e.target.checked)}
        />
        {label}
      </label>
      {hint && <p className="ml-9 text-sm text-fg-muted">{hint}</p>}
    </div>
  );
}

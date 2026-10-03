// The back office's form fields: a labelled input with its hint and the
// server's refusal under it, and a row of choices. Used by every settings
// modal, so they all read and refuse alike.

import { useId, type ReactNode } from "react";
import { cx } from "../../shared/ui/cx";

export const inputClass =
  "min-h-tap w-full rounded border border-line-strong bg-raised px-4 text-fg outline-accent-tint focus-visible:border-accent focus-visible:outline-3";

export function Field({ label, hint, error, children }: { label: string; hint?: string; error: string | null; children: (id: string) => ReactNode }) {
  const id = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-base font-semibold">
        {label}
      </label>
      {children(id)}
      {hint && <p className="text-sm text-fg-muted">{hint}</p>}
      {error && (
        <p role="alert" className="text-sm text-danger">
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
      <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-tap-gap">
        {options.map((o) => (
          <button
            key={o.value}
            type="button"
            role="radio"
            aria-checked={value === o.value}
            onClick={() => onChange(o.value)}
            className={cx(
              "min-h-tap rounded border px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
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

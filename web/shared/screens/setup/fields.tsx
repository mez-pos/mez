// The wizard's form fields: a label, the control, a hint, and what is wrong
// under it. The back office has its own (backoffice/src/FormFields.tsx); these
// are here because the wizard is a shared screen and a shared screen cannot
// reach into an app.

import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from "react";
import { cx } from "../../ui/cx";
import { radioGroup } from "../../ui/radio-group";
import { inputClass } from "../../ui/field";

// 16px text: a smaller field makes a phone's browser zoom in (tokens.md).
// The kit's field, at 16px so a phone does not zoom into it (tokens.md).
const control = `${inputClass} text-[1rem]`;

interface FrameProps {
  label: string;
  hint?: string;
  error?: string | undefined;
  optional?: boolean;
  children: (ids: { id: string; describedBy: string | undefined; invalid: boolean }) => ReactNode;
}

function FieldFrame({ label, hint, error, optional, children }: FrameProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const describedBy = [error ? errorId : null, hint ? hintId : null].filter(Boolean).join(" ") || undefined;
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-base font-semibold">
        {label}
        {optional && <span className="ml-2 font-normal text-fg-muted">Optional</span>}
      </label>
      {children({ id, describedBy, invalid: Boolean(error) })}
      {error && (
        <p id={errorId} role="alert" className="text-sm text-danger">
          {error}
        </p>
      )}
      {hint && (
        <p id={hintId} className="text-sm text-fg-muted">
          {hint}
        </p>
      )}
    </div>
  );
}

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "value" | "id"> {
  label: string;
  value: string;
  onChange: (value: string) => void;
  hint?: string;
  error?: string | undefined;
  optional?: boolean;
}

export function TextField({ label, value, onChange, hint, error, optional, className, ...rest }: TextFieldProps) {
  return (
    <FieldFrame label={label} {...(hint ? { hint } : {})} error={error} {...(optional ? { optional } : {})}>
      {({ id, describedBy, invalid }) => (
        <input
          id={id}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          aria-invalid={invalid}
          aria-describedby={describedBy}
          autoComplete="off"
          spellCheck={false}
          className={cx(control, className)}
          {...rest}
        />
      )}
    </FieldFrame>
  );
}

export interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, "onChange" | "value" | "id"> {
  label: string;
  value: string;
  options: readonly { value: string; label: string }[];
  onChange: (value: string) => void;
  hint?: string;
  error?: string | undefined;
}

export function SelectField({ label, value, options, onChange, hint, error, className, ...rest }: SelectFieldProps) {
  return (
    <FieldFrame label={label} {...(hint ? { hint } : {})} error={error}>
      {({ id, describedBy, invalid }) => (
        <select
          id={id}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          aria-invalid={invalid}
          aria-describedby={describedBy}
          className={cx(control, className)}
          {...rest}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      )}
    </FieldFrame>
  );
}

export interface CardChoiceProps<T extends string> {
  label: string;
  value: T;
  options: readonly { value: T; label: string; blurb?: string }[];
  onChange: (value: T) => void;
  /** The narrowest a card may be, as a CSS length: the grid fits as many as the width holds. */
  min?: string;
}

/**
 * A choice among a few, each a card with a line saying what it means
 * (components.md, "an option inside a form"). The chosen one has a radio's
 * filled dot as well as the green border and tint: a choice is never shown by
 * colour alone (SET-9).
 */
export function CardChoice<T extends string>({ label, value, options, onChange, min = "11.5rem" }: CardChoiceProps<T>) {
  return (
    <div className="flex flex-col gap-1.5">
      <p className="text-base font-semibold">{label}</p>
      <div
        role="radiogroup"
        {...radioGroup}
        aria-label={label}
        className="grid gap-3"
        style={{ gridTemplateColumns: `repeat(auto-fill, minmax(${min}, 1fr))` }}
      >
        {options.map((o) => {
          const chosen = o.value === value;
          return (
            <button
              key={o.value}
              type="button"
              role="radio"
              aria-checked={chosen}
              onClick={() => onChange(o.value)}
              className={cx(
                "flex min-h-tap items-start gap-3 rounded border px-4 py-3 text-left outline-offset-2 outline-focus focus-visible:outline-3",
                chosen ? "border-accent bg-accent-tint" : "border-line-strong bg-raised hover:bg-raised-high",
              )}
            >
              <span
                aria-hidden="true"
                className={cx(
                  "mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-round border-2",
                  chosen ? "border-accent" : "border-line-strong",
                )}
              >
                {chosen && <span className="size-2.5 rounded-round bg-accent" />}
              </span>
              <span className="flex min-w-0 flex-col gap-0.5">
                <span className={cx("text-base font-semibold", chosen && "text-accent-text")}>{o.label}</span>
                {o.blurb && <span className="text-sm text-fg-2">{o.blurb}</span>}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

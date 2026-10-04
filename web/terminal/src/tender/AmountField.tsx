// A field for an amount of money (D07): drawn as the design's input, and keyed
// like a till. Digits shift in from the right in minor units (numpad-keys.ts),
// from a keyboard while it has focus or from the keypad while it is the active
// field, so an amount is never a string to parse.

import { useId } from "react";
import type { Money as Amount } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { keyFromKeyboard, type NumKey } from "../../../shared/ui/numpad-keys";

interface AmountFieldProps {
  label: string;
  amount: Amount;
  /** The field the keypad types into. */
  active: boolean;
  /** Shown instead of a zero amount: "Enter amount". */
  placeholder?: string | undefined;
  /** A share by card is its whole amount: nothing to key. */
  disabled?: boolean;
  onActivate: () => void;
  onKey: (key: NumKey) => void;
}

export function AmountField({ label, amount, active, placeholder, disabled = false, onActivate, onKey }: AmountFieldProps) {
  const empty = amount.minor === 0 && placeholder !== undefined;
  const labelId = useId();
  const valueId = useId();
  return (
    <div className="flex flex-col gap-2">
      <span id={labelId} className="text-base font-semibold text-fg">
        {label}
      </span>
      <button
        type="button"
        // Named by its label and then what is in it, so the amount is read too.
        id={valueId}
        aria-labelledby={`${labelId} ${valueId}`}
        disabled={disabled}
        onClick={onActivate}
        onFocus={onActivate}
        onKeyDown={(e) => {
          // Ctrl+0 and the like are the browser's, not digits for the till.
          if (e.ctrlKey || e.metaKey || e.altKey) return;
          const key = keyFromKeyboard(e.key);
          if (!key) return;
          e.preventDefault();
          onKey(key);
        }}
        className={cx(
          "flex min-h-charge w-full items-center rounded border bg-raised px-4 text-left outline-offset-0 disabled:opacity-60",
          // A field's focus is green, with a soft halo (tokens.md).
          active && !disabled ? "border-accent outline outline-3 outline-accent-tint" : "border-line-strong",
        )}
      >
        {empty ? <span className="text-base text-fg-muted">{placeholder}</span> : <Money amount={amount} className="text-xl font-bold" />}
      </button>
    </div>
  );
}

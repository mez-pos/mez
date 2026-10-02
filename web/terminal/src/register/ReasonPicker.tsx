// A choice of reason: one per void, comp or discount, from a fixed list, so
// reports can count them (cart-logic.ts).

import { cx } from "../../../shared/ui/cx";
import type { Reason } from "./cart-logic";

interface ReasonPickerProps {
  reasons: readonly Reason[];
  chosen: string | null;
  onChoose: (code: string) => void;
  label: string;
}

export function ReasonPicker({ reasons, chosen, onChoose, label }: ReasonPickerProps) {
  return (
    <fieldset>
      <legend className="text-lg font-semibold">{label}</legend>
      <div role="radiogroup" aria-label={label} className="mt-3 grid grid-cols-2 gap-tap-gap">
        {reasons.map((r) => (
          <button
            key={r.code}
            type="button"
            role="radio"
            aria-checked={chosen === r.code}
            onClick={() => onChoose(r.code)}
            className={cx(
              "min-h-tap rounded border px-4 py-2.5 text-left text-base",
              "outline-offset-2 outline-line-focus focus-visible:outline-2",
              chosen === r.code ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line bg-raised text-fg-2",
            )}
          >
            {r.label}
          </button>
        ))}
      </div>
    </fieldset>
  );
}

// A quantity: tabular, in one pill, with − and + as full tap targets. Without
// onChange it is a plain figure, for a line that can no longer change.
//
// It reports the next value and holds none itself: an order line's quantity
// changes by appending an event on the server (docs/02-domain.md), so the
// screen decides what a tap does.

import { canStep, defaultQtyBounds, stepQty, type QtyBounds } from "./qty-step";
import { cx } from "./cx";
import { focusButton } from "./focus";

export interface QtyProps {
  value: number;
  onChange?: (next: number) => void;
  bounds?: QtyBounds;
  /** What is being counted, for screen readers: "Flat white". */
  label?: string;
  className?: string;
}

export function Qty({ value, onChange, bounds = defaultQtyBounds, label, className }: QtyProps) {
  const figure = <span className="min-w-8 px-1 text-center font-mono text-lg font-semibold tabular-nums">{value}</span>;
  if (!onChange) {
    return <span className={cx("inline-flex items-center", className)}>{figure}</span>;
  }

  const step = (by: 1 | -1) => () => onChange(stepQty(value, by, bounds));
  const button = `inline-flex size-tap items-center justify-center rounded-pill text-xl hover:bg-raised-high ${focusButton} disabled:opacity-40`;

  return (
    <span
      role="group"
      aria-label={label ? `Quantity of ${label}` : "Quantity"}
      className={cx("inline-flex items-center rounded-pill border border-line-strong bg-raised", className)}
    >
      <button type="button" aria-label="Decrease" className={cx(button, "text-fg")} onClick={step(-1)} disabled={!canStep(value, -1, bounds)}>
        {"−"}
      </button>
      <output aria-live="polite">{figure}</output>
      <button type="button" aria-label="Increase" className={cx(button, "text-accent-text")} onClick={step(1)} disabled={!canStep(value, 1, bounds)}>
        +
      </button>
    </span>
  );
}

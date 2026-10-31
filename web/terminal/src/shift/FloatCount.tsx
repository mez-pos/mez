// The opening float, counted (D02): a card for each note and coin with a box
// for how many and the value of that row beside it; the counted total is
// FloatTotal, a green panel the screen keeps in its foot. It shows what has
// been counted and nothing else: no expected figure reaches it, as at the
// close (ADR-094, ADR-150).
//
// A currency with no table of notes is one box for the whole amount.

import { useState } from "react";
import { amountInputText } from "../../../shared/money";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { faceText, setFaceCount, typedFloat, type FloatCounted } from "./open-shift-logic";
import { facesFor } from "./shift-logic";

interface FloatCountProps {
  currency: string;
  value: FloatCounted;
  onChange: (next: FloatCounted) => void;
  disabled?: boolean;
}

const boxClass =
  "h-tap min-w-0 flex-1 rounded border border-line-strong bg-raised px-3 text-right font-mono text-lg font-semibold tabular-nums " +
  "text-fg outline-offset-2 placeholder:text-fg-faint focus:border-accent focus:outline-3 focus:outline-accent-tint disabled:opacity-40";

export function FloatCount({ currency, value, onChange, disabled = false }: FloatCountProps) {
  const faces = facesFor(currency);

  return (
    <section aria-label="Opening float" className="grid grid-cols-2 gap-3.5 sm:grid-cols-3 xl:grid-cols-4">
      <h2 className="col-span-full text-meta font-bold uppercase text-fg-muted">Denomination count</h2>
      {faces.length > 0 ? (
        faces.map((face) => {
          const count = value.tally[face] ?? 0;
          const name = faceText(face, currency);
          return (
            <label key={face} className="flex flex-col gap-2 rounded border border-line p-3.5">
              {/* The note's name, as staff say it: the row's value beside the box is the figure. */}
              <span className="self-start whitespace-nowrap font-mono text-sm tabular-nums text-fg-muted">{name}</span>
              <span className="flex items-center gap-2">
                <input
                  inputMode="numeric"
                  autoComplete="off"
                  placeholder="0"
                  aria-label={`How many of ${name}`}
                  value={count === 0 ? "" : String(count)}
                  disabled={disabled}
                  onFocus={(e) => e.currentTarget.select()}
                  onChange={(e) => onChange({ ...value, tally: setFaceCount(value.tally, face, e.target.value) })}
                  className={boxClass}
                />
                <Money amount={{ minor: face * count, currency }} tone={count === 0 ? "muted" : "default"} className="min-w-16 text-base" />
              </span>
            </label>
          );
        })
      ) : (
        <TotalBox currency={currency} typed={value.typed} disabled={disabled} onTyped={(typed) => onChange({ ...value, typed })} />
      )}
    </section>
  );
}

/** The counted opening float: the one total on the screen, in a green panel. */
export function FloatTotal({ currency, total, className }: { currency: string; total: number; className?: string }) {
  return (
    <div className={cx("rounded-card bg-accent-tint px-6 py-5 short:py-3", className)}>
      <p className="text-base text-fg-2">Counted opening float</p>
      <Money amount={{ minor: total, currency }} variant="total" tone="accent" />
    </div>
  );
}

/** The whole amount, for a currency with no notes and coins to count. */
function TotalBox({
  currency,
  typed,
  disabled,
  onTyped,
}: {
  currency: string;
  typed: number | null;
  disabled: boolean;
  onTyped: (minor: number | null) => void;
}) {
  const [text, setText] = useState(typed === null || typed === 0 ? "" : amountInputText({ minor: typed, currency }));
  const bad = typedFloat(text, currency) === null;
  return (
    <label className="col-span-full flex flex-col gap-2 rounded border border-line p-3.5 sm:col-span-2">
      <span className="text-sm text-fg-muted">Total counted ({currency})</span>
      <input
        inputMode="decimal"
        autoComplete="off"
        placeholder="0"
        value={text}
        disabled={disabled}
        aria-invalid={bad}
        onFocus={(e) => e.currentTarget.select()}
        onChange={(e) => {
          setText(e.target.value);
          onTyped(typedFloat(e.target.value, currency));
        }}
        className={boxClass}
      />
      {bad && (
        <span role="alert" className="text-sm text-danger">
          Type the total as a plain amount, like 1500 or 1500.50.
        </span>
      )}
    </label>
  );
}

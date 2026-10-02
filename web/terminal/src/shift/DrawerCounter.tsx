// The counter (terminal-close.md): a row per note and coin, largest first.
// Tap a row, type its count on the pad, Next for the row below. It shows
// what has been counted and nothing else: no expected figure reaches it.
// A currency with no table is counted as one total on the pad.

import { useState } from "react";
import { Button, Money, NumPad } from "../../../shared/ui";
import { applyAmountKey, type NumKey } from "../../../shared/ui/numpad-keys";
import { cx } from "../../../shared/ui/cx";
import { facesFor, keyCount, nextFace, tallyTotal, type Tally } from "./shift-logic";

/** The most a typed total can be: far past any drawer. */
const maxTyped = 99_999_999_99;

export interface Counted {
  tally: Tally;
  /** The total typed on the pad, for a currency with no table. */
  typed: number;
}

interface DrawerCounterProps {
  title: string;
  currency: string;
  value: Counted;
  onChange: (next: Counted) => void;
  disabled?: boolean;
}

export function DrawerCounter({ title, currency, value, onChange, disabled = false }: DrawerCounterProps) {
  const rows = facesFor(currency);
  const [picked, setPicked] = useState<number | null>(rows[0] ?? null);

  const key = (k: NumKey) => {
    if (rows.length === 0) onChange({ ...value, typed: applyAmountKey(value.typed, k, maxTyped) });
    else if (picked !== null) onChange({ ...value, tally: keyCount(value.tally, picked, k) });
  };
  const total = rows.length > 0 ? tallyTotal(value.tally) : value.typed;

  return (
    <section aria-label={title} className="flex w-full max-w-3xl flex-col gap-4">
      <h2 className="font-serif text-xl">{title}</h2>
      <div className="flex flex-wrap items-start gap-6">
        {rows.length > 0 ? (
          <ol aria-label="Notes and coins" className="flex min-w-72 flex-1 flex-col gap-1">
            {rows.map((face) => {
              const count = value.tally[face] ?? 0;
              const here = face === picked;
              return (
                <li key={face}>
                  <button
                    type="button"
                    disabled={disabled}
                    aria-pressed={here}
                    onClick={() => setPicked(face)}
                    className={cx(
                      "grid min-h-tap w-full grid-cols-[1fr_5rem_8rem] items-center gap-3 rounded border px-4 text-left",
                      "outline-offset-2 outline-line-focus focus-visible:outline-2 disabled:opacity-40",
                      here ? "border-accent bg-accent-tint text-accent-text" : "border-line bg-raised hover:bg-raised-high",
                    )}
                  >
                    <Money amount={{ minor: face, currency }} className="text-left" />
                    <span className="text-right font-mono text-base tabular-nums">
                      <span className="sr-only">Count </span>
                      {count}
                    </span>
                    <Money amount={{ minor: face * count, currency }} tone={count === 0 ? "muted" : "default"} className="text-right" />
                  </button>
                </li>
              );
            })}
          </ol>
        ) : (
          <p className="min-w-72 flex-1 text-base text-fg-muted">Type the total counted on the pad.</p>
        )}
        <div className="flex flex-col gap-tap-gap">
          <NumPad layout="amount" size="lg" label={picked !== null && rows.length > 0 ? "Count of this note" : "Total counted"} onKey={key} disabled={disabled} />
          {rows.length > 0 && (
            <Button disabled={disabled || picked === null} onClick={() => picked !== null && setPicked(nextFace(rows, picked))}>
              Next
            </Button>
          )}
        </div>
      </div>
      <p className="flex items-baseline justify-between border-t border-line pt-3">
        <span className="font-serif text-xl">Counted</span>
        <Money amount={{ minor: total, currency }} variant="total" />
      </p>
    </section>
  );
}

/** A counter with nothing counted yet. */
export const emptyCount: Counted = { tally: {}, typed: 0 };

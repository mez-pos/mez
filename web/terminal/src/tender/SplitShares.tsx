// The shares of a split in use, over the tender controls: which are paid,
// which one the next tender is for. Tapping a share moves to it.

import type { SplitResponse } from "../../../shared/api-types";
import { Button, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { payerLabel } from "./split-logic";

interface SplitSharesProps {
  split: SplitResponse;
  amountsKeyed: number;
  paid: ReadonlySet<number>;
  current: number | null;
  /** The bill moved since the split: it must be made again. */
  stale: boolean;
  disabled: boolean;
  onPick: (payer: number) => void;
  onClear: () => void;
}

export function SplitShares({ split, amountsKeyed, paid, current, stale, disabled, onPick, onClear }: SplitSharesProps) {
  return (
    <section aria-label="Split" className="mb-5 rounded-card border border-line bg-chrome shadow-rest p-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-meta font-semibold uppercase text-fg-muted">
          Split {split.kind === "evenly" ? "evenly" : `by ${split.kind}`} · {split.shares.length} shares
        </p>
        <Button variant="ghost" disabled={disabled} onClick={onClear}>
          Clear split
        </Button>
      </div>
      {stale && (
        <p role="alert" className="mt-2 text-base text-danger">
          The bill changed after it was split. Clear the split and split it again.
        </p>
      )}
      <div role="radiogroup" aria-label="Share to pay" className="mt-2 flex flex-wrap gap-tap-gap">
        {split.shares.map((s) => {
          const done = paid.has(s.payer);
          return (
            <button
              key={s.payer}
              type="button"
              role="radio"
              aria-checked={current === s.payer}
              disabled={disabled || done || stale}
              onClick={() => onPick(s.payer)}
              className={cx(
                "flex min-h-tap flex-col items-start gap-0.5 rounded border px-3 py-2 text-left",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                current === s.payer ? "border-accent bg-accent-tint text-accent-text" : "border-line bg-raised",
                done && "opacity-60",
              )}
            >
              <span className="flex items-center gap-2 text-base font-semibold">
                {payerLabel(split.kind, s.payer, amountsKeyed)}
                {done && <StatusChip status={{ kind: "payment", status: "captured" }} />}
              </span>
              <Money amount={s.amount} className={cx(done && "line-through")} />
            </button>
          );
        })}
      </div>
    </section>
  );
}

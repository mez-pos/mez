// The top of the waiter's bill (T05): how it is split, between how many, and a
// row for each share saying whether it is paid. Collect on a row makes it the
// share the payment below is for. Split equally is made here; By item and
// Custom open the split panel (tender/SplitPanel.tsx), and what it completes
// comes back as these rows.

import { CircleCheckIcon } from "lucide-react";
import type { Money as Amount, SplitKind } from "../../../shared/api-types";
import { Button, Money, Qty } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { maxWays, type TillSplit } from "../tender/split-logic";
import { billModes, shareStatus, type ShareRow } from "./bill-logic";

interface BillSharesProps {
  /** The split on screen: equal while it is made here, or the one completed. */
  kind: SplitKind;
  /** The shares are settled: a payment was begun, or the split panel completed one. */
  fixed: boolean;
  /** Something is paid already: the server divides items only on an unpaid bill. */
  partPaid: boolean;
  ways: number;
  rows: readonly ShareRow[];
  /** What each guest pays, when it is one figure. */
  each: Amount | null;
  /** The rows are the last answer, and the next is on its way. */
  settling: boolean;
  disabled: boolean;
  onWays: (ways: number) => void;
  onMode: (kind: TillSplit) => void;
  /** Lets go of a settled split, to make another. */
  onChange: () => void;
  onCollect: (payer: number) => void;
}

export function BillShares({ kind, fixed, partPaid, ways, rows, each, settling, disabled, onWays, onMode, onChange, onCollect }: BillSharesProps) {
  return (
    <section aria-label="Split">
      <div className="flex flex-wrap items-center gap-3">
        <div role="radiogroup" aria-label="How the bill is split" className="inline-flex gap-1 rounded-pill bg-sunken p-1">
          {billModes.map((m) => {
            const on = m.kind === kind;
            return (
              <button
                key={m.kind}
                type="button"
                role="radio"
                aria-checked={on}
                disabled={disabled || fixed || (m.kind === "item" && partPaid)}
                onClick={() => onMode(m.kind)}
                className={cx(
                  "min-h-tap rounded-pill px-5 text-base outline-offset-2 outline-attention focus-visible:outline-3",
                  on ? "bg-accent font-semibold text-accent-fg" : "text-fg hover:bg-raised-high disabled:opacity-40",
                )}
              >
                {m.label}
              </button>
            );
          })}
        </div>
        {fixed && (
          <Button variant="ghost" disabled={disabled} onClick={onChange} className="ml-auto">
            Change the split
          </Button>
        )}
      </div>

      {kind === "evenly" && (
        <div className="mt-4 flex flex-wrap items-center gap-3.5">
          <span className="text-base text-fg-2">Split between</span>
          {fixed ? <Qty value={ways} /> : <Qty value={ways} onChange={onWays} bounds={{ min: 1, max: maxWays }} label="guests" />}
          <span className="text-base text-fg-2">{ways === 1 ? "guest" : "guests"}</span>
          {each && (
            <p className={cx("ml-auto text-base text-fg-muted", settling && "opacity-50")}>
              <Money amount={each} /> each
            </p>
          )}
        </div>
      )}
      {!fixed && partPaid && <p className="mt-2.5 text-sm text-fg-muted">By item is for a bill with nothing paid yet.</p>}

      <ul className={cx("mt-4 grid gap-2.5", settling && "opacity-60")}>
        {rows.map((row) => {
          const paid = row.state === "paid";
          return (
            <li
              key={row.payer}
              className={cx(
                "flex min-h-18 items-center gap-3 rounded-card border px-4 py-3",
                paid ? "border-chip-success-border bg-accent-tint" : row.state === "collecting" ? "border-accent bg-raised" : "border-line bg-raised",
              )}
            >
              <span
                aria-hidden="true"
                className={cx(
                  "flex size-11 shrink-0 items-center justify-center rounded-round text-base font-bold",
                  paid ? "bg-chip-success-bg text-chip-success-fg" : "bg-sunken text-fg-muted",
                )}
              >
                {row.payer + 1}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-base font-bold">{row.name}</p>
                <p className={cx("text-sm", paid ? "font-semibold text-chip-success-fg" : "text-fg-muted")}>{shareStatus(row)}</p>
              </div>
              <Money amount={row.amount} variant="heading" />
              {paid && <CircleCheckIcon aria-hidden="true" size={24} className="shrink-0 text-chip-success-fg" />}
              {row.state === "waiting" && (
                <Button disabled={disabled || settling} onClick={() => onCollect(row.payer)} aria-label={`Collect from ${row.name}`}>
                  Collect
                </Button>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

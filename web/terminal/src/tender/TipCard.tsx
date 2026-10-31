// The tip card (D07): three tips in one tap, a custom amount, and what the
// payment comes to with it. A tip is kept apart from the bill, always: it is
// never part of what the invoice says was sold. The design's line about where
// tips go is left out: who shares a tip is the venue's rule, not this screen's
// to promise. A tip over the venue's ceiling says it needs a manager, who
// approves it when the payment is taken (ADR-261).

import type { Money as Amount } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import type { NumKey } from "../../../shared/ui/numpad-keys";
import { AmountField } from "./AmountField";
import { tipFor, tipPercents } from "./tender-logic";

interface TipCardProps {
  /** What this payment settles of the bill. */
  bill: Amount;
  /**
   * What the percentages are of, in minor units: the balance or the share
   * being paid, which does not move as the cash and the tip are keyed.
   */
  base: number;
  tip: Amount;
  /** Bill and tip together: what the guest hands over. */
  collect: Amount;
  /** Tips come out of the cash handed over. */
  cash: boolean;
  /** "Over 50%: needs a manager" for a tip over the venue's ceiling, or null (tipNote). */
  note: string | null;
  active: boolean;
  busy: boolean;
  onActivate: () => void;
  onKey: (key: NumKey) => void;
  onTip: (minor: number) => void;
}

export function TipCard({ bill, base, tip, collect, cash, note, active, busy, onActivate, onKey, onTip }: TipCardProps) {
  const money = (minor: number): Amount => ({ minor, currency: bill.currency });
  const offers = tipPercents.map((percent) => ({ percent, minor: tipFor(base, percent, bill.currency) })).filter((o) => o.minor > 0);
  const chosen = offers.find((o) => o.minor === tip.minor);

  return (
    // Under the amount and the pad below 1536 it is as wide as both, so the
    // sums take a column of their own beside the choices; from 1536 it is a
    // column itself (D07).
    <section aria-label="Tip" className="grid items-end gap-x-8 rounded-card border border-line bg-raised p-5 short:p-4 lg:grid-cols-2 2xl:grid-cols-1">
      <div>
        <h2 className="font-serif text-xl">Add a tip?</h2>
        <p className="mt-1 text-sm text-fg-muted">
          {cash ? "It comes out of the change. It is never part of the bill." : "It is charged with the bill and kept apart from it."}
        </p>

        {offers.length > 0 && (
          <div role="group" aria-label="Tip" className="mt-4 grid grid-cols-3 gap-2.5 short:mt-3">
            {offers.map((o) => {
              const on = chosen?.percent === o.percent;
              return (
                <button
                  key={o.percent}
                  type="button"
                  aria-pressed={on}
                  disabled={busy}
                  // A second tap takes the tip off again.
                  onClick={() => onTip(on ? 0 : o.minor)}
                  className={cx(
                    "flex min-h-16 flex-col items-center justify-center gap-0.5 rounded border outline-offset-2 outline-focus focus-visible:outline-3 disabled:opacity-40",
                    on ? "border-highlight bg-chip-warning-bg text-chip-warning-fg" : "border-line-strong bg-raised hover:bg-raised-high",
                  )}
                >
                  <span className="text-lg font-bold">{o.percent}%</span>
                  <Money amount={money(o.minor)} className={cx("text-sm", !on && "text-fg-muted")} />
                </button>
              );
            })}
          </div>
        )}

        <div className="mt-4 short:mt-3">
          <AmountField label="Custom amount" amount={tip} active={active} placeholder="Enter amount" disabled={busy} onActivate={onActivate} onKey={onKey} />
        </div>
      </div>

      <div className="mt-5 grid gap-1.5 border-t border-line pt-4 text-base short:mt-4 short:pt-3">
        <div className="flex justify-between text-fg-2">
          <span>Bill</span>
          <Money amount={bill} />
        </div>
        <div className="flex justify-between text-fg-2">
          <span>{chosen ? `Tip (${chosen.percent}%)` : "Tip"}</span>
          <Money amount={tip} />
        </div>
        {note && <TipNote note={note} />}
        <div className="mt-1 flex items-baseline justify-between">
          <span className="font-serif text-xl">To collect</span>
          <Money amount={collect} variant="heading" />
        </div>
      </div>
    </section>
  );
}

/** A tip over the ceiling, as a status: the colour and the words. */
export function TipNote({ note }: { note: string }) {
  return (
    <p role="status" className="flex items-center gap-2 rounded bg-chip-warning-bg px-3 py-1.5 text-sm font-semibold text-chip-warning-fg">
      <span aria-hidden="true" className="size-2 shrink-0 rounded-round bg-highlight" />
      {note}
    </p>
  );
}

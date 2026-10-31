// The amount card (D07): what is to be collected, what was handed over, the
// round sums a guest is likely to hand over, and the change to give back. Card
// and UPI have no change: there the amount follows the balance, and the payment
// is taken on the venue's own machine or code after Collect (PayStates.tsx).
//
// The change is said in the payment screen's foot at every size (ADR-381); the
// card repeats it, as drawn, only from 1536, where there is room for it.

import { BanknoteIcon } from "lucide-react";
import type { Money as Amount, PaymentMethod } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import type { NumKey } from "../../../shared/ui/numpad-keys";
import { AmountField } from "./AmountField";
import { quickShown } from "./tender-logic";

interface AmountCardProps {
  method: PaymentMethod;
  /** The balance, or the share of it being paid. */
  target: Amount;
  /** A split share is being paid: card and UPI take exactly it. */
  share: boolean;
  amount: Amount;
  active: boolean;
  /** Exact first, then the round sums above it, in minor units. Cash only. */
  quick: readonly number[];
  /** Cash to hand back, or null when there is none to show. */
  change: Amount | null;
  busy: boolean;
  onActivate: () => void;
  onKey: (key: NumKey) => void;
  onQuick: (minor: number) => void;
  /** Clears the amount for one that is none of the quick ones. */
  onCustom: () => void;
}

const chip =
  "flex min-h-tap items-center justify-center rounded border px-2 text-base outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40";

export function AmountCard(props: AmountCardProps) {
  const { method, target, share, amount, active, quick, change, busy, onActivate, onKey, onQuick, onCustom } = props;
  const cash = method === "cash";
  const money = (minor: number): Amount => ({ minor, currency: target.currency });

  return (
    <section aria-label="Amount" className="rounded-card border border-line bg-raised p-5 short:p-4">
      <p className="text-base text-fg-2">{share ? "This share" : "Amount to collect"}</p>
      <Money amount={target} variant="display" className="block text-left leading-tight" />

      <div className="mt-4 short:mt-3">
        <AmountField
          label={cash ? "Amount received" : "Amount"}
          amount={amount}
          active={active}
          placeholder={cash ? "Enter amount" : undefined}
          disabled={busy || (share && !cash)}
          onActivate={onActivate}
          onKey={onKey}
        />
      </div>

      {cash ? (
        <div role="group" aria-label="Quick amounts" className="mt-3 grid grid-cols-[repeat(auto-fill,minmax(6.5rem,1fr))] gap-2">
          {quickShown(quick, 4).map((q) => {
            const on = amount.minor === q;
            return (
              <button
                key={q}
                type="button"
                aria-pressed={on}
                disabled={busy}
                onClick={() => onQuick(q)}
                className={cx(
                  chip,
                  on ? "border-accent bg-accent-tint font-bold text-accent-text" : "border-line-strong bg-raised font-semibold hover:bg-raised-high",
                )}
              >
                <Money amount={money(q)} />
              </button>
            );
          })}
          <button type="button" disabled={busy} onClick={onCustom} className={cx(chip, "border-line-strong bg-raised font-semibold hover:bg-raised-high")}>
            Custom
          </button>
        </div>
      ) : (
        <p className="mt-4 text-sm text-fg-muted">
          {method === "card"
            ? "Collect, take it on the card machine, then mark it paid."
            : method === "upi"
              ? "Collect, let the guest pay by UPI, then mark it paid."
              : "Collect, take the payment, then mark it paid."}
        </p>
      )}

      {cash && (
        <div className="mt-4 hidden items-center justify-between gap-4 rounded-card border border-chip-success-border bg-chip-success-bg/50 px-5 py-4 2xl:flex">
          <div>
            <p className="text-base text-fg-2">Change to return</p>
            <Money amount={change ?? money(0)} variant="total" tone="accent" className="block text-left leading-tight" />
          </div>
          <span aria-hidden="true" className="flex size-14 shrink-0 items-center justify-center rounded bg-chip-success-bg text-accent-text">
            <BanknoteIcon size={28} strokeWidth={1.5} />
          </span>
        </div>
      )}
    </section>
  );
}

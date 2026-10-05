// Collecting one share on the waiter's bill (T05): how it is paid, a tip, and
// the payment itself. A share is paid whole, in one payment.
//
// The design draws a QR made for the share and a wait for the bank to confirm.
// The tablet has no gateway and must not need one (PayStates.tsx says why): the
// UPI code and the card machine are the venue's own, so the waiter says when
// the money has come, and "Mark as paid" records it. Cash is keyed, or one of
// the round sums tapped, and the change is said before it is taken.

import { BanknoteIcon, CreditCardIcon, QrCodeIcon, type LucideIcon } from "lucide-react";
import { useState } from "react";
import type { Money as Amount, PaymentMethod } from "../../../shared/api-types";
import { Button, Money, NumPad, type NumKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { AmountField } from "../tender/AmountField";
import { methodNames, tipFor, tipPercents, type TenderPlan } from "../tender/tender-logic";
import { billMethods } from "./bill-logic";

const icons: Readonly<Record<PaymentMethod, LucideIcon>> = { upi: QrCodeIcon, card: CreditCardIcon, cash: BanknoteIcon, other: BanknoteIcon };

const chip =
  "flex min-h-tap items-center justify-center rounded border px-3 text-base outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40";
const chipOn = "border-accent bg-accent-tint font-bold text-accent-text";
const chipOff = "border-line-strong bg-raised font-semibold hover:bg-raised-high";

interface BillCollectProps {
  /** Whose share, or null for a bill nobody shares. */
  from: string | null;
  /** The share: what this payment settles of the bill. */
  target: Amount;
  method: PaymentMethod;
  /** Cash handed over, in minor units. */
  entered: number;
  tip: number;
  reference: string;
  plan: TenderPlan;
  /** Exact first, then the round sums above it. Cash only. */
  quick: readonly number[];
  busy: boolean;
  /** Why the last try was not recorded. */
  error: string | null;
  onMethod: (method: PaymentMethod) => void;
  onEntered: (minor: number) => void;
  onKey: (key: NumKey) => void;
  onTip: (minor: number) => void;
  onReference: (text: string) => void;
  onSend: () => void;
}

export function BillCollect(props: BillCollectProps) {
  const { from, target, method, entered, tip, reference, plan, quick, busy, error, onMethod, onEntered, onKey, onTip, onReference, onSend } = props;
  const [pad, setPad] = useState(false);
  const money = (minor: number): Amount => ({ minor, currency: target.currency });
  const cash = method === "cash";
  const Icon = icons[method];
  const offers = tipPercents.map((percent) => ({ percent, minor: tipFor(target.minor, percent, target.currency) })).filter((o) => o.minor > 0);
  const who = from ?? "the guest";

  return (
    <section aria-label="Collect" className="mt-5">
      <h2 className="font-serif text-xl">
        Collect <Money amount={target} variant="heading" />
        {from && ` from ${from}`}
      </h2>

      <div role="radiogroup" aria-label="Payment method" className="mt-3 grid grid-cols-3 gap-2.5">
        {billMethods.map((m) => {
          const on = m === method;
          const Tile = icons[m];
          return (
            <button
              key={m}
              type="button"
              role="radio"
              aria-checked={on}
              disabled={busy}
              onClick={() => {
                setPad(false);
                onMethod(m);
              }}
              className={cx(
                "flex h-20 flex-col items-center justify-center gap-1.5 rounded-card border text-base",
                "outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40",
                on ? "border-accent bg-accent-tint font-bold text-accent-text" : "border-line-strong bg-raised font-semibold hover:bg-raised-high",
              )}
            >
              <Tile aria-hidden="true" size={24} strokeWidth={on ? 1.9 : 1.75} />
              {methodNames[m]}
            </button>
          );
        })}
      </div>

      {offers.length > 0 && (
        <div role="group" aria-label="Tip" className="mt-3 flex flex-wrap items-center gap-2">
          <span className="mr-1 text-base text-fg-2">Tip</span>
          <button type="button" aria-pressed={tip === 0} disabled={busy} onClick={() => onTip(0)} className={cx(chip, tip === 0 ? chipOn : chipOff)}>
            None
          </button>
          {offers.map((o) => {
            const on = tip === o.minor;
            return (
              <button
                key={o.percent}
                type="button"
                aria-pressed={on}
                disabled={busy}
                onClick={() => onTip(o.minor)}
                className={cx(chip, "gap-2", on ? chipOn : chipOff)}
              >
                {o.percent}%
                <Money amount={money(o.minor)} className="text-sm font-normal" />
              </button>
            );
          })}
        </div>
      )}

      <div className="mt-4 rounded-card border border-line bg-raised p-4">
        {cash ? (
          <>
            <AmountField
              label="Cash received"
              amount={money(entered)}
              active={pad}
              placeholder="Tap an amount, or here to key one"
              disabled={busy}
              onActivate={() => setPad(true)}
              onKey={onKey}
            />
            <div role="group" aria-label="Quick amounts" className="mt-2.5 grid grid-cols-4 gap-2">
              {quick.slice(0, 4).map((q) => (
                <button
                  key={q}
                  type="button"
                  aria-pressed={entered === q}
                  disabled={busy}
                  onClick={() => onEntered(q)}
                  className={cx(chip, entered === q ? chipOn : chipOff)}
                >
                  <Money amount={money(q)} />
                </button>
              ))}
            </div>
            {pad && <NumPad layout="amount" label="Cash received" onKey={onKey} disabled={busy} className="mt-3" />}
          </>
        ) : (
          <div className="flex items-center gap-4">
            <span aria-hidden="true" className="flex size-24 shrink-0 items-center justify-center rounded bg-sunken text-fg-2">
              <Icon size={44} strokeWidth={1.5} />
            </span>
            <div className="min-w-0 flex-1">
              <p className="text-base font-bold">{method === "upi" ? `Show the venue's UPI code to ${who}` : "Key this on the card machine"}</p>
              <p className="mt-1 text-sm text-fg-muted">
                <Money amount={money(plan.charge)} className="font-bold text-fg" />
                {tip > 0 && (
                  <>
                    , with a tip of <Money amount={money(tip)} /> in it
                  </>
                )}
              </p>
              <p className="mt-2 flex items-center gap-2 text-sm font-semibold text-chip-warning-fg">
                <span aria-hidden="true" className="size-2 rounded-round bg-highlight" />
                Waiting for you to see it paid
              </p>
            </div>
          </div>
        )}

        {error ? (
          <p role="alert" className="mt-3 text-base text-danger">
            {error} Trying again sends the same payment, so it cannot be recorded twice.
          </p>
        ) : (
          plan.problem && cash && entered > 0 && <p className="mt-3 text-sm text-fg-muted">{plan.problem}</p>
        )}

        <div className="mt-3 flex gap-2.5">
          {cash && (
            <p
              aria-live="polite"
              className="flex min-h-charge min-w-0 flex-1 items-center justify-between gap-3 rounded bg-accent-tint px-4 text-base text-fg-2"
            >
              Change to return
              <Money amount={money(plan.problem === null ? plan.change : 0)} variant="heading" tone="accent" />
            </p>
          )}
          {!cash && (
            <input
              value={reference}
              onChange={(e) => onReference(e.target.value)}
              disabled={busy}
              maxLength={40}
              autoComplete="off"
              spellCheck={false}
              aria-label={method === "upi" ? "UPI transaction ID, optional" : "Slip reference, optional"}
              placeholder={method === "upi" ? "UPI transaction ID (optional)" : "Slip reference (optional)"}
              className="min-h-charge min-w-0 flex-1 rounded border border-line-strong bg-raised px-4 text-base text-fg outline-accent-tint placeholder:text-fg-muted focus-visible:border-accent focus-visible:outline-3"
            />
          )}
          <Button variant="primary" size="charge" disabled={busy || plan.problem !== null} onClick={onSend} className="w-52 shrink-0">
            {busy ? "Recording" : error ? "Try again" : cash ? "Cash received" : "Mark as paid"}
          </Button>
        </div>
      </div>
    </section>
  );
}

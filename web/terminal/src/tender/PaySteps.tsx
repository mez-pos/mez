// The three steps along the top of the payment screen (D07): Payment, Split
// Bill, Complete. The first two are how to get between paying and splitting;
// Complete is where a paid bill arrives, never a button, so it is drawn as a
// plain label with no pill to tap. A paid bill shows no strip at all: the
// invoice is headed "Paid in full" (ADR-381). And under them, on the payment
// step, the method tiles.

import { BanknoteIcon, CreditCardIcon, QrCodeIcon, type LucideIcon } from "lucide-react";
import type { PaymentMethod } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";
import { methodNames, type PayStep } from "./tender-logic";

interface PayStepsProps {
  step: PayStep;
  disabled: boolean;
  onPayment: () => void;
  onSplit: () => void;
}

const steps: readonly { id: PayStep; label: string }[] = [
  { id: "payment", label: "Payment" },
  { id: "split", label: "Split Bill" },
  { id: "complete", label: "Complete" },
];

export function PaySteps({ step, disabled, onPayment, onSplit }: PayStepsProps) {
  const paid = step === "complete";
  return (
    <ol aria-label="Steps" className="grid grid-cols-3 gap-3">
      {steps.map((s, i) => {
        const on = s.id === step;
        const shape = cx(
          "flex min-h-14 w-full items-center justify-center gap-3 rounded-card px-3 text-lg font-semibold short:min-h-12 2xl:min-h-15",
          on ? "bg-accent text-accent-fg" : s.id === "complete" ? "text-fg-muted" : "bg-sunken text-fg-muted",
        );
        const body = (
          <>
            <span
              aria-hidden="true"
              className={cx(
                "flex size-7.5 shrink-0 items-center justify-center rounded-round text-base font-bold",
                on ? "bg-accent-fg/20" : "bg-line-strong/60",
              )}
            >
              {i + 1}
            </span>
            {s.label}
          </>
        );
        return (
          <li key={s.id} aria-current={on ? "step" : undefined}>
            {s.id === "complete" || paid ? (
              <span className={shape}>{body}</span>
            ) : (
              <button
                type="button"
                disabled={disabled}
                onClick={s.id === "payment" ? onPayment : onSplit}
                className={cx(shape, "outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-60", !on && "hover:bg-raised-high")}
              >
                {body}
              </button>
            )}
          </li>
        );
      })}
    </ol>
  );
}

// The design draws a Wallet tile too: no wallet is a payment method here.
const methods: readonly { method: PaymentMethod; icon: LucideIcon }[] = [
  { method: "cash", icon: BanknoteIcon },
  { method: "card", icon: CreditCardIcon },
  { method: "upi", icon: QrCodeIcon },
];

interface MethodTilesProps {
  method: PaymentMethod;
  disabled: boolean;
  onChoose: (method: PaymentMethod) => void;
}

export function MethodTiles({ method, disabled, onChoose }: MethodTilesProps) {
  return (
    <div role="radiogroup" aria-label="Payment method" className="grid grid-cols-3 gap-3.5">
      {methods.map(({ method: m, icon: Icon }) => {
        const on = m === method;
        return (
          <button
            key={m}
            type="button"
            role="radio"
            aria-checked={on}
            disabled={disabled}
            onClick={() => onChoose(m)}
            className={cx(
              // On a short screen the icon sits beside the word, and the row is a key's height.
              "flex h-20 flex-col items-center justify-center gap-2 rounded-card border text-lg short:h-14 short:flex-row short:gap-3 2xl:h-29 2xl:gap-3",
              "outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40",
              on ? "border-accent bg-accent-tint font-bold text-accent-text" : "border-line-strong bg-raised font-semibold text-fg hover:bg-raised-high",
            )}
          >
            <Icon aria-hidden="true" size={30} strokeWidth={1.7} />
            {methodNames[m]}
          </button>
        );
      })}
    </div>
  );
}

// A discount on a line or on the order: a percentage or an amount, typed on
// the pad, a reason, then a manager's approval. The pad enters from the
// right: 1250 is 12.50% or ₹12.50, as the amount pad does everywhere.

import { useState } from "react";
import type { DiscountRequest, Money as Amount } from "../../../shared/api-types";
import { applyAmountKey, Money, NumPad } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { ApprovalSheet } from "./ApprovalSheet";
import { percent } from "../../../shared/ui/timeline-entries";
import { discountProblem, discountReasons, discountRequest, type DiscountEntry } from "./cart-logic";
import { ReasonPicker } from "./ReasonPicker";

interface DiscountSheetProps {
  /** A line, by id and name, or the whole order. */
  target: { readonly lineId: number; readonly name: string } | "order";
  /** What the discount would come off. */
  worth: Amount;
  onSubmit: (request: DiscountRequest) => Promise<void>;
  onClose: () => void;
}

export function DiscountSheet({ target, worth, onSubmit, onClose }: DiscountSheetProps) {
  const [kind, setKind] = useState<DiscountEntry["kind"]>("pct");
  const [value, setValue] = useState(0);
  const [reason, setReason] = useState<string | null>(null);

  const entry: DiscountEntry = kind === "pct" ? { kind, basisPoints: value } : { kind, minor: value };
  const problem = discountProblem(entry, worth) ?? (reason ? null : "Choose a reason.");
  const max = kind === "pct" ? 10000 : worth.minor;

  return (
    <ApprovalSheet
      title={target === "order" ? "Discount the order" : `Discount ${target.name}`}
      action="Apply discount"
      ready={problem === null}
      problem={problem}
      onSubmit={(approval) =>
        onSubmit(discountRequest(target === "order" ? "order" : { lineId: target.lineId }, entry, worth.currency, reason ?? "", approval))
      }
      onClose={onClose}
    >
      <div className="flex flex-col gap-6">
        <div role="radiogroup" aria-label="Discount by" className="grid grid-cols-2 gap-tap-gap">
          {(["pct", "amount"] as const).map((k) => (
            <button
              key={k}
              type="button"
              role="radio"
              aria-checked={kind === k}
              onClick={() => {
                setKind(k);
                setValue(0);
              }}
              className={cx(
                "min-h-tap rounded border px-4 text-base",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                kind === k ? "border-line-focus bg-raised-high font-semibold text-fg" : "border-line bg-raised text-fg-2",
              )}
            >
              {k === "pct" ? "Percentage" : "Amount"}
            </button>
          ))}
        </div>
        <div className="flex items-baseline justify-between">
          <span className="text-sm text-fg-muted">
            Off <Money amount={worth} />
          </span>
          <span className="font-mono text-xl tabular-nums" aria-live="polite">
            {kind === "pct" ? percent(value) : <Money amount={{ minor: value, currency: worth.currency }} />}
          </span>
        </div>
        <NumPad layout="amount" label="Discount" onKey={(k) => setValue((v) => applyAmountKey(v, k, max))} />
        <ReasonPicker label="Why?" reasons={discountReasons} chosen={reason} onChoose={setReason} />
      </div>
    </ApprovalSheet>
  );
}

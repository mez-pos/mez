// What an order comes to (D03, D05): the rows above the rule, and the one
// total under it. Every figure is the server's projection, never arithmetic
// done here. The order rail and the tables screen's panel both show it.
//
// The rail folds it on a short screen (TILL-2): the tax rows become one,
// "incl. GST", and a tap on the subtotal and that row opens them out. The two
// rows together are the tap target, so it is 48px without costing a row.

import { ChevronDownIcon, XIcon } from "lucide-react";
import { useState } from "react";
import type { DiscountResponse, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { humanise } from "../../../shared/ui/timeline-entries";
import { amountOwed, foldedTax, taxRows } from "./rail-logic";

interface OrderTotalsProps {
  order: OrderResponse | undefined;
  currency: string;
  /** Offers to take an order discount off (ADR-373); absent where the totals are only read. */
  onRemoveDiscount?: (discount: DiscountResponse) => void;
  /** Under `short:`, fold the tax rows into one and close the rows up: the order rail's. */
  fold?: boolean;
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });
const discountLabel = (d: DiscountResponse): string => humanise(d.reason) ?? d.reason;

export function OrderTotals({ order, currency, onRemoveDiscount, fold = false }: OrderTotalsProps) {
  const [open, setOpen] = useState(false);
  const totals = order?.totals;
  const zero: Amount = { minor: 0, currency };
  const orderDiscounts = order?.discounts.filter((d) => d.scope === "order") ?? [];
  const paid = totals !== undefined && totals.paid.minor > 0;
  const subtotal = totals?.subtotal ?? zero;
  const taxes = taxRows(order, currency);
  const folded = fold ? foldedTax(order, currency) : null;

  return (
    <>
      <div className="grid gap-1 text-base">
        {totals && (totals.line_discounts.minor > 0 || totals.order_discounts.minor > 0) && <TotalRow label="Items" amount={totals.gross} />}
        {totals && totals.line_discounts.minor > 0 && <TotalRow label="Line discounts" amount={negate(totals.line_discounts)} tone="danger" />}
        {orderDiscounts.map((d) => (
          <TotalRow
            key={d.seq}
            label={`Discount · ${discountLabel(d)}`}
            amount={negate(d.value)}
            tone="danger"
            {...(onRemoveDiscount && order?.status === "open" ? { onRemove: () => onRemoveDiscount(d) } : {})}
          />
        ))}
        <span className={folded ? "contents short:hidden" : "contents"}>
          <TotalRow label="Subtotal" amount={subtotal} />
          {taxes.map((row) => (
            <TotalRow key={row.label} label={row.label} amount={row.amount} />
          ))}
        </span>
        {folded && (
          <button
            type="button"
            aria-expanded={open}
            onClick={() => setOpen((o) => !o)}
            className="hidden w-full gap-1 rounded text-left outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2 short:grid"
          >
            <TotalRow label="Subtotal" amount={subtotal} />
            <span className="flex items-center justify-between gap-2 text-fg-2">
              <span className="flex min-w-0 items-center gap-1.5">
                {folded.label}
                <ChevronDownIcon aria-hidden="true" size={16} className={cx("shrink-0", open && "rotate-180")} />
              </span>
              <Money amount={folded.amount} />
            </span>
            {open && taxes.map((row) => <TotalRow key={row.label} label={row.label} amount={row.amount} className="pl-4" />)}
          </button>
        )}
        {totals && totals.rounding.minor !== 0 && <TotalRow label="Cash rounding" amount={totals.rounding} />}
        {totals && paid && <TotalRow label="Paid" amount={negate(totals.net_paid)} />}
      </div>
      <div className={cx("mt-3 flex items-baseline justify-between border-t border-line-strong pt-3", fold && "short:mt-2 short:pt-2")}>
        <span className="font-serif text-xl">{paid ? "Balance" : "Total"}</span>
        <Money amount={amountOwed(totals, currency)} variant="total" {...(fold ? { className: "short:leading-tight" } : {})} />
      </div>
    </>
  );
}

interface TotalRowProps {
  label: string;
  amount: Amount;
  tone?: "default" | "danger";
  onRemove?: () => void;
  className?: string;
}

// A span, not a div: the folded rows sit inside a button.
function TotalRow({ label, amount, tone = "default", onRemove, className }: TotalRowProps) {
  return (
    <span className={cx("flex items-center justify-between gap-2 text-fg-2", className)}>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {onRemove && (
        <button
          type="button"
          aria-label={`Remove ${label}`}
          onClick={onRemove}
          className="flex size-tap shrink-0 items-center justify-center rounded-round outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
        >
          <XIcon aria-hidden="true" size={16} />
        </button>
      )}
      <Money amount={amount} tone={tone} />
    </span>
  );
}

// What an order comes to (D03, D05): the rows above the rule, and the one
// total under it. Every figure is the server's projection, never arithmetic
// done here. The order rail and the tables screen's panel both show it.

import type { DiscountResponse, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { humanise } from "../../../shared/ui/timeline-entries";
import { amountOwed, taxRows } from "./rail-logic";

interface OrderTotalsProps {
  order: OrderResponse | undefined;
  currency: string;
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });
const discountLabel = (d: DiscountResponse): string => humanise(d.reason) ?? d.reason;

export function OrderTotals({ order, currency }: OrderTotalsProps) {
  const totals = order?.totals;
  const zero: Amount = { minor: 0, currency };
  const orderDiscounts = order?.discounts.filter((d) => d.scope === "order") ?? [];
  const paid = totals !== undefined && totals.paid.minor > 0;

  return (
    <>
      <div className="grid gap-1 text-base">
        {totals && (totals.line_discounts.minor > 0 || totals.order_discounts.minor > 0) && <TotalRow label="Items" amount={totals.gross} />}
        {totals && totals.line_discounts.minor > 0 && <TotalRow label="Line discounts" amount={negate(totals.line_discounts)} tone="danger" />}
        {orderDiscounts.map((d) => (
          <TotalRow key={d.seq} label={`Discount · ${discountLabel(d)}`} amount={negate(d.value)} tone="danger" />
        ))}
        <TotalRow label="Subtotal" amount={totals?.subtotal ?? zero} />
        {taxRows(order, currency).map((row) => (
          <TotalRow key={row.label} label={row.label} amount={row.amount} />
        ))}
        {totals && paid && <TotalRow label="Paid" amount={negate(totals.net_paid)} />}
      </div>
      <div className="mt-3 flex items-baseline justify-between border-t border-line-strong pt-3">
        <span className="font-serif text-xl">{paid ? "Balance" : "Total"}</span>
        <Money amount={amountOwed(totals, currency)} variant="total" />
      </div>
    </>
  );
}

function TotalRow({ label, amount, tone = "default" }: { label: string; amount: Amount; tone?: "default" | "danger" }) {
  return (
    <div className="flex justify-between text-fg-2">
      <span>{label}</span>
      <Money amount={amount} tone={tone} />
    </div>
  );
}

// The foot of the rail (D03): what the order comes to, and the two things to do
// with it. The total never leaves the screen. Save Order parks it (hold); Pay
// takes it to the payment screen, in the one green. Send to Kitchen is added
// above them only while something waits to go: the design draws it on the tables
// screen, and a counter order has no other way to reach the kitchen (ADR-151).

import { ArrowRightIcon, SendIcon } from "lucide-react";
import type { DiscountResponse, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { humanise } from "../../../shared/ui/timeline-entries";
import type { SendPlan } from "./send-logic";
import { sendLabel } from "./send-logic";

interface RailFooterProps {
  order: OrderResponse | undefined;
  currency: string;
  sendPlan: SendPlan;
  /** Why Pay is disabled; null when it can be taken. */
  chargeBlock: string | null;
  /** "Pay", or what finishes an order already paid. */
  chargeLabel: string;
  onSend: () => void;
  onSave: () => void;
  onPay: () => void;
}

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });
const discountLabel = (d: DiscountResponse): string => humanise(d.reason) ?? d.reason;

export function RailFooter({ order, currency, sendPlan, chargeBlock, chargeLabel, onSend, onSave, onPay }: RailFooterProps) {
  const totals = order?.totals;
  const zero: Amount = { minor: 0, currency };
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const held = order?.status === "held";
  const orderDiscounts = order?.discounts.filter((d) => d.scope === "order") ?? [];
  const owed = totals && totals.paid.minor > 0 ? totals.balance_due : (totals?.total ?? zero);

  return (
    <footer className="border-t border-line px-5 pb-5 pt-4">
      {live && !held && sendPlan.waiting > 0 && (
        <Button className="mb-3 w-full" onClick={onSend}>
          <SendIcon aria-hidden="true" size={17} />
          {sendLabel(sendPlan)}
        </Button>
      )}
      <div className="grid gap-1 text-base">
        {totals && (totals.line_discounts.minor > 0 || totals.order_discounts.minor > 0) && <TotalRow label="Items" amount={totals.gross} />}
        {totals && totals.line_discounts.minor > 0 && <TotalRow label="Line discounts" amount={negate(totals.line_discounts)} tone="danger" />}
        {orderDiscounts.map((d) => (
          <TotalRow key={d.seq} label={`Discount · ${discountLabel(d)}`} amount={negate(d.value)} tone="danger" />
        ))}
        <TotalRow label="Subtotal" amount={totals?.subtotal ?? zero} />
        <TotalRow label="Tax" amount={totals?.tax ?? zero} />
        {totals && totals.paid.minor > 0 && <TotalRow label="Paid" amount={negate(totals.net_paid)} />}
      </div>
      <div className="mt-3 flex items-baseline justify-between border-t border-line-strong pt-3">
        <span className="font-serif text-xl">{totals && totals.paid.minor > 0 ? "Balance" : "Total"}</span>
        <Money amount={owed} variant="total" />
      </div>
      <div className="mt-4 flex gap-3">
        <Button size="charge" className="w-32 shrink-0" disabled={order?.status !== "open"} onClick={onSave}>
          Save Order
        </Button>
        <Button
          variant="primary"
          size="charge"
          disabled={chargeBlock !== null}
          onClick={onPay}
          aria-describedby={chargeBlock ? "pay-block" : undefined}
          className="flex-1"
        >
          {chargeLabel}
          {chargeLabel === "Pay" && <Money amount={owed} className="text-lg" />}
          <ArrowRightIcon aria-hidden="true" size={20} />
        </Button>
      </div>
      {chargeBlock && order && (
        <p id="pay-block" className="mt-2 text-sm text-fg-muted">
          {chargeBlock}
        </p>
      )}
    </footer>
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

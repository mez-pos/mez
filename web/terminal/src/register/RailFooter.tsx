// The foot of the rail (D03): what the order comes to, and the two things to do
// with it. The total never leaves the screen. Save Order parks it (hold); Pay
// takes it to the payment screen, in the one green. Send to Kitchen is added
// above them only while something waits to go: the design draws it on the tables
// screen, and a counter order has no other way to reach the kitchen (ADR-151).

import { ArrowRightIcon, SendIcon } from "lucide-react";
import type { OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { OrderTotals } from "./OrderTotals";
import { amountOwed } from "./rail-logic";
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

export function RailFooter({ order, currency, sendPlan, chargeBlock, chargeLabel, onSend, onSave, onPay }: RailFooterProps) {
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const held = order?.status === "held";
  const owed = amountOwed(order?.totals, currency);

  return (
    <footer className="border-t border-line px-5 pb-5 pt-4">
      {live && !held && sendPlan.waiting > 0 && (
        <Button className="mb-3 w-full" onClick={onSend}>
          <SendIcon aria-hidden="true" size={17} />
          {sendLabel(sendPlan)}
        </Button>
      )}
      <OrderTotals order={order} currency={currency} />
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

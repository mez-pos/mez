// The foot of the rail (D03): what the order comes to, and the two things to do
// with it. The total never leaves the screen. Save Order parks it (hold); Pay
// takes it to the payment screen, in the one green. Send to Kitchen is added
// above them only while something waits to go: the design draws it on the tables
// screen, and a counter order has no other way to reach the kitchen (ADR-151).
//
// A waiter's foot (T02) is Save and Send to Kitchen: sending is the one green
// there, and the bill (T05) is reached from the rail's "...", as Bill and pay.

import { ArrowRightIcon, BookmarkIcon, SendIcon } from "lucide-react";
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
  waiter?: boolean;
}

export function RailFooter({ order, currency, sendPlan, chargeBlock, chargeLabel, onSend, onSave, onPay, waiter = false }: RailFooterProps) {
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const held = order?.status === "held";
  const owed = amountOwed(order?.totals, currency);
  const canSend = live && !held && sendPlan.waiting > 0;

  if (waiter) {
    return (
      <footer className="border-t border-line px-4 pb-4 pt-3 2xl:px-5 2xl:pb-5 2xl:pt-4">
        <OrderTotals order={order} currency={currency} />
        <div className="mt-3 flex gap-3 2xl:mt-4">
          <Button size="charge" className="shrink-0 px-4 2xl:w-32 2xl:px-7" disabled={order?.status !== "open"} onClick={onSave}>
            <BookmarkIcon aria-hidden="true" size={17} />
            Save
          </Button>
          <Button variant="primary" size="charge" className="min-w-0 flex-1 px-4 2xl:px-7" disabled={!canSend} onClick={onSend}>
            <SendIcon aria-hidden="true" size={19} />
            {live ? sendLabel(sendPlan) : "Send to Kitchen"}
          </Button>
        </div>
      </footer>
    );
  }

  return (
    <footer className="border-t border-line px-4 pb-4 pt-3 2xl:px-5 2xl:pb-5 2xl:pt-4">
      {canSend && (
        <Button className="mb-3 w-full" onClick={onSend}>
          <SendIcon aria-hidden="true" size={17} />
          {sendLabel(sendPlan)}
        </Button>
      )}
      <OrderTotals order={order} currency={currency} />
      <div className="mt-3 flex gap-3 2xl:mt-4">
        <Button size="charge" className="shrink-0 px-4 2xl:w-32 2xl:px-7" disabled={order?.status !== "open"} onClick={onSave}>
          Save Order
        </Button>
        <Button
          variant="primary"
          size="charge"
          disabled={chargeBlock !== null}
          onClick={onPay}
          aria-describedby={chargeBlock ? "pay-block" : undefined}
          className="min-w-0 flex-1 px-4 2xl:px-7"
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

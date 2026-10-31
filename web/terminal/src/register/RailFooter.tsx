// The foot of the rail (D03): what the order comes to, and the two things to do
// with it. The total never leaves the screen. Save Order parks it (hold); Pay
// takes it to the payment screen, in the one green. Send to Kitchen is added
// above them only while something waits to go: the design draws it on the tables
// screen, and a counter order has no other way to reach the kitchen (ADR-151).
//
// Every label fits the rail at 1024 (TECH-3). Below 2xl Save Order is "Save"
// with its icon and Pay has the rest of the row; the arrow comes back at 2xl.
// On a short screen (TILL-2) the foot gives the lines more room: the tax rows
// fold into one, and Send to Kitchen joins the button row as "Send". Pay keeps
// the largest share of that row: Save shrinks to its icon, and Pay drops its
// amount (the total is just above it).
//
// A waiter's foot (T02) is Save and Send to Kitchen: sending is the one green
// there, and the bill (T05) is reached from the rail's "...", as Bill and pay.

import { ArrowRightIcon, BookmarkIcon, SendIcon } from "lucide-react";
import type { DiscountResponse, OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
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
  /** Takes an order discount off (ADR-373). */
  onRemoveDiscount?: (discount: DiscountResponse) => void;
}

const foot = "border-t border-line px-4 pb-4 pt-3 short:pb-3 short:pt-2 2xl:px-5 2xl:pb-5 2xl:pt-4";
const row = "mt-3 flex gap-tap-gap short:mt-2 2xl:mt-4 2xl:gap-3";
// Save gives Pay the room: its word and icon, its own width, no more.
const save = "shrink-0 px-4 2xl:px-5";

export function RailFooter({ order, currency, sendPlan, chargeBlock, chargeLabel, onSend, onSave, onPay, waiter = false, onRemoveDiscount }: RailFooterProps) {
  const totalsProps = { order, currency, fold: true, ...(onRemoveDiscount ? { onRemoveDiscount } : {}) };
  const live = order?.lines.some((l) => l.status === "active") ?? false;
  const held = order?.status === "held";
  const owed = amountOwed(order?.totals, currency);
  const canSend = live && !held && sendPlan.waiting > 0;
  const charging = chargeLabel === "Pay";
  // Send joins the row only beside a plain Pay: "Invoice and close" needs the row's width.
  const sendInRow = canSend && charging;

  if (waiter) {
    return (
      <footer className={foot}>
        <OrderTotals {...totalsProps} />
        <div className={row}>
          <Button size="charge" className={save} disabled={order?.status !== "open"} onClick={onSave}>
            <BookmarkIcon aria-hidden="true" size={17} />
            Save
          </Button>
          <Button variant="primary" size="charge" className="min-w-0 flex-1 px-4 2xl:px-7" disabled={!canSend} onClick={onSend}>
            {/* Below xl the words need the icon's room: the rail is 320px at 1024. */}
            <SendIcon aria-hidden="true" size={19} className="hidden shrink-0 xl:block" />
            {live ? sendLabel(sendPlan) : "Send to Kitchen"}
          </Button>
        </div>
      </footer>
    );
  }

  return (
    <footer className={foot}>
      {canSend && (
        <Button className={cx("mb-3 w-full short:mb-2", sendInRow && "short:hidden")} onClick={onSend}>
          <SendIcon aria-hidden="true" size={17} />
          {sendLabel(sendPlan)}
        </Button>
      )}
      <OrderTotals {...totalsProps} />
      <div className={row}>
        <Button
          size="charge"
          className={cx(save, sendInRow && "short:w-charge short:px-0")}
          disabled={order?.status !== "open"}
          onClick={onSave}
          aria-label="Save order"
        >
          <BookmarkIcon aria-hidden="true" size={17} className="2xl:hidden" />
          <span className={cx(sendInRow && "short:sr-only")}>
            Save<span className="hidden 2xl:inline"> Order</span>
          </span>
        </Button>
        {sendInRow && (
          // A wrapper, so the button's own inline-flex does not fight `hidden`.
          <div className="hidden short:contents">
            <Button size="charge" className="min-w-0 flex-2 px-3" aria-label={sendLabel(sendPlan)} onClick={onSend}>
              <SendIcon aria-hidden="true" size={17} className="hidden shrink-0 xl:block" />
              Send
            </Button>
          </div>
        )}
        <Button
          variant="primary"
          size="charge"
          disabled={chargeBlock !== null}
          onClick={onPay}
          aria-describedby={chargeBlock ? "pay-block" : undefined}
          className={cx("min-w-0 flex-1 px-4 2xl:px-6", sendInRow && "short:flex-3")}
        >
          {chargeLabel}
          {charging && <Money amount={owed} className={cx("text-lg", sendInRow && "short:hidden")} />}
          <ArrowRightIcon aria-hidden="true" size={20} className="hidden shrink-0 2xl:block" />
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

// The panel a paid bill ends on (D07): that it is paid, how much and how, the
// change still to hand back, and New Order, which closes this order and starts
// the next. The design heads it in script; a status is never script (tokens.md),
// so the heading is the serif and the script is the thank-you under it. The
// receipt is printed with the payment and reprinted from the invoice beside
// this; sending one by SMS or email does not exist.

import { ArrowRightIcon } from "lucide-react";
import type { Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { serviceLabels } from "../register/rail-logic";
import { methodNames } from "./tender-logic";

interface PaidPanelProps {
  order: OrderResponse;
  title: string;
  /** Cash still to hand back from the last payment, if any. */
  change: Amount | null;
  busy: boolean;
  /** Why New Order failed, or what it waits for. */
  message: { text: string; tone: "hint" | "error" } | null;
  onNewOrder: () => void;
}

export function PaidPanel({ order, title, change, busy, message, onNewOrder }: PaidPanelProps) {
  const methods = [...new Set(order.tenders.filter((t) => t.status !== "voided").map((t) => methodNames[t.method]))].join(" + ");
  return (
    <aside aria-label="Payment complete" className="flex w-100 shrink-0 flex-col overflow-y-auto border-l border-line bg-sunken px-6 py-7">
      <div className="text-center" aria-live="polite">
        <h2 className="font-serif text-total text-attention-text">Payment successful</h2>
        <Money amount={order.totals.net_paid} variant="display" tone="accent" className="mt-5 block text-center leading-tight" />
        <p className="mt-2 text-base text-fg-2">
          Paid for <span className="font-mono">{order.id}</span>
          <br />
          {[...new Set([title, serviceLabels[order.service], methods])].filter(Boolean).join(" · ")}
        </p>
        {order.totals.tips.minor > 0 && (
          <p className="mt-2 flex items-center justify-center gap-2 text-sm text-fg-muted">
            Tips, apart from the bill <Money amount={order.totals.tips} />
          </p>
        )}
        {change && change.minor > 0 && (
          <div className="mt-5 rounded-card border border-chip-success-border bg-raised px-5 py-4">
            <p className="text-base text-fg-2">Change to return</p>
            <Money amount={change} variant="total" tone="accent" className="block text-center leading-tight" />
          </div>
        )}
        <p aria-hidden="true" className="mt-5 font-script text-xl text-fg-2">
          Thank you for dining with us ♡
        </p>
      </div>

      <div className="mt-7 grid gap-2.5">
        <Button
          variant="attention"
          size="charge"
          disabled={busy || !order.invoiced}
          onClick={onNewOrder}
          aria-describedby={message ? "paid-message" : undefined}
        >
          {busy ? "Closing" : "New Order"}
          <ArrowRightIcon aria-hidden="true" size={19} />
        </Button>
        {message && (
          <p
            id="paid-message"
            role={message.tone === "error" ? "alert" : undefined}
            className={message.tone === "error" ? "text-sm text-danger" : "text-sm text-fg-muted"}
          >
            {message.text}
          </p>
        )}
      </div>

      <p className="mt-auto pt-8 text-center text-meta uppercase text-fg-faint">Good food brings people together</p>
    </aside>
  );
}

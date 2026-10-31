// The panel a paid bill ends on (D07): that it is paid, how much and how, the
// change still to hand back, and Next order, which starts the next. A counter
// sale is closed already, by the payment that settled it (ADR-383); a table's
// ticket is closed by Next order. A table's lines the kitchen has not seen
// are a warning here, with Send now (TILL-1). The design heads it in script; a status is never script (tokens.md),
// so the heading is the serif, in its own heading ink rather than terracotta,
// which asks for attention a paid bill does not need (ADR-381), and the script
// is the thank-you under it. The receipt is printed with the payment; Print
// receipt here prints a copy, which is a reprint and a manager's call. Sending
// one by SMS or email does not exist.

import { ArrowRightIcon, PrinterIcon } from "lucide-react";
import type { Money as Amount, OrderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { useVenueClock } from "../../../shared/use-venue";
import { ticketName } from "../bills/orders-logic";
import { serviceLabels } from "../register/rail-logic";
import { methodNames } from "./tender-logic";
import { UnsentNote } from "./UnsentNote";

interface PaidPanelProps {
  order: OrderResponse;
  title: string;
  /** Cash still to hand back from the last payment, if any. */
  change: Amount | null;
  busy: boolean;
  /** Why Next order failed, or what it waits for. */
  message: { text: string; tone: "hint" | "error" } | null;
  /** Prints a copy of the receipt, with a manager's approval. Null until there is an invoice. */
  onPrint: (() => void) | null;
  /** Lines the kitchen has not seen, with Send now; null when there are none. */
  unsent: { words: string; onSend: () => void; busy: boolean } | null;
  onNewOrder: () => void;
}

export function PaidPanel({ order, title, change, busy, message, onPrint, unsent, onNewOrder }: PaidPanelProps) {
  const { timezone } = useVenueClock();
  const methods = [...new Set(order.tenders.filter((t) => t.status !== "voided").map((t) => methodNames[t.method]))].join(" + ");
  return (
    <aside
      aria-label="Payment complete"
      className="flex w-rail shrink-0 flex-col overflow-y-auto border-l border-line bg-sunken px-5 py-6 short:py-4 2xl:px-6 2xl:py-7"
    >
      <div className="text-center" aria-live="polite">
        <h2 className="font-serif text-total leading-tight">Payment successful</h2>
        <Money amount={order.totals.net_paid} variant="display" tone="accent" className="mt-5 block text-center leading-tight short:mt-3" />
        <p className="mt-2 text-base text-fg-2">
          Paid for {ticketName(order, timezone)}
          <br />
          {[...new Set([title, serviceLabels[order.service], methods])].filter(Boolean).join(" · ")}
        </p>
        {order.totals.tips.minor > 0 && (
          <p className="mt-2 flex items-center justify-center gap-2 text-sm text-fg-muted">
            Tips, apart from the bill <Money amount={order.totals.tips} />
          </p>
        )}
        {change && change.minor > 0 && (
          <div className="mt-5 rounded-card border border-chip-success-border bg-raised px-5 py-4 short:mt-3 short:py-3">
            <p className="text-base text-fg-2">Change to return</p>
            <Money amount={change} variant="total" tone="accent" className="block text-center leading-tight" />
          </div>
        )}
        <p aria-hidden="true" className="mt-5 font-script text-xl text-fg-2 short:mt-3">
          Thank you for dining with us ♡
        </p>
      </div>

      <div className="mt-7 grid gap-2.5 short:mt-5">
        {unsent && <UnsentNote words={unsent.words} onSend={unsent.onSend} busy={busy || unsent.busy} />}
        {onPrint && (
          <Button disabled={busy} onClick={onPrint}>
            <PrinterIcon aria-hidden="true" size={18} />
            Print receipt
          </Button>
        )}
        <Button
          variant="attention"
          size="charge"
          disabled={busy || !order.invoiced}
          onClick={onNewOrder}
          aria-describedby={message ? "paid-message" : undefined}
        >
          {busy ? "Closing" : "Next order"}
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

      <p className="mt-auto pt-8 text-center text-meta uppercase text-fg-faint short:pt-5">Good food brings people together</p>
    </aside>
  );
}

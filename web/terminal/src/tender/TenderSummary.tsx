// The tender screen's right-hand rail (terminal-tender.md): what the ticket
// comes to, what has been paid, what is still owed and the change to hand
// back. Amount due is the server's balance; it never scrolls away.

import type { ReactNode } from "react";
import type { Money as Amount, OrderResponse, PaymentMethod, TenderResponse } from "../../../shared/api-types";
import { Button, Meta, Money } from "../../../shared/ui";

interface TenderSummaryProps {
  order: OrderResponse;
  label: string;
  /** Cash to hand back, or null when there is none to show. */
  change: Amount | null;
  /** The 58px tender button. */
  action: ReactNode;
  /** Under the button: why it is disabled, or why the server refused. */
  message: { text: string; tone: "hint" | "error" } | null;
  busy: boolean;
  onBack: () => void;
}

export const methodNames: Record<PaymentMethod, string> = {
  cash: "Cash",
  card: "Card",
  upi: "UPI",
  other: "Other",
};

export function TenderSummary({ order, label, change, action, message, busy, onBack }: TenderSummaryProps) {
  const payments = order.tenders.filter((t) => t.status !== "voided");
  return (
    <aside aria-label="Payment" className="flex w-104 shrink-0 flex-col border-l border-line bg-chrome">
      <header className="border-b border-line px-5 py-4">
        <Meta items={["Charge", label]} />
        <p className="mt-1 font-mono text-base text-fg-2">{order.id}</p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
        <Row label="Total" amount={order.totals.total} />
        {payments.length > 0 && (
          <ol aria-label="Payments taken" className="mt-2 border-t border-line pt-2">
            {payments.map((t) => (
              <PaymentRow key={t.id} tender={t} />
            ))}
          </ol>
        )}
      </div>

      <footer className="border-t border-line px-5 pb-5 pt-4">
        <div className="flex items-baseline justify-between" aria-live="polite">
          <span className="font-serif text-xl">Amount due</span>
          <Money amount={order.totals.balance_due} variant="total" />
        </div>
        {change && (
          <div className="mt-3 flex items-baseline justify-between rounded border border-dashed border-line-strong bg-canvas px-4 py-3" aria-live="polite">
            <span className="text-base text-fg-2">Change due</span>
            <Money amount={change} variant="total" tone="success" />
          </div>
        )}
        <div className="mt-4 flex flex-col gap-tap-gap">
          {action}
          {message && (
            <p role={message.tone === "error" ? "alert" : undefined} className={message.tone === "error" ? "text-sm text-danger" : "text-sm text-fg-muted"}>
              {message.text}
            </p>
          )}
          <Button variant="ghost" disabled={busy} onClick={onBack}>
            Back to ticket
          </Button>
        </div>
      </footer>
    </aside>
  );
}

function Row({ label, amount }: { label: string; amount: Amount }) {
  return (
    <div className="flex justify-between py-1 text-base text-fg-2">
      <span>{label}</span>
      <Money amount={amount} />
    </div>
  );
}

function PaymentRow({ tender }: { tender: TenderResponse }) {
  return (
    <li className="py-1.5">
      <div className="flex justify-between gap-3 text-sm text-fg-muted">
        <span className="min-w-0 truncate">
          Paid · {methodNames[tender.method]}
          {tender.reference && <span className="font-mono"> · {tender.reference}</span>}
        </span>
        <Money amount={tender.amount} />
      </div>
      {tender.tip.minor > 0 && (
        <div className="flex justify-between text-sm text-fg-faint">
          <span>Tip, apart from the bill</span>
          <Money amount={tender.tip} />
        </div>
      )}
    </li>
  );
}

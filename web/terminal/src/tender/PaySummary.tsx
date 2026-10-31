// The payment screen's left panel (D07): the way back to the order, where it
// is and how it is served, its lines, what it comes to, what has been paid, and
// the two things that can still change the bill: a discount and a note. The
// totals and the payments are pinned at the foot, as the order rail's are, so
// the balance never scrolls away; the lines scroll above them (ADR-381).

import { ArrowLeftIcon, FileTextIcon, TagIcon, UsersIcon, UtensilsIcon } from "lucide-react";
import type { OrderResponse, TenderResponse } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { OrderTotals } from "../register/OrderTotals";
import { guestsText, serviceLabels } from "../register/rail-logic";
import { methodNames } from "./tender-logic";

interface PaySummaryProps {
  order: OrderResponse;
  /** "Table T4", or how the order is served. */
  title: string;
  busy: boolean;
  /** Null once the bill cannot change: it is paid, or its invoice is issued. */
  onDiscount: (() => void) | null;
  onNote: (() => void) | null;
  onBack: () => void;
}

export function PaySummary({ order, title, busy, onDiscount, onNote, onBack }: PaySummaryProps) {
  const guests = guestsText(order.covers);
  const payments = order.tenders.filter((t) => t.status !== "voided");
  const lines = order.lines.filter((l) => l.status !== "voided");

  return (
    <aside aria-label="Order" className="flex w-rail shrink-0 flex-col border-r border-line bg-chrome">
      <header className="px-5 pb-4 pt-5 short:pb-3 short:pt-4">
        <Button disabled={busy} onClick={onBack} className="w-full justify-start">
          <ArrowLeftIcon aria-hidden="true" size={18} />
          Back to Order
        </Button>
        <div className="mt-5 flex items-center gap-3 short:mt-3">
          <h1 className="min-w-0 truncate font-serif text-total leading-tight">{title}</h1>
          {/* An order with no table is headed by how it is served: no need to say it twice. */}
          {title !== serviceLabels[order.service] && (
            <span className="inline-flex min-h-8 shrink-0 items-center gap-1.5 rounded-pill bg-sunken px-3 text-sm font-semibold">
              <UtensilsIcon aria-hidden="true" size={14} />
              {serviceLabels[order.service]}
            </span>
          )}
        </div>
        <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-fg-muted short:mt-1">
          {guests && (
            <span className="inline-flex items-center gap-1.5">
              <UsersIcon aria-hidden="true" size={15} />
              {guests}
            </span>
          )}
          <span className="font-mono">{order.id}</span>
        </p>
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-5 pb-4 short:pb-3">
        <ol aria-label="Items">
          {lines.map((line) => (
            <li key={line.id} className="flex items-start gap-3 border-b border-line-soft py-3">
              <span className="min-w-0 flex-1">
                <span className="block text-base font-bold">{line.name}</span>
                {line.modifiers && line.modifiers.length > 0 && (
                  <span className="block text-sm text-fg-muted">{line.modifiers.map((m) => m.name).join(", ")}</span>
                )}
                {line.status === "comped" && <span className="block text-sm text-fg-muted">Comped</span>}
              </span>
              <span className="font-mono text-base tabular-nums text-fg-2">×{line.qty}</span>
              <Money amount={line.gross} className={cx("min-w-18 font-semibold", line.status === "active" ? "text-fg-heading" : "text-fg-muted")} />
            </li>
          ))}
        </ol>

        {(onDiscount || onNote) && (
          <div className="mt-5 grid gap-2.5 short:mt-3">
            {onDiscount && (
              <Button disabled={busy} onClick={onDiscount} className="w-full justify-start">
                <TagIcon aria-hidden="true" size={17} />
                Add Discount
              </Button>
            )}
            {onNote && (
              <button
                type="button"
                disabled={busy}
                onClick={onNote}
                className="flex min-h-tap w-full items-center gap-2.5 rounded-pill bg-raised-high px-5 text-left text-base text-fg-muted outline-offset-2 outline-attention hover:bg-sunken focus-visible:outline-3 disabled:opacity-40"
              >
                <FileTextIcon aria-hidden="true" size={17} />
                Add order note
              </button>
            )}
          </div>
        )}
      </div>

      <footer className="shrink-0 border-t border-line px-5 pb-4 pt-3 short:pb-3">
        <OrderTotals order={order} currency={order.currency} />
        {payments.length > 0 && (
          // A bill split many ways scrolls its payments here rather than push the lines off.
          <ol aria-label="Payments taken" className="mt-3 max-h-40 overflow-y-auto border-t border-line pt-2">
            {payments.map((t) => (
              <PaymentRow key={t.id} tender={t} />
            ))}
          </ol>
        )}
      </footer>
    </aside>
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

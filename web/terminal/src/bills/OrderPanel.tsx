// The selected order (D09), down the right of the Orders screen: what it is,
// how it was paid, its items and totals, and what can be done with it. A
// settled bill is reprinted, refunded or has a payment voided, each a
// manager's call; an order still open goes back to the order screen or to be
// paid. The order is read like any other and kept current by the stream.
//
// Left out, as the server has none of them: the cashier's and waiter's names
// (an order names people by an id this screen cannot read), and "Email to
// guest". In its place are the two things the old bill screen had and the
// design does not draw: the invoice as issued, and the order's history.

import { BanIcon, FileTextIcon, HistoryIcon, PrinterIcon, RotateCcwIcon } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { GuestRoundProgress, Money as Amount, OrderResponse, TenderResponse } from "../../../shared/api-types";
import { Button, Money, StatusChip, useToast } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { HistorySheet } from "../register/HistorySheet";
import { failureOf } from "../register/order-messages";
import { taxRows } from "../register/rail-logic";
import { ReprintSheet } from "../tender/InvoicePanel";
import { methodNames } from "../tender/tender-logic";
import { anythingLeft, refundReasonText } from "./bills-logic";
import { InvoiceSheet, PickPaymentSheet, VoidSheet } from "./BillSheets";
import { collectWords, paymentText, statusOf, timeText, voidableTenders, whereText } from "./orders-logic";
import { RefundSheet } from "./RefundSheet";
import { useBill } from "./use-bills";

interface OrderPanelProps {
  orderId: string;
  names: ReadonlyMap<number, string>;
  /** What an order to collect is called out by, from the day's list (ADR-253). The order itself does not say. */
  pickupCode?: string | undefined;
  /** How far the kitchen has come with an order to collect, from the same list. */
  progress?: GuestRoundProgress | undefined;
  /** Brings an order still open to the order screen. */
  onOpenOrder: (order: OrderResponse) => void;
  /** Brings an order with money owed to the payment screen. */
  onPay: (order: OrderResponse) => void;
}

type Open =
  { kind: "refund" } | { kind: "pick" } | { kind: "void"; tender: TenderResponse } | { kind: "reprint" } | { kind: "invoice" } | { kind: "history" } | null;

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });
const shell = "flex w-panel shrink-0 flex-col border-l border-line bg-chrome";

export function OrderPanel({ orderId, names, pickupCode, progress, onOpenOrder, onPay }: OrderPanelProps) {
  const { order, error, commands } = useBill(orderId);
  const [open, setOpen] = useState<Open>(null);
  const [handing, setHanding] = useState(false);
  const toast = useToast();

  if (!order) {
    return (
      <aside aria-label="Order" className={cx(shell, "items-center justify-center p-6 text-center")}>
        <p role={error ? "alert" : undefined} className={error ? "text-base text-danger" : "text-base text-fg-muted"}>
          {error ? failureOf(error) : "Loading the order"}
        </p>
      </aside>
    );
  }

  const invoice = order.invoices.at(-1);
  const closed = order.status === "closed";
  const owed = order.totals.balance_due.minor > 0;
  const canRefund = invoice !== undefined && anythingLeft(order);
  const canVoid = voidableTenders(order.tenders);
  const totals = order.totals;
  const payment = paymentText(order.tenders);
  // An order to collect that is paid and still out: the counter calls its
  // code and hands it over, which closes its ticket and frees the code.
  const collecting = pickupCode !== undefined && pickupCode !== "" && !closed;
  const kitchen = collectWords(progress);
  const handOver = () => {
    setHanding(true);
    commands
      .close()
      .then(() => toast(`${pickupCode} handed over.`, { tone: "success" }))
      .catch((err: unknown) => toast(err instanceof Error ? err.message : "That did not work. Try again.", { tone: "warning" }))
      .finally(() => setHanding(false));
  };

  return (
    <aside aria-label="Order" className={shell}>
      <div className="min-h-0 flex-1 overflow-y-auto p-4 xl:p-5 2xl:p-6">
        <header className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="wrap-break-word font-serif text-xl 2xl:text-total">{invoice?.invoice_no ?? order.id}</h2>
            <p className="mt-1 text-sm text-fg-muted">
              {whereText(order, names, pickupCode)} · {timeText(order.closed_at ?? order.opened_at, new Date())}
            </p>
          </div>
          <StatusChip status={{ kind: "day", status: statusOf(order) }} className="shrink-0" />
        </header>

        {collecting && (
          <section
            aria-label="Pickup code"
            className={cx("mt-5 rounded-card border p-4 text-center", kitchen.ready ? "border-line bg-accent-tint" : "border-line bg-raised")}
          >
            <p className="text-meta font-bold uppercase text-fg-muted">Call out</p>
            <p className="mt-1 font-mono text-display font-bold tracking-[.16em] text-accent-text">{pickupCode}</p>
            <p role="status" className="mt-1 text-base font-semibold">
              {kitchen.label}
            </p>
          </section>
        )}

        <dl className="mt-5 grid gap-2.5 text-base">
          {invoice && <Fact label="Order">{order.id}</Fact>}
          {order.covers > 0 && <Fact label="Guests">{order.covers}</Fact>}
          <Fact label="Payment">{payment ?? "Not paid yet"}</Fact>
        </dl>

        <div aria-hidden="true" className="my-5 h-px bg-line" />
        <h3 className="mb-2 text-meta font-bold uppercase text-fg-muted">Items</h3>
        <ul>
          {order.lines.map((l) => (
            <li key={l.id} className="flex items-start gap-3 border-b border-line-soft py-2.5 text-base">
              <span className="min-w-6 font-mono font-bold text-fg-muted">{l.qty}×</span>
              <span className="min-w-0 flex-1">
                <span className={cx("font-semibold", l.status === "voided" && "text-fg-muted line-through")}>{l.name}</span>
                {l.modifiers && l.modifiers.length > 0 && <span className="block text-sm text-fg-muted">{l.modifiers.map((m) => m.name).join(", ")}</span>}
                {l.status !== "active" && <span className="block text-sm text-fg-muted">{l.status === "voided" ? "Voided" : "On the house"}</span>}
                {l.credited_qty > 0 && <span className="block text-sm text-danger">{l.credited_qty} refunded</span>}
              </span>
              <Money amount={l.due} className="font-bold" />
            </li>
          ))}
        </ul>
        {order.lines.length === 0 && <p className="text-base text-fg-muted">Nothing on this order yet.</p>}

        <div className="mt-4 grid gap-1.5 text-base">
          <Row label="Subtotal" amount={totals.subtotal} />
          {taxRows(order, order.currency).map((row) => (
            <Row key={row.label} label={row.label} amount={row.amount} />
          ))}
          {(totals.credited.minor > 0 || owed) && <Row label="Total" amount={totals.total} />}
          {totals.credited.minor > 0 && <Row label="Credited" amount={negate(totals.credited)} danger />}
          {totals.refunded.minor > 0 && <Row label="Refunded" amount={negate(totals.refunded)} danger />}
          {totals.tips.minor > 0 && <Row label="Tip" amount={totals.tips} />}
          <div aria-hidden="true" className="my-1 h-px bg-line" />
          <div className="flex items-baseline justify-between">
            <span className="font-serif text-xl">{owed ? "Amount due" : "Paid"}</span>
            <Money amount={owed ? totals.balance_due : totals.net_paid} variant="heading" className="2xl:text-total" />
          </div>
        </div>

        {order.tenders.length > 0 && (
          <section aria-label="Payments taken" className="mt-5">
            <h3 className="mb-1 text-meta font-bold uppercase text-fg-muted">Payments</h3>
            <ul>
              {order.tenders.map((t) => (
                <li key={t.id} className="border-b border-line-soft py-2 text-base">
                  <div className="flex items-center justify-between gap-3">
                    <span className="flex min-w-0 flex-wrap items-center gap-2">
                      {methodNames[t.method]}
                      {t.reference && <span className="font-mono text-sm text-fg-muted">{t.reference}</span>}
                      {t.status !== "captured" && <StatusChip status={{ kind: "payment", status: t.status }} />}
                    </span>
                    <Money amount={t.amount} />
                  </div>
                  {t.refunded.minor > 0 && (
                    <div className="flex justify-between text-sm text-fg-muted">
                      <span>Refunded</span>
                      <Money amount={negate(t.refunded)} />
                    </div>
                  )}
                </li>
              ))}
            </ul>
          </section>
        )}

        {order.credits.length > 0 && (
          <section aria-label="Credit notes" className="mt-5">
            <h3 className="mb-1 text-meta font-bold uppercase text-fg-muted">Credit notes</h3>
            <ul>
              {order.credits.map((c) => (
                <li key={c.seq} className="flex items-baseline justify-between gap-3 border-b border-line-soft py-2">
                  <span className="min-w-0">
                    <span className="block font-mono text-base">{c.credit_no}</span>
                    <span className="text-sm text-fg-muted">{refundReasonText(c.reason)}</span>
                  </span>
                  <Money amount={negate(c.total)} tone="danger" />
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>

      <footer className="grid shrink-0 gap-2.5 border-t border-line p-4 pt-3 xl:p-5 xl:pt-4 2xl:p-6 2xl:pt-4">
        {owed && order.status === "reopened" && (
          <Button variant="primary" onClick={() => onPay(order)}>
            Take the balance
          </Button>
        )}
        {collecting && (
          <>
            <Button variant="primary" size="charge" disabled={owed || handing} onClick={handOver}>
              {handing ? "Handing over" : "Handed over"}
            </Button>
            {owed && <p className="text-center text-sm text-fg-muted">Take payment first: Pay, then Handed over.</p>}
          </>
        )}
        {!closed && order.status !== "reopened" && (
          <div className="flex gap-2.5">
            <Button variant={collecting ? "secondary" : "primary"} onClick={() => onOpenOrder(order)} className="flex-1">
              Open order
            </Button>
            <Button disabled={!owed || order.status === "held"} onClick={() => onPay(order)} className="flex-1">
              Pay
            </Button>
          </div>
        )}
        <Button disabled={!invoice} onClick={() => setOpen({ kind: "reprint" })}>
          <PrinterIcon aria-hidden="true" size={18} />
          Reprint receipt
        </Button>
        <div className="flex gap-2.5">
          <Button disabled={!invoice} onClick={() => setOpen({ kind: "invoice" })} className="flex-1">
            <FileTextIcon aria-hidden="true" size={18} />
            Invoice
          </Button>
          <Button onClick={() => setOpen({ kind: "history" })} className="flex-1">
            <HistoryIcon aria-hidden="true" size={18} />
            History
          </Button>
        </div>
        <div className="flex gap-2.5">
          <Button variant="danger" disabled={!canRefund || owed} onClick={() => setOpen({ kind: "refund" })} className="flex-1">
            <RotateCcwIcon aria-hidden="true" size={17} />
            Refund
          </Button>
          <Button
            variant="danger"
            disabled={canVoid.length === 0}
            onClick={() => setOpen(canVoid.length === 1 && canVoid[0] ? { kind: "void", tender: canVoid[0] } : { kind: "pick" })}
            className="flex-1"
          >
            <BanIcon aria-hidden="true" size={17} />
            Void
          </Button>
        </div>
        {closed && !canRefund && (
          <p className="text-sm text-fg-muted">{invoice ? "Everything on this bill has been credited." : "This bill has no invoice."}</p>
        )}
      </footer>

      {open?.kind === "refund" && (
        <RefundSheet
          order={order}
          credit={commands.credit}
          onDone={(no) => toast(`Credit note ${no} issued. It is printing.`, { tone: "success" })}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === "pick" && <PickPaymentSheet tenders={canVoid} onPick={(tender) => setOpen({ kind: "void", tender })} onClose={() => setOpen(null)} />}
      {open?.kind === "void" && <VoidSheet tender={open.tender} closed={closed} voidPayment={commands.voidPayment} onClose={() => setOpen(null)} />}
      {open?.kind === "history" && <HistorySheet orderId={order.id} onClose={() => setOpen(null)} />}
      {open?.kind === "invoice" && invoice && <InvoiceSheet order={order} seq={invoice.seq} onClose={() => setOpen(null)} />}
      {open?.kind === "reprint" && invoice && (
        <ReprintSheet
          invoiceNo={invoice.invoice_no}
          onSubmit={(reason, approval) => commands.reprint(invoice.seq, reason, approval)}
          onClose={() => setOpen(null)}
        />
      )}
    </aside>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-fg-2">{label}</dt>
      <dd className="min-w-0 truncate font-bold">{children}</dd>
    </div>
  );
}

function Row({ label, amount, danger = false }: { label: string; amount: Amount; danger?: boolean }) {
  return (
    <div className="flex justify-between text-fg-2">
      <span>{label}</span>
      <Money amount={amount} tone={danger ? "danger" : "default"} />
    </div>
  );
}

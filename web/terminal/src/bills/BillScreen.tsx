// A settled bill (terminal-past-bills.md): its invoice and credit notes on
// the left; its totals, payments, Refund, Reprint, History and the way back
// on the right. Void on a payment reopens a closed bill in the same command,
// and Take the balance brings it to the register to be paid again.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { IssuedInvoiceResponse, Money as Amount, OrderResponse, TenderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Meta, Money, StatusChip, useToast } from "../../../shared/ui";
import { ApprovalSheet } from "../register/ApprovalSheet";
import { HistorySheet } from "../register/HistorySheet";
import { failureOf } from "../register/order-messages";
import { ReasonPicker } from "../register/ReasonPicker";
import { orderQueryKey } from "../register/use-ticket";
import { InvoicePreview } from "../tender/InvoicePreview";
import { ReprintSheet } from "../tender/InvoicePanel";
import { methodNames } from "../tender/TenderSummary";
import { anythingLeft, paymentVoidReasons, refundReasonText, voidable } from "./bills-logic";
import { RefundSheet } from "./RefundSheet";
import { useBill } from "./use-bills";

interface BillScreenProps {
  orderId: string;
  onBack: () => void;
  /** Brings a reopened bill to the register to take its balance. */
  onTakeBalance: (orderId: string) => void;
}

type Open = { kind: "refund" } | { kind: "void"; tender: TenderResponse } | { kind: "reprint" } | { kind: "history" } | null;

export function BillScreen({ orderId, onBack, onTakeBalance }: BillScreenProps) {
  const { order, error, commands } = useBill(orderId);
  const [open, setOpen] = useState<Open>(null);
  const toast = useToast();

  if (!order) {
    return (
      <div className="m-auto text-center">
        <p className={error ? "text-base text-danger" : "text-base text-fg-muted"}>{error ? failureOf(error) : "Loading the bill"}</p>
        <Button className="mt-4" onClick={onBack}>
          Back to the register
        </Button>
      </div>
    );
  }
  const invoice = order.invoices.at(-1);
  const owed = order.totals.balance_due.minor > 0;
  const canRefund = invoice !== undefined && anythingLeft(order);

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Bill" className="flex min-w-0 flex-1 flex-col items-center gap-4 overflow-y-auto px-6 py-5">
        {invoice ? <InvoiceFor order={order} seq={invoice.seq} /> : <p className="text-base text-fg-muted">This bill has no invoice.</p>}
        {order.credits.length > 0 && (
          <section aria-label="Credit notes" className="w-88 max-w-full">
            <h2 className="font-serif text-lg">Credit notes</h2>
            <ol>
              {order.credits.map((c) => (
                <li key={c.seq} className="flex items-baseline justify-between gap-3 border-b border-line-soft py-2">
                  <span className="min-w-0">
                    <span className="block font-mono text-base">{c.credit_no}</span>
                    <span className="text-sm text-fg-muted">{refundReasonText(c.reason)}</span>
                  </span>
                  <Money amount={negate(c.total)} tone="danger" />
                </li>
              ))}
            </ol>
          </section>
        )}
      </section>

      <aside aria-label="Payments" className="flex w-104 shrink-0 flex-col border-l border-line bg-chrome">
        <header className="border-b border-line px-5 py-4">
          <Meta items={["Past bill", order.status === "closed" ? "Closed" : "Reopened"]} />
          <p className="mt-1 font-mono text-base text-fg-2">{order.id}</p>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-3">
          <Row label="Total" amount={order.totals.total} />
          {order.totals.credited.minor > 0 && <Row label="Credited" amount={negate(order.totals.credited)} />}
          {order.totals.refunded.minor > 0 && <Row label="Refunded" amount={negate(order.totals.refunded)} />}
          <ol aria-label="Payments taken" className="mt-3 border-t border-line pt-2">
            {order.tenders.map((t) => (
              <li key={t.id} className="flex flex-col gap-1 border-b border-line-soft py-2.5">
                <div className="flex items-center justify-between gap-3">
                  <span className="flex items-center gap-2 text-base">
                    {methodNames[t.method]}
                    {t.reference && <span className="font-mono text-sm text-fg-muted">{t.reference}</span>}
                    <StatusChip status={{ kind: "payment", status: t.status }} />
                  </span>
                  <Money amount={t.amount} />
                </div>
                {t.refunded.minor > 0 && (
                  <div className="flex justify-between text-sm text-fg-muted">
                    <span>Refunded</span>
                    <Money amount={negate(t.refunded)} />
                  </div>
                )}
                {voidable(t) && (
                  <Button variant="danger" className="self-start" onClick={() => setOpen({ kind: "void", tender: t })}>
                    Void
                  </Button>
                )}
              </li>
            ))}
          </ol>
        </div>
        <footer className="flex flex-col gap-tap-gap border-t border-line px-5 pb-5 pt-4">
          {owed && (
            <div className="flex items-baseline justify-between">
              <span className="font-serif text-xl">Amount due</span>
              <Money amount={order.totals.balance_due} variant="total" />
            </div>
          )}
          {owed ? (
            <Button variant="primary" size="charge" onClick={() => onTakeBalance(order.id)}>
              Take the balance
            </Button>
          ) : (
            <Button variant="primary" size="charge" disabled={!canRefund} onClick={() => setOpen({ kind: "refund" })}>
              Refund
            </Button>
          )}
          {!canRefund && !owed && (
            <p className="text-sm text-fg-muted">
              {invoice ? "Everything on this bill has been credited." : "This bill has no invoice."}
            </p>
          )}
          <Button disabled={!invoice} onClick={() => setOpen({ kind: "reprint" })}>
            Reprint
          </Button>
          <Button onClick={() => setOpen({ kind: "history" })}>History</Button>
          <Button variant="ghost" onClick={onBack}>
            Back to the register
          </Button>
        </footer>
      </aside>

      {open?.kind === "refund" && (
        <RefundSheet
          order={order}
          credit={commands.credit}
          onDone={(no) => toast(`Credit note ${no} issued. It is printing.`, { tone: "success" })}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === "void" && (
        <VoidSheet tender={open.tender} closed={order.status === "closed"} voidPayment={commands.voidPayment} onClose={() => setOpen(null)} />
      )}
      {open?.kind === "history" && <HistorySheet orderId={order.id} onClose={() => setOpen(null)} />}
      {open?.kind === "reprint" && invoice && (
        <ReprintSheet
          invoiceNo={invoice.invoice_no}
          onSubmit={(reason, approval) => commands.reprint(invoice.seq, reason, approval)}
          onClose={() => setOpen(null)}
        />
      )}
    </div>
  );
}

function InvoiceFor({ order, seq }: { order: OrderResponse; seq: number }) {
  const store = useAuthStore();
  const latest = order.invoices.find((i) => i.seq === seq);
  const invoice = useQuery({
    queryKey: [...orderQueryKey(order.id), "invoice", seq, latest?.reprints, order.last_seq],
    queryFn: ({ signal }) =>
      store.api.get<IssuedInvoiceResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/invoices/${seq}`, { signal }),
    staleTime: Infinity,
  });
  if (invoice.isError) return <p className="text-base text-danger">{failureOf(invoice.error)}</p>;
  return invoice.data ? <InvoicePreview invoice={invoice.data} /> : <p className="text-base text-fg-muted">Loading the invoice</p>;
}

interface VoidSheetProps {
  tender: TenderResponse;
  closed: boolean;
  voidPayment: (paymentId: number, reason: string, approval: { authorised_by: string; pin: string }) => Promise<void>;
  onClose: () => void;
}

function VoidSheet({ tender, closed, voidPayment, onClose }: VoidSheetProps) {
  const [reason, setReason] = useState<string | null>(null);
  return (
    <ApprovalSheet
      title={`Void the ${methodNames[tender.method].toLowerCase()} payment`}
      action="Void it"
      danger
      ready={reason !== null}
      problem={reason ? null : "Choose a reason."}
      onSubmit={(approval) => voidPayment(tender.id, reason ?? "", approval)}
      onClose={onClose}
    >
      <div className="flex flex-col gap-6">
        <p className="flex items-baseline justify-between text-base text-fg-2">
          <span>The money was never taken</span>
          <Money amount={tender.amount} />
        </p>
        {closed && <p className="text-base text-fg-2">The bill reopens so the balance can be paid again. Its invoice keeps its number.</p>}
        <ReasonPicker label="Why?" reasons={paymentVoidReasons} chosen={reason} onChoose={setReason} />
      </div>
    </ApprovalSheet>
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

const negate = (a: Amount): Amount => ({ ...a, minor: -a.minor });

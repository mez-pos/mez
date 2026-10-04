// The sheets the Orders screen opens over a settled bill: its invoice as it
// was issued, which payment to void when there are several, and the void
// itself, which takes a manager and reopens a closed bill in the same command.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { IssuedInvoiceResponse, OrderResponse, Override, TenderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Money, Sheet } from "../../../shared/ui";
import { ApprovalSheet } from "../register/ApprovalSheet";
import { failureOf } from "../register/order-messages";
import { ReasonPicker } from "../register/ReasonPicker";
import { orderQueryKey } from "../register/use-ticket";
import { InvoicePreview } from "../tender/InvoicePreview";
import { methodNames } from "../tender/tender-logic";
import { paymentVoidReasons } from "./bills-logic";

interface InvoiceSheetProps {
  order: OrderResponse;
  seq: number;
  onClose: () => void;
}

export function InvoiceSheet({ order, seq, onClose }: InvoiceSheetProps) {
  const store = useAuthStore();
  const latest = order.invoices.find((i) => i.seq === seq);
  const invoice = useQuery({
    queryKey: [...orderQueryKey(order.id), "invoice", seq, latest?.reprints, order.last_seq],
    queryFn: ({ signal }) => store.api.get<IssuedInvoiceResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/invoices/${seq}`, { signal }),
    staleTime: Infinity,
  });
  return (
    <Sheet open onClose={onClose} title="Invoice">
      <div className="flex justify-center px-6 py-4">
        {invoice.isError && (
          <p role="alert" className="text-base text-danger">
            {failureOf(invoice.error)}
          </p>
        )}
        {invoice.data ? <InvoicePreview invoice={invoice.data} /> : !invoice.isError && <p className="text-base text-fg-muted">Loading the invoice</p>}
      </div>
    </Sheet>
  );
}

interface PickPaymentSheetProps {
  tenders: readonly TenderResponse[];
  onPick: (tender: TenderResponse) => void;
  onClose: () => void;
}

/** A bill paid in several payments: which one was never taken. */
export function PickPaymentSheet({ tenders, onPick, onClose }: PickPaymentSheetProps) {
  return (
    <Sheet open onClose={onClose} title="Void which payment?">
      <ul className="grid gap-2.5 px-6 py-4">
        {tenders.map((t) => (
          <li key={t.id}>
            <button
              type="button"
              onClick={() => onPick(t)}
              className="flex min-h-charge w-full items-center justify-between gap-3 rounded border border-line-strong bg-raised px-4 text-left text-base outline-offset-2 outline-attention hover:bg-raised-high focus-visible:outline-3"
            >
              <span className="font-semibold">
                {methodNames[t.method]}
                {t.reference && <span className="ml-2 font-mono text-sm font-normal text-fg-muted">{t.reference}</span>}
              </span>
              <Money amount={t.amount} className="font-bold" />
            </button>
          </li>
        ))}
      </ul>
    </Sheet>
  );
}

interface VoidSheetProps {
  tender: TenderResponse;
  /** The bill is closed: voiding reopens it. */
  closed: boolean;
  voidPayment: (paymentId: number, reason: string, approval: Override) => Promise<void>;
  onClose: () => void;
}

export function VoidSheet({ tender, closed, voidPayment, onClose }: VoidSheetProps) {
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

// What the tender screen shows once the bill is paid (Task 3): the invoice
// the settling payment issued, Close ticket, and Reprint. A reprint is a
// manager's call, logged on the order, and never a new number.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { IssuedInvoiceResponse, OrderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button } from "../../../shared/ui";
import { ApprovalSheet } from "../register/ApprovalSheet";
import { reprintReasons } from "../register/cart-logic";
import { failureOf } from "../register/order-messages";
import { ReasonPicker } from "../register/ReasonPicker";
import { orderQueryKey, type Ticket } from "../register/use-ticket";
import { InvoicePreview } from "./InvoicePreview";

interface InvoicePanelProps {
  order: OrderResponse;
  ticket: Pick<Ticket, "issueInvoice" | "reprint">;
  /** Says why issuing a late invoice failed. */
  onError: (message: string) => void;
}

export function InvoicePanel({ order, ticket, onError }: InvoicePanelProps) {
  const store = useAuthStore();
  const latest = order.invoices.at(-1);
  const [issuing, setIssuing] = useState(false);
  const [reprinting, setReprinting] = useState(false);

  // Keyed by the reprint count, so a reprint redraws "Reprinted 1 time".
  const invoice = useQuery({
    queryKey: [...orderQueryKey(order.id), "invoice", latest?.seq, latest?.reprints],
    queryFn: ({ signal }) => store.api.get<IssuedInvoiceResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/invoices/${latest?.seq ?? 0}`, { signal }),
    enabled: latest !== undefined,
    staleTime: Infinity,
  });

  if (!latest) {
    // A bill paid before invoices came with the settling payment.
    return (
      <div className="m-auto flex flex-col items-center gap-4 text-center">
        <p className="font-serif text-total">Paid in full</p>
        <p className="text-base text-fg-muted">This bill has no invoice yet.</p>
        <Button
          disabled={issuing}
          onClick={() => {
            setIssuing(true);
            ticket
              .issueInvoice()
              .catch((err: unknown) => onError(err instanceof Error ? err.message : String(err)))
              .finally(() => setIssuing(false));
          }}
        >
          {issuing ? "Issuing" : "Issue the invoice"}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col items-center gap-4 overflow-y-auto">
      <div className="flex w-88 max-w-full items-baseline justify-between">
        <p className="font-serif text-xl">Paid in full</p>
        <Button onClick={() => setReprinting(true)}>Reprint</Button>
      </div>
      {invoice.isPending && <p className="text-base text-fg-muted">Loading the invoice</p>}
      {invoice.isError && (
        <p role="alert" className="text-base text-danger">
          {failureOf(invoice.error)}
        </p>
      )}
      {invoice.data && <InvoicePreview invoice={invoice.data} />}
      <p className="w-88 max-w-full text-sm text-fg-faint">
        The receipt went to the print queue with the payment. A reprint prints a copy marked DUPLICATE and keeps the number.
      </p>
      {reprinting && (
        <ReprintSheet
          invoiceNo={latest.invoice_no}
          onSubmit={(reason, approval) => ticket.reprint(latest.seq, reason, approval)}
          onClose={() => setReprinting(false)}
        />
      )}
    </div>
  );
}

export interface ReprintSheetProps {
  invoiceNo: string;
  onSubmit: (reason: string, approval: Parameters<Ticket["reprint"]>[2]) => Promise<void>;
  onClose: () => void;
}

export function ReprintSheet({ invoiceNo, onSubmit, onClose }: ReprintSheetProps) {
  const [reason, setReason] = useState<string | null>(null);
  return (
    <ApprovalSheet
      title={`Reprint ${invoiceNo}`}
      action="Reprint"
      ready={reason !== null}
      problem={reason ? null : "Choose a reason."}
      onSubmit={(approval) => onSubmit(reason ?? "", approval)}
      onClose={onClose}
    >
      <div className="flex flex-col gap-6">
        <p className="text-base text-fg-2">A copy with the same number. It is logged on this order with who allowed it.</p>
        <ReasonPicker label="Why?" reasons={reprintReasons} chosen={reason} onChoose={setReason} />
      </div>
    </ApprovalSheet>
  );
}

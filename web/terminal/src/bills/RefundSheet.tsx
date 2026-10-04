// Give money back on a settled bill (terminal-past-bills.md, ADR-083):
// what goes back, the server's credit note figures for it, the payment it
// goes back to, why, then a manager's PIN. Every figure is the preview's.

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import type { CreditPlanResponse, OrderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Money, NumPad, Qty, applyAmountKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { ApprovalSheet } from "../register/ApprovalSheet";
import { failureOf } from "../register/order-messages";
import { ReasonPicker } from "../register/ReasonPicker";
import { orderQueryKey } from "../register/use-ticket";
import { methodNames } from "../tender/tender-logic";
import { rateText } from "../tender/invoice-format";
import { refundReasons, refundable, selectionOf, unitsLeft, type RefundDraft } from "./bills-logic";
import type { BillCommands } from "./use-bills";

/** The most the pad types: far past any bill; the server refuses past what is left. */
const amountCeiling = 100_000_000_000;

interface RefundSheetProps {
  order: OrderResponse;
  credit: BillCommands["credit"];
  onDone: (creditNo: string) => void;
  onClose: () => void;
}

export function RefundSheet({ order, credit, onDone, onClose }: RefundSheetProps) {
  const store = useAuthStore();
  const [draft, setDraft] = useState<RefundDraft>({ kind: "items", units: {} });
  const payments = refundable(order.tenders);
  const [paymentId, setPaymentId] = useState<number | null>(payments.length === 1 ? payments[0]!.id : null);
  const [reason, setReason] = useState<string | null>(null);
  const selection = selectionOf(draft, order.currency);

  const preview = useQuery({
    queryKey: [...orderQueryKey(order.id), "credit-preview", order.last_seq, selection],
    queryFn: ({ signal }) => store.api.post<CreditPlanResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/credit-notes/preview`, selection, { signal }),
    enabled: selection !== null,
    staleTime: Infinity,
    retry: false,
  });
  const plan = selection ? preview.data : undefined;
  const problem = !plan ? "Choose what goes back." : paymentId === null ? "Choose the payment it goes back to." : reason === null ? "Choose a reason." : null;

  return (
    <ApprovalSheet
      title="Refund"
      action="Refund"
      ready={problem === null && !preview.isFetching}
      problem={problem}
      onSubmit={async (approval) => {
        if (!plan || paymentId === null || reason === null || !selection) return;
        onDone(await credit(selection, paymentId, plan.total, reason, approval));
      }}
      onClose={onClose}
    >
      <div className="flex flex-col gap-6">
        <div role="radiogroup" aria-label="What goes back" className="grid grid-cols-2 gap-tap-gap">
          {(["items", "amount"] as const).map((kind) => (
            <button
              key={kind}
              type="button"
              role="radio"
              aria-checked={draft.kind === kind}
              onClick={() => setDraft(kind === "items" ? { kind, units: {} } : { kind, minor: 0 })}
              className={cx(
                "min-h-tap rounded border px-3 text-base font-semibold outline-offset-2 outline-line-focus focus-visible:outline-2",
                draft.kind === kind ? "border-accent bg-accent-tint text-accent-text" : "border-line text-fg-2",
              )}
            >
              {kind === "items" ? "Items" : "An amount"}
            </button>
          ))}
        </div>

        {draft.kind === "items" ? (
          <ol>
            {order.lines
              .filter((l) => unitsLeft(l) > 0)
              .map((l) => (
                <li key={l.id} className="flex items-center justify-between gap-3 border-b border-line-soft py-2">
                  <span className="min-w-0">
                    <span className="block text-base font-medium">{l.name}</span>
                    <span className="text-sm text-fg-muted">
                      <span className="font-mono">{unitsLeft(l)}</span> of <span className="font-mono">{l.qty}</span> left to credit
                    </span>
                  </span>
                  <Qty
                    value={draft.units[l.id] ?? 0}
                    bounds={{ min: 0, max: unitsLeft(l) }}
                    label={l.name}
                    onChange={(n) => setDraft({ kind: "items", units: { ...draft.units, [l.id]: n } })}
                  />
                </li>
              ))}
          </ol>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="flex min-h-tap items-center justify-end rounded border border-line-focus bg-canvas px-4">
              <Money amount={{ minor: draft.minor, currency: order.currency }} className="text-xl" />
            </div>
            <NumPad
              layout="amount"
              label="Amount to credit"
              onKey={(k) => setDraft((d) => (d.kind === "amount" ? { kind: "amount", minor: applyAmountKey(d.minor, k, amountCeiling) } : d))}
            />
          </div>
        )}

        <section aria-label="Credit note" aria-live="polite" className="rounded-card border border-line bg-sunken p-4">
          {!selection && <p className="text-base text-fg-muted">The credit note's figures show here.</p>}
          {selection && preview.isPending && <p className="text-base text-fg-muted">Working it out</p>}
          {selection && preview.isError && (
            <p role="alert" className="text-base text-danger">
              {failureOf(preview.error)}
            </p>
          )}
          {plan && <PlanFigures plan={plan} />}
        </section>

        {payments.length > 0 ? (
          <div role="radiogroup" aria-label="Back to" className="flex flex-col gap-2">
            <p className="text-base font-semibold">Back to</p>
            {payments.map((t) => (
              <button
                key={t.id}
                type="button"
                role="radio"
                aria-checked={paymentId === t.id}
                onClick={() => setPaymentId(t.id)}
                className={cx(
                  "flex min-h-tap items-center justify-between gap-3 rounded border px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2",
                  paymentId === t.id ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line",
                )}
              >
                <span>
                  {methodNames[t.method]}
                  {t.reference && <span className="font-mono text-fg-muted"> {t.reference}</span>}
                </span>
                <Money amount={t.amount} />
              </button>
            ))}
          </div>
        ) : (
          <p className="text-base text-danger">No payment on this bill has anything left to refund.</p>
        )}

        <ReasonPicker label="Why?" reasons={refundReasons} chosen={reason} onChoose={setReason} />
      </div>
    </ApprovalSheet>
  );
}

function PlanFigures({ plan }: { plan: CreditPlanResponse }) {
  return (
    <div className="flex flex-col gap-1 text-base">
      {plan.lines.map((l) => (
        <div key={l.line_id} className="flex justify-between gap-3">
          <span>{l.qty > 0 ? `${l.qty} × ${l.name}` : `${l.name}, part`}</span>
          <Money amount={l.amount} />
        </div>
      ))}
      {plan.components.map((c) => (
        <div key={`${c.name}-${c.rate}`} className="flex justify-between gap-3 text-sm text-fg-muted">
          <span>
            {c.name} {rateText(c.rate)}
          </span>
          <Money amount={c.amount} />
        </div>
      ))}
      <div className="mt-2 flex items-baseline justify-between border-t border-line pt-2">
        <span className="font-serif text-xl">Credit note total</span>
        <Money amount={plan.total} variant="total" />
      </div>
    </div>
  );
}

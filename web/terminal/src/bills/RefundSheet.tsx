// Give money back on a settled bill (D09's modal, ADR-083): what goes back,
// the payment it goes back to, why, then a manager's PIN. Every figure is the
// server's credit note preview; nothing is worked out here.
//
// The design offers selected items or the whole bill. An amount, which the
// till already had, is the third choice. Its lines about the kitchen and the
// guest being told are left out: neither is. A credit note is what is issued.

import { useQuery } from "@tanstack/react-query";
import { RotateCcwIcon, ShieldAlertIcon } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { CreditPlanResponse, CreditSelection, Money as Amount, OrderResponse } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Money, NumPad, Qty, applyAmountKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { ApprovalSheet } from "../register/ApprovalSheet";
import { failureOf } from "../register/order-messages";
import { ReasonPicker } from "../register/ReasonPicker";
import { orderQueryKey } from "../register/use-ticket";
import { rateText } from "../tender/invoice-format";
import { methodNames } from "../tender/tender-logic";
import { fullSelection, leftOn, refundReasons, refundable, selectionOf, unitsLeft, unitsText, type RefundDraft } from "./bills-logic";
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
  const [draft, setDraft] = useState<RefundDraft>({ kind: "items", units: {} });
  const payments = refundable(order.tenders);
  const [paymentId, setPaymentId] = useState<number | null>(payments.length === 1 ? payments[0]!.id : null);
  const [reason, setReason] = useState<string | null>(null);
  const selection = selectionOf(draft, order.currency, order.lines);
  const everything = fullSelection(order.lines);

  const preview = usePreview(order, selection);
  // The whole bill's figure is on its card whether or not it is the one chosen.
  const whole = usePreview(order, everything);
  const plan = selection ? preview.data : undefined;
  const payment = payments.find((t) => t.id === paymentId);
  const problem = !plan
    ? "Choose what goes back."
    : !payment
      ? "Choose the payment it goes back to."
      : plan.total.minor > leftOn(payment)
        ? "That is more than this payment has left. Refund it in parts, one payment each."
        : reason === null
          ? "Choose a reason."
          : null;
  const units = draft.kind === "items" ? draft.units : {};
  const picked = unitsText(units, order.lines);

  return (
    <ApprovalSheet
      title={`Refund ${order.invoices.at(-1)?.invoice_no ?? order.id}?`}
      action={
        plan ? (
          <>
            Refund <Money amount={plan.total} />
          </>
        ) : (
          "Refund"
        )
      }
      danger
      modal={{ icon: <RotateCcwIcon size={26} strokeWidth={1.9} /> }}
      ready={problem === null && !preview.isFetching}
      problem={problem}
      onSubmit={async (approval) => {
        if (!plan || paymentId === null || reason === null || !selection) return;
        onDone(await credit(selection, paymentId, plan.total, reason, approval));
      }}
      onClose={onClose}
    >
      <div className="flex flex-col gap-5">
        <p className="text-base text-fg">Choose what to return. A credit note is issued and printed, and a manager&apos;s PIN is required.</p>

        <div role="radiogroup" aria-label="What goes back" className="grid gap-2.5">
          <Choice
            on={draft.kind === "items"}
            title="Partial refund, selected items"
            detail={picked || "Choose the items below"}
            amount={draft.kind === "items" ? plan?.total : undefined}
            onChoose={() => setDraft({ kind: "items", units: {} })}
          />
          <Choice
            on={draft.kind === "full"}
            title="Full refund"
            detail="Everything on this bill not yet credited"
            amount={whole.data?.total}
            disabled={everything === null}
            onChoose={() => setDraft({ kind: "full" })}
          />
          <Choice
            on={draft.kind === "amount"}
            title="An amount"
            detail="A part of the bill, not tied to an item"
            amount={draft.kind === "amount" ? plan?.total : undefined}
            onChoose={() => setDraft({ kind: "amount", minor: 0 })}
          />
        </div>

        {draft.kind === "items" && (
          <ol aria-label="Items to refund">
            {order.lines
              .filter((l) => unitsLeft(l) > 0)
              .map((l) => (
                <li key={l.id} className="flex items-center justify-between gap-3 border-b border-line-soft py-2">
                  <span className="min-w-0">
                    <span className="block text-base font-semibold text-fg">{l.name}</span>
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
        )}

        {draft.kind === "amount" && (
          <div className="mx-auto flex w-full max-w-80 flex-col gap-3">
            <div className="flex min-h-charge items-center justify-end rounded border border-accent bg-raised px-4 outline outline-3 outline-accent-tint">
              <Money amount={{ minor: draft.minor, currency: order.currency }} className="text-xl font-bold" />
            </div>
            <NumPad
              layout="amount"
              label="Amount to credit"
              onKey={(k) => setDraft((d) => (d.kind === "amount" ? { kind: "amount", minor: applyAmountKey(d.minor, k, amountCeiling) } : d))}
            />
          </div>
        )}

        {selection && (
          <section aria-label="Credit note" aria-live="polite" className="rounded-card border border-line bg-sunken p-4">
            {preview.isPending && <p className="text-base text-fg-muted">Working it out</p>}
            {preview.isError && (
              <p role="alert" className="text-base text-danger">
                {failureOf(preview.error)}
              </p>
            )}
            {plan && <PlanFigures plan={plan} />}
          </section>
        )}

        {payments.length > 1 && (
          <div role="radiogroup" aria-label="Back to" className="flex flex-col gap-2">
            <p className="text-lg font-semibold text-fg">Back to</p>
            {payments.map((t) => (
              <button
                key={t.id}
                type="button"
                role="radio"
                aria-checked={paymentId === t.id}
                onClick={() => setPaymentId(t.id)}
                className={cx(
                  "flex min-h-tap items-center justify-between gap-3 rounded border px-4 text-base outline-offset-2 outline-attention focus-visible:outline-3",
                  paymentId === t.id ? "border-accent bg-accent-tint font-semibold text-accent-text" : "border-line-strong bg-raised text-fg",
                )}
              >
                <span>
                  {methodNames[t.method]}
                  {t.reference && <span className="font-mono text-fg-muted"> {t.reference}</span>}
                </span>
                <Money amount={{ minor: leftOn(t), currency: t.amount.currency }} />
              </button>
            ))}
          </div>
        )}
        {payments.length === 1 && payment && (
          <p className="flex items-baseline justify-between gap-3 text-base text-fg-2">
            <span>
              Back to the {methodNames[payment.method].toLowerCase()} payment
              {payment.reference && <span className="font-mono text-fg-muted"> {payment.reference}</span>}
            </span>
            <Money amount={{ minor: leftOn(payment), currency: payment.amount.currency }} />
          </p>
        )}
        {payments.length === 0 && (
          <p role="alert" className="text-base text-danger">
            No payment on this bill has anything left to refund.
          </p>
        )}

        <ReasonPicker label="Reason (required)" reasons={refundReasons} chosen={reason} onChoose={setReason} />

        <p className="flex gap-3 rounded border border-chip-warning-border bg-chip-warning-bg p-3.5 text-sm text-fg">
          <ShieldAlertIcon aria-hidden="true" size={19} strokeWidth={1.9} className="shrink-0 text-chip-warning-fg" />
          Refunds appear on the Z-report and in the manager&apos;s audit trail.
        </p>
      </div>
    </ApprovalSheet>
  );
}

/** The server's credit note figures for a selection. */
function usePreview(order: OrderResponse, selection: CreditSelection | null) {
  const store = useAuthStore();
  return useQuery({
    queryKey: [...orderQueryKey(order.id), "credit-preview", order.last_seq, selection],
    queryFn: ({ signal }) => store.api.post<CreditPlanResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/credit-notes/preview`, selection, { signal }),
    enabled: selection !== null,
    staleTime: Infinity,
    retry: false,
  });
}

interface ChoiceProps {
  on: boolean;
  title: string;
  detail: ReactNode;
  /** The server's figure for it, once there is one. */
  amount: Amount | undefined;
  disabled?: boolean;
  onChoose: () => void;
}

/** One way to refund, as D09 draws it: a radio mark, what it is, and what it comes to. */
function Choice({ on, title, detail, amount, disabled = false, onChoose }: ChoiceProps) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={on}
      disabled={disabled}
      onClick={() => !on && onChoose()}
      className={cx(
        "flex min-h-charge items-center gap-3 rounded border px-4 py-3 text-left outline-offset-2 outline-attention focus-visible:outline-3 disabled:opacity-40",
        on ? "border-accent bg-accent-tint" : "border-line-strong bg-raised hover:bg-raised-high",
      )}
    >
      <span
        aria-hidden="true"
        className={cx("flex size-5 shrink-0 items-center justify-center rounded-round border-2", on ? "border-accent" : "border-line-strong")}
      >
        {on && <span className="size-2.5 rounded-round bg-accent" />}
      </span>
      <span className="min-w-0 flex-1">
        <span className={cx("block text-base font-bold", on ? "text-accent-text" : "text-fg")}>{title}</span>
        <span className="block text-sm text-fg-2">{detail}</span>
      </span>
      {amount && <Money amount={amount} variant="heading" />}
    </button>
  );
}

function PlanFigures({ plan }: { plan: CreditPlanResponse }) {
  return (
    <div className="flex flex-col gap-1 text-base text-fg">
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
        <Money amount={plan.total} variant="heading" />
      </div>
    </div>
  );
}

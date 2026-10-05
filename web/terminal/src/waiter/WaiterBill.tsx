// The waiter's bill (T05), at "/pay/:id" for whoever works from the waiter's
// frame: the table and what it owes, how the bill is split, a row for each
// share, the share being collected and how, and along the foot what has been
// collected and Close table.
//
// The design has no "Complete split" step: the guests are counted and Collect
// is tapped. So an equal split stays the server's live answer until a payment
// begins, and is settled from then on: what is left would divide differently.
// By item and Custom are made on the split panel the payment screen has, which
// takes the screen while it is up.
//
// Left out: Print bill before payment (no bill is printed until it is paid and
// numbered, ADR-078), the QR and the Wallet tile (BillCollect.tsx). Print
// receipt comes with the invoice, and is a copy with a manager's approval.

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ArrowLeftIcon, ArrowRightIcon, CircleCheckIcon, PrinterIcon } from "lucide-react";
import { useState } from "react";
import { Link } from "wouter";
import type { Money as Amount, OrderResponse, PaymentMethod, SplitResponse, TenderPayment } from "../../../shared/api-types";
import { useAuthStore } from "../../../shared/auth/use-auth";
import { Button, Money, applyAmountKey, useToast, type NumKey } from "../../../shared/ui";
import { wordsOf } from "../register/order-messages";
import { newKey } from "../register/ticket-key";
import { orderQueryKey, type Ticket } from "../register/use-ticket";
import { ReprintSheet } from "../tender/InvoicePanel";
import { maxWays, splitStale, type SplitDraft, type TillSplit } from "../tender/split-logic";
import { SplitPanel } from "../tender/SplitPanel";
import { attemptKey, methodNames, planTender, quickTenders, type Attempt } from "../tender/tender-logic";
import { closeBlock, eachShare, shareName, shareRows, tablePays, type PaidShares } from "./bill-logic";
import { BillCollect } from "./BillCollect";
import { BillShares } from "./BillShares";
import { backFromOrder } from "./table-logic";

interface WaiterBillProps {
  order: OrderResponse;
  ticket: Ticket;
  /** "Table T4", or how the order is served. */
  title: string;
  /** The table is closed: back to the tables, where it is cleared. */
  onClosed: () => void;
}

/** A split whose shares no longer move: one the split panel completed, or an equal one a payment began on. */
interface Settled {
  split: SplitResponse;
  amountsKeyed: number;
}

/** The most the keypad types for cash: ₹10 crore, far past any bill. */
const padCeiling = 1_000_000_000;

const nobodyPaid: PaidShares = new Map();

export function WaiterBill({ order, ticket, title, onClosed }: WaiterBillProps) {
  const store = useAuthStore();
  const toast = useToast();
  const currency = order.currency;
  const money = (minor: number): Amount => ({ minor, currency });
  const due = order.totals.balance_due.minor;
  const [draft, setDraft] = useState<SplitDraft>({ kind: "evenly", ways: 1, amounts: [], payers: {} });
  const [making, setMaking] = useState(false);
  const [settled, setSettled] = useState<Settled | null>(null);
  const [paid, setPaid] = useState<PaidShares>(nobodyPaid);
  const [picked, setPicked] = useState<number | null>(null);
  const [method, setMethod] = useState<PaymentMethod>("upi");
  const [entered, setEntered] = useState(0);
  const [tip, setTip] = useState(0);
  const [reference, setReference] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [change, setChange] = useState<number | null>(null);
  const [reprinting, setReprinting] = useState(false);

  // The equal split, asked again whenever the guests or the order move, as the
  // split panel asks. The last answer stays up, dimmed, while the next comes.
  const request = { kind: "evenly" as const, ways: draft.ways };
  const live = useQuery({
    queryKey: [...orderQueryKey(order.id), "split", order.last_seq, request],
    queryFn: ({ signal }) => store.api.post<SplitResponse>(`/api/v1/orders/${encodeURIComponent(order.id)}/split`, request, { signal }),
    enabled: due > 0 && !settled && !making,
    staleTime: Infinity,
    retry: false,
    placeholderData: keepPreviousData,
  });
  const settling = !settled && (live.isPlaceholderData || live.isFetching);
  const split = settled?.split ?? (due > 0 ? live.data : undefined);
  const amountsKeyed = settled?.amountsKeyed ?? 0;
  const stale = settled !== null && due > 0 && splitStale(settled.split, new Set(paid.keys()), due);
  const collecting = split && !stale && due > 0 ? tablePays(split, paid, picked) : null;
  const share = split?.shares.find((s) => s.payer === collecting);
  const target = share?.amount.minor ?? 0;
  const cash = method === "cash";
  const plan = planTender({ method, entered: cash ? entered : target, tip, due: target, whole: true });
  const invoice = order.invoices.at(-1);
  const block = closeBlock(order);
  // While money is owed the rows say why the table stays open; any other reason is said.
  const closeNote = due > 0 ? null : block;

  /** A new payment: nothing keyed for the last one carries over. */
  const fresh = () => {
    setEntered(0);
    setTip(0);
    setReference("");
    setAttempt(null);
    setError(null);
  };

  const letGo = () => {
    setSettled(null);
    setPaid(nobodyPaid);
    setPicked(null);
    setDraft({ ...draft, kind: "evenly" });
    fresh();
  };

  const send = async () => {
    if (!split || collecting === null || plan.problem || busy) return;
    const ref = reference.trim();
    const body: TenderPayment = { method, amount: money(plan.amount), tip: money(tip), ...(!cash && ref ? { reference: ref } : {}) };
    const next = attemptKey(attempt, body, () => newKey());
    const from = shareName(split.kind, collecting, amountsKeyed, split.shares.length);
    setAttempt(next);
    setBusy(true);
    setError(null);
    // Settled before the money goes, and it stays so if the answer is lost: a
    // payment recorded unseen must not have its share divided again.
    if (!settled) setSettled({ split, amountsKeyed: 0 });
    try {
      await ticket.tender(body, next.key);
      setPaid(new Map(paid).set(collecting, method));
      setPicked(null);
      setChange(cash ? plan.change : null);
      fresh();
      toast(`${methodNames[method]} payment recorded · ${from}`);
    } catch (err) {
      setError(wordsOf(err));
    } finally {
      setBusy(false);
    }
  };

  const close = () => {
    setBusy(true);
    setError(null);
    ticket
      .close()
      .then(() => {
        toast(`${title} closed`);
        onClosed();
      })
      .catch((err: unknown) => {
        setError(wordsOf(err));
        setBusy(false);
      });
  };

  const openPanel = (kind: TillSplit) => {
    if (kind === "evenly") return;
    setDraft({ ...draft, kind, ways: Math.max(2, Math.min(draft.ways, kind === "item" ? maxWays - 1 : maxWays)) });
    setMaking(true);
  };

  if (making) {
    return (
      <div className="flex min-w-0 flex-1 flex-col px-5 py-4">
        <SplitPanel
          order={order}
          title={title}
          draft={draft}
          onDraft={setDraft}
          onUse={(made, keyed) => {
            setSettled({ split: made, amountsKeyed: keyed });
            setPaid(nobodyPaid);
            setPicked(null);
            setMaking(false);
            fresh();
          }}
          onBack={() => {
            setDraft({ ...draft, kind: "evenly" });
            setMaking(false);
          }}
        />
      </div>
    );
  }

  const back = backFromOrder(order);
  const each = split && (settled?.split.kind ?? "evenly") === "evenly" ? eachShare(split) : null;
  const methods = [...new Set(order.tenders.filter((t) => t.status !== "voided").map((t) => methodNames[t.method]))].join(" + ");

  return (
    <div className="flex min-w-0 flex-1 justify-center overflow-hidden px-5 py-4">
      <section aria-label="Settle bill" className="flex w-165 max-w-full flex-col overflow-hidden rounded-sheet border border-line bg-canvas shadow-modal">
        <header className="flex min-h-18 shrink-0 items-center gap-3.5 border-b border-line bg-chrome px-5">
          <Link
            href={back.href}
            aria-label={back.label}
            className="flex size-tap shrink-0 items-center justify-center rounded-round bg-sunken outline-offset-2 outline-line-focus hover:bg-raised-high focus-visible:outline-2"
          >
            <ArrowLeftIcon aria-hidden="true" size={20} />
          </Link>
          <h1 className="min-w-0 truncate font-serif text-xl">Settle bill · {title}</h1>
          <Money amount={order.totals.total} variant="heading" className="ml-auto shrink-0" />
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          {due > 0 && (
            <>
              <BillShares
                kind={settled?.split.kind ?? "evenly"}
                fixed={settled !== null}
                partPaid={order.totals.net_paid.minor !== 0}
                ways={settled ? settled.split.shares.length : draft.ways}
                rows={split ? shareRows(split, amountsKeyed, paid, collecting) : []}
                each={each !== null ? money(each) : null}
                settling={settling}
                disabled={busy}
                onWays={(ways) => {
                  setDraft({ ...draft, ways });
                  setPicked(null);
                  fresh();
                }}
                onMode={openPanel}
                onChange={letGo}
                onCollect={(payer) => {
                  setPicked(payer);
                  fresh();
                }}
              />
              {!split && (
                <p role={live.isError ? "alert" : undefined} className={live.isError ? "mt-4 text-base text-danger" : "mt-4 text-base text-fg-muted"}>
                  {live.isError ? wordsOf(live.error) : "Working it out"}
                </p>
              )}
              {stale && (
                <div role="alert" className="mt-4 rounded bg-danger-tint p-4 text-base text-danger">
                  <p>The bill changed after it was split, or money was taken elsewhere. Split what is left again.</p>
                  <Button disabled={busy} onClick={letGo} className="mt-3">
                    Split it again
                  </Button>
                </div>
              )}
              {change !== null && change > 0 && (
                <p
                  aria-live="polite"
                  className="mt-4 flex items-baseline justify-between gap-3 rounded-card border border-chip-success-border bg-accent-tint px-4 py-3 text-base"
                >
                  Change to return from the last payment
                  <Money amount={money(change)} variant="heading" tone="accent" />
                </p>
              )}
              {share && split && (
                <BillCollect
                  from={split.shares.length > 1 ? shareName(split.kind, share.payer, amountsKeyed, split.shares.length) : null}
                  target={share.amount}
                  method={method}
                  entered={entered}
                  tip={tip}
                  reference={reference}
                  plan={plan}
                  // The tip comes out of the cash: "exact" is the share and the tip.
                  quick={cash ? quickTenders(target + tip, currency) : []}
                  busy={busy || settling}
                  error={error}
                  onMethod={(next) => {
                    setMethod(next);
                    fresh();
                  }}
                  onEntered={(minor) => {
                    setEntered(minor);
                    setError(null);
                  }}
                  onKey={(k: NumKey) => {
                    setEntered((e) => applyAmountKey(e, k, padCeiling));
                    setError(null);
                  }}
                  onTip={(minor) => {
                    setTip(minor);
                    setError(null);
                  }}
                  onReference={setReference}
                  onSend={() => void send()}
                />
              )}
            </>
          )}

          {due <= 0 && (
            <div aria-live="polite" className="flex flex-col items-center px-4 py-8 text-center">
              <span aria-hidden="true" className="flex size-16 items-center justify-center rounded-round bg-chip-success-bg text-chip-success-fg">
                <CircleCheckIcon size={34} strokeWidth={1.75} />
              </span>
              <h2 className="mt-4 font-serif text-total">Paid in full</h2>
              <p className="mt-2 text-base text-fg-2">{[methods, invoice ? `Invoice ${invoice.invoice_no}` : null].filter(Boolean).join(" · ")}</p>
              {order.totals.tips.minor > 0 && (
                <p className="mt-1 text-sm text-fg-muted">
                  Tips, apart from the bill <Money amount={order.totals.tips} />
                </p>
              )}
              {change !== null && change > 0 && (
                <div className="mt-5 rounded-card border border-chip-success-border bg-raised px-6 py-4">
                  <p className="text-base text-fg-2">Change to return</p>
                  <Money amount={money(change)} variant="total" tone="accent" className="block text-center leading-tight" />
                </div>
              )}
              <p className="mt-5 text-sm text-fg-muted">The receipt went to the print queue with the payment.</p>
              <p aria-hidden="true" className="mt-5 font-script text-xl text-fg-2">
                Good Teams, Great Service ♡
              </p>
              {error && (
                <p role="alert" className="mt-4 text-base text-danger">
                  {error}
                </p>
              )}
            </div>
          )}
        </div>

        <footer className="flex shrink-0 flex-wrap items-center gap-3 border-t border-line bg-chrome px-5 py-4">
          <div className="min-w-0 flex-1">
            <p className="text-sm text-fg-muted">Collected so far</p>
            <p className="flex items-baseline gap-2">
              <Money amount={order.totals.net_paid} variant="heading" />
              <span className="text-base text-fg-muted">
                of <Money amount={order.totals.total} />
              </span>
            </p>
          </div>
          {invoice && (
            <Button size="charge" disabled={busy} onClick={() => setReprinting(true)}>
              <PrinterIcon aria-hidden="true" size={18} />
              Print receipt
            </Button>
          )}
          <Button
            variant="primary"
            size="charge"
            disabled={busy || block !== null}
            onClick={close}
            aria-describedby={closeNote ? "close-note" : undefined}
            className="w-60"
          >
            {busy && block === null ? "Closing" : order.table_id ? "Close table" : "Close order"}
            <ArrowRightIcon aria-hidden="true" size={19} />
          </Button>
          {closeNote && (
            <p id="close-note" className="w-full text-sm text-fg-muted">
              {closeNote}
            </p>
          )}
        </footer>
      </section>

      {reprinting && invoice && (
        <ReprintSheet
          invoiceNo={invoice.invoice_no}
          onSubmit={(reason, approval) => ticket.reprint(invoice.seq, reason, approval)}
          onClose={() => setReprinting(false)}
        />
      )}
    </div>
  );
}

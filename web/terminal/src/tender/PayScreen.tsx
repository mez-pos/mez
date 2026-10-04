// The payment screen (D07): the order down the left, and in the middle the
// three steps. Payment is a method, the amount received with the change to
// return, and a tip kept apart from the bill. Split Bill hands the bill out in
// shares, each then one payment. Complete is the invoice, with the paid panel
// on the right and New Order. ADR-156 records what this screen draws.
//
// Every figure is the server's but the change, the part of a cash payment that
// settles the bill, and a tip offered as a percentage, which tender-logic.ts
// works out. A split comes from the server too.

import { useState } from "react";
import type { Money as Amount, OrderResponse, PaymentMethod, SplitResponse, TenderPayment } from "../../../shared/api-types";
import { Button, Money, NumPad, applyAmountKey, type NumKey } from "../../../shared/ui";
import { ArrowRightIcon } from "lucide-react";
import { TicketPanels, type Panel } from "../register/TicketPanels";
import { newKey } from "../register/ticket-key";
import type { Ticket } from "../register/use-ticket";
import { AmountCard } from "./AmountCard";
import { InvoicePanel, ReprintSheet } from "./InvoicePanel";
import { PaidPanel } from "./PaidPanel";
import { MethodTiles, PaySteps } from "./PaySteps";
import { PaySummary } from "./PaySummary";
import { nextShare, splitStale } from "./split-logic";
import { SplitPanel } from "./SplitPanel";
import { SplitShares } from "./SplitShares";
import { attemptKey, collectOf, payStep, planTender, quickTenders, type Attempt } from "./tender-logic";
import { TipCard } from "./TipCard";

interface PayScreenProps {
  order: OrderResponse;
  ticket: Ticket;
  /** "Table T4", or how the order is served. */
  title: string;
  onBack: () => void;
}

/** The most the pad types for cash or a tip: ₹10 crore, far past any bill. */
const padCeiling = 1_000_000_000;

type Field = "amount" | "tip";

/** A split in use: the server's shares, which are paid, and which is next. */
interface SplitInUse {
  split: SplitResponse;
  amountsKeyed: number;
  paid: ReadonlySet<number>;
  current: number | null;
}

export function PayScreen({ order, ticket, title, onBack }: PayScreenProps) {
  const { tender } = ticket;
  const currency = order.currency;
  const due = order.totals.balance_due.minor;
  const [method, setMethod] = useState<PaymentMethod>("cash");
  // Null means "the balance": card and UPI follow it as it moves.
  const [entered, setEntered] = useState<number | null>(0);
  const [tip, setTip] = useState(0);
  const [reference, setReference] = useState("");
  const [field, setField] = useState<Field>("amount");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [lastChange, setLastChange] = useState<number | null>(null);
  const [splitting, setSplitting] = useState(false);
  const [inUse, setInUse] = useState<SplitInUse | null>(null);
  const [panel, setPanel] = useState<Panel | null>(null);
  const [reprinting, setReprinting] = useState(false);
  const invoice = order.invoices.at(-1);

  const paid = due <= 0;
  const step = payStep(paid, splitting);
  const stale = inUse !== null && splitStale(inUse.split, inUse.paid, due);
  const share = inUse && inUse.current !== null && !stale ? inUse.split.shares[inUse.current] : undefined;
  // A share is paid whole: card and UPI take exactly it, cash at least it.
  const target = share ? share.amount.minor : due;
  const whole = share !== undefined;
  const cash = method === "cash";
  const amountShown = entered === null || (whole && !cash) ? target : cash ? entered : Math.min(entered, target);
  const plan = stale
    ? { amount: 0, change: 0, charge: 0, problem: "Split the bill again before taking payment." }
    : planTender({ method, entered: amountShown, tip, due: target, whole });
  const money = (minor: number): Amount => ({ minor, currency });
  const collect = collectOf(plan, target, tip);

  const choose = (next: PaymentMethod) => {
    setMethod(next);
    setEntered(next === "cash" ? 0 : null);
    setTip(0);
    setReference("");
    setField(next !== "cash" && whole ? "tip" : "amount");
    setError(null);
  };

  const takeSplit = (split: SplitResponse, amountsKeyed: number) => {
    setInUse({ split, amountsKeyed, paid: new Set(), current: 0 });
    setSplitting(false);
    // A tip keyed against the whole bill is not the first share's.
    setTip(0);
    setEntered(cash ? 0 : null);
    setField(cash ? "amount" : "tip");
    setError(null);
  };

  /** A key for a field, from the keypad or a keyboard. */
  const keyIn = (into: Field, k: NumKey) => {
    setError(null);
    if (into === "tip") {
      setTip((t) => applyAmountKey(t, k, padCeiling));
      return;
    }
    if (whole && !cash) return;
    // A card amount that is following the balance starts afresh on the first
    // digit, so a part payment is keyed, not the balance backspaced away.
    const from = entered === null && k !== "back" ? 0 : amountShown;
    setEntered(applyAmountKey(from, k, cash ? padCeiling : target));
  };

  const send = async () => {
    if (plan.problem || busy) return;
    const ref = reference.trim();
    const body: TenderPayment = {
      method,
      amount: money(plan.amount),
      tip: money(tip),
      ...(!cash && ref ? { reference: ref } : {}),
    };
    const next = attemptKey(attempt, body, () => newKey());
    setAttempt(next);
    setBusy(true);
    setError(null);
    try {
      await tender(body, next.key);
      setAttempt(null);
      if (inUse && inUse.current !== null) {
        const paidShares = new Set(inUse.paid).add(inUse.current);
        setInUse({ ...inUse, paid: paidShares, current: nextShare(inUse.split, paidShares) });
      }
      setLastChange(cash ? plan.change : null);
      setEntered(cash ? 0 : null);
      setTip(0);
      setReference("");
      setField(!cash && inUse ? "tip" : "amount");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  /** New Order: this order is closed, and the till starts the next one. */
  const closeOrder = () => {
    setBusy(true);
    setError(null);
    ticket
      .close()
      .catch((err: unknown) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setBusy(false));
  };

  const typing = cash && amountShown > 0;
  const change = typing ? money(plan.change) : lastChange !== null ? money(lastChange) : null;
  const message = error ? { text: error, tone: "error" as const } : !paid && plan.problem ? { text: plan.problem, tone: "hint" as const } : null;
  // A payment that leaves some of the bill owed says so before it is taken.
  const short = !paid && plan.problem === null && !error ? target - plan.amount : 0;
  // A discount or a note changes the bill: not once it is paid or invoiced.
  const open = !paid && !order.invoiced;

  return (
    <div className="flex min-w-0 flex-1">
      <PaySummary
        order={order}
        title={title}
        busy={busy}
        onDiscount={open ? () => setPanel({ kind: "discount", lineId: null }) : null}
        onNote={open ? () => setPanel({ kind: "note" }) : null}
        onBack={onBack}
      />

      <section aria-label="Payment" className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-6 pb-5 pt-5 2xl:px-7 2xl:pt-6">
          <PaySteps
            step={step}
            disabled={busy}
            onPayment={() => setSplitting(false)}
            onSplit={() => {
              setSplitting(true);
              setError(null);
            }}
          />

          {step === "split" && (
            <div className="mt-6">
              <SplitPanel order={order} onUse={takeSplit} onCancel={() => setSplitting(false)} />
            </div>
          )}

          {step === "complete" && (
            <div className="mt-6 flex min-h-0 flex-1 flex-col">
              <InvoicePanel order={order} ticket={ticket} onError={setError} />
            </div>
          )}

          {step === "payment" && (
            <>
              {inUse && (
                <div className="mt-5">
                  <SplitShares
                    split={inUse.split}
                    amountsKeyed={inUse.amountsKeyed}
                    paid={inUse.paid}
                    current={inUse.current}
                    stale={stale}
                    disabled={busy}
                    onPick={(payer) => {
                      setInUse({ ...inUse, current: payer });
                      setEntered(cash ? 0 : null);
                      setTip(0);
                      // Another share is another payment: it never reuses a failed one's key.
                      setAttempt(null);
                      setError(null);
                    }}
                    onClear={() => {
                      setInUse(null);
                      setEntered(cash ? 0 : null);
                      setTip(0);
                      setField("amount");
                    }}
                  />
                </div>
              )}
              <h2 className="mt-5 font-serif text-xl 2xl:mt-6 2xl:text-total">Select payment method</h2>
              <div className="mt-3 2xl:mt-4">
                <MethodTiles method={method} disabled={busy} onChoose={choose} />
              </div>

              <div className="mt-5 grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-4 2xl:grid-cols-[minmax(0,1fr)_23.75rem_19rem]">
                <AmountCard
                  method={method}
                  target={money(target)}
                  share={whole}
                  amount={money(amountShown)}
                  active={field === "amount"}
                  // The tip comes out of the cash: "exact" is the bill and the tip.
                  quick={cash ? quickTenders(target + tip, currency) : []}
                  change={change}
                  reference={reference}
                  busy={busy}
                  onActivate={() => setField("amount")}
                  onKey={(k) => keyIn("amount", k)}
                  onQuick={(q) => {
                    setEntered(q);
                    setField("amount");
                    setError(null);
                  }}
                  onCustom={() => {
                    setEntered(0);
                    setField("amount");
                    setError(null);
                  }}
                  onReference={(text) => {
                    setReference(text);
                    setError(null);
                  }}
                />
                <TipCard
                  bill={money(plan.problem === null ? plan.amount : target)}
                  base={target}
                  tip={money(tip)}
                  collect={money(collect)}
                  cash={cash}
                  active={field === "tip"}
                  busy={busy}
                  onActivate={() => setField("tip")}
                  onKey={(k) => keyIn("tip", k)}
                  onTip={(minor) => {
                    setTip(minor);
                    setError(null);
                  }}
                />
                {/* The keypad the design leaves to a keyboard: a touch till has none (rule 4). */}
                <div className="col-span-2 max-w-80 rounded-card border border-line bg-raised p-4 2xl:col-span-1 2xl:max-w-none">
                  <NumPad
                    layout="amount"
                    label={field === "tip" ? "Tip" : cash ? "Amount received" : "Amount"}
                    onKey={(k) => keyIn(field, k)}
                    disabled={busy}
                  />
                </div>
              </div>
            </>
          )}
        </div>

        {step === "payment" && (
          <div className="flex shrink-0 items-center gap-5 border-t border-line bg-canvas px-6 py-3.5 2xl:px-7">
            <p
              role={message?.tone === "error" ? "alert" : undefined}
              className={message?.tone === "error" ? "flex-1 text-base text-danger" : "flex-1 text-base text-fg-muted"}
            >
              {message?.text}
              {short > 0 && (
                <>
                  A part payment: <Money amount={money(short)} /> stays owed.
                </>
              )}
            </p>
            <Button variant="primary" size="charge" disabled={busy || plan.problem !== null} onClick={() => void send()}>
              {busy ? "Sending" : "Complete payment"}
              {!busy && (
                <>
                  <span aria-hidden="true">·</span>
                  <Money amount={money(collect)} className="text-lg" />
                  <ArrowRightIcon aria-hidden="true" size={20} />
                </>
              )}
            </Button>
          </div>
        )}
      </section>

      {step === "complete" && (
        <PaidPanel
          order={order}
          title={title}
          change={lastChange !== null ? money(lastChange) : null}
          busy={busy}
          message={error ? { text: error, tone: "error" } : !order.invoiced ? { text: "Issue the invoice first.", tone: "hint" } : null}
          onPrint={invoice ? () => setReprinting(true) : null}
          onNewOrder={closeOrder}
        />
      )}

      {reprinting && invoice && (
        <ReprintSheet
          invoiceNo={invoice.invoice_no}
          onSubmit={(reason, approval) => ticket.reprint(invoice.seq, reason, approval)}
          onClose={() => setReprinting(false)}
        />
      )}

      <TicketPanels panel={panel} onPanel={setPanel} ticket={ticket} hasChoices={() => false} onModify={() => undefined} />
    </div>
  );
}

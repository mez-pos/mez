// The payment screen (D07): the order down the left, and in the middle the
// three steps. Payment is a method, the amount received with the change to
// return, and a tip kept apart from the bill. Split Bill hands the bill out in
// shares, each then one payment (D08a). A card or UPI payment waits to be
// marked paid, and one that leaves some owed is confirmed (D08b). Complete is
// the invoice, with the paid panel on the right and New Order. ADR-156 and
// ADR-159 record what this screen draws.
//
// Every figure is the server's but the change, the part of a cash payment that
// settles the bill, and a tip offered as a percentage, which tender-logic.ts
// works out. A split comes from the server too.
//
// A tip over the venue's ceiling needs a manager (ADR-261). The primary button
// opens the approval first: cash is then sent with it; a card or UPI payment
// carries it to Mark as paid, the approval given before the machine is keyed.
// The server checks the ceiling and the PIN again, and when it asks for an
// approval the till did not know it needed, the sheet opens then.

import { useEffect, useState } from "react";
import type { Money as Amount, OrderResponse, Override, PaymentMethod, SplitResponse, TenderRequest } from "../../../shared/api-types";
import { Button, Money, NumPad, applyAmountKey, type NumKey } from "../../../shared/ui";
import { ArrowRightIcon } from "lucide-react";
import { useVenue } from "../../../shared/use-venue";
import { clearUnsettled, loadUnsettled, saveUnsettled } from "./unsettled-store";
import { TicketPanels, type Panel } from "../register/TicketPanels";
import { newKey } from "../register/ticket-key";
import type { Ticket } from "../register/use-ticket";
import { AmountCard } from "./AmountCard";
import { InvoicePanel, ReprintSheet } from "./InvoicePanel";
import { PaidPanel } from "./PaidPanel";
import { ReceivedCard, RefusedCard, UncertainCard, WaitingCard, type Received } from "./PayStates";
import { MethodTiles, PaySteps } from "./PaySteps";
import { PaySummary } from "./PaySummary";
import { nextShare, payerLabel, splitStale, type SplitDraft } from "./split-logic";
import { SplitPanel } from "./SplitPanel";
import { ShareStrip, SplitShares } from "./SplitShares";
import {
  afterFailure,
  approvalRefused,
  attemptKey,
  collectOf,
  methodNames,
  payStep,
  payView,
  planTender,
  quickTenders,
  tenderOutcome,
  tipApprovalAsked,
  tipNote,
  type Attempt,
} from "./tender-logic";
import { TipApprovalSheet } from "./TipApprovalSheet";
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

/** What a payment needs once it is known to be recorded, kept as it was sent: the order moves under it. */
interface Sent {
  body: TenderRequest;
  /** Cash to hand back. */
  change: number;
  /** The balance when it was sent. */
  due: number;
  /** The share it paid, in a split, and whose it was. */
  share: number | null;
  who: string | null;
}

/** A payment whose answer was lost: the tenders on the order before it, to tell it apart. */
interface Unsettled {
  before: readonly number[];
  sent: Sent;
}

/** What is kept across a reload while a payment's answer is lost (ADR-368): the payment, and the key that replays rather than repeats it. */
interface Held {
  attempt: Attempt | null;
  unsettled: Unsettled;
}

function isHeld(value: unknown): value is Held {
  if (typeof value !== "object" || value === null) return false;
  const v = value as Partial<Held>;
  return typeof v.unsettled === "object" && v.unsettled !== null && Array.isArray(v.unsettled.before) && typeof v.unsettled.sent === "object";
}

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
  // A payment that may have been recorded holds the screen until the order
  // says, across a reload or a lock as well (ADR-368): what was held for
  // this order is taken up again, and asked about below.
  const [held] = useState(() => loadUnsettled(order.id, isHeld));
  const [attempt, setAttempt] = useState<Attempt | null>(held?.attempt ?? null);
  const [unsettled, setUnsettled] = useState<Unsettled | null>(held?.unsettled ?? null);
  const holdUnsettled = (u: Unsettled, kept: Attempt | null) => {
    setUnsettled(u);
    saveUnsettled<Held>(order.id, { attempt: kept, unsettled: u });
  };
  const releaseUnsettled = () => {
    setUnsettled(null);
    clearUnsettled(order.id);
  };
  const [checking, setChecking] = useState(false);
  const [lastChange, setLastChange] = useState<number | null>(null);
  const [splitting, setSplitting] = useState(false);
  const [inUse, setInUse] = useState<SplitInUse | null>(null);
  // The split being made is kept here, so a look at the payment step does not
  // lose it. By item is the design's first mode; the server divides the lines
  // only while nothing is paid.
  const [draft, setDraft] = useState<SplitDraft>(() => ({
    kind: order.totals.net_paid.minor === 0 ? "item" : "evenly",
    ways: 2,
    amounts: [],
    payers: {},
  }));
  // When a card or UPI payment began to be taken, away from the till.
  const [waiting, setWaiting] = useState<number | null>(null);
  const [received, setReceived] = useState<Received | null>(null);
  const [panel, setPanel] = useState<Panel | null>(null);
  const [reprinting, setReprinting] = useState(false);
  // The tip ceiling (ADR-261): null until the venue is read, and the server decides meanwhile.
  const ceiling = useVenue()?.tips_max_percent ?? null;
  // Which payment the server asked a manager for, and the approval given for
  // which: each holds only while the method, the amount and the tip are as they were.
  const [asked, setAsked] = useState<string | null>(null);
  const [approval, setApproval] = useState<{ for: string; override: Override } | null>(null);
  const [approving, setApproving] = useState(false);
  const invoice = order.invoices.at(-1);

  const paid = due <= 0;
  const step = payStep(paid, splitting);
  const stale = inUse !== null && splitStale(inUse.split, inUse.paid, due);
  const share = inUse && inUse.current !== null && !stale ? inUse.split.shares[inUse.current] : undefined;
  const cash = method === "cash";
  // A share is paid whole: card and UPI take exactly it, cash at least it.
  // The whole bill in cash is the balance as the venue rounds cash (ADR-377).
  const target = share ? share.amount.minor : cash && order.totals.cash_due.minor > 0 ? order.totals.cash_due.minor : due;
  const whole = share !== undefined;
  const amountShown = entered === null || (whole && !cash) ? target : cash ? entered : Math.min(entered, target);
  const plan = stale
    ? { amount: 0, change: 0, charge: 0, problem: "Split the bill again before taking payment." }
    : planTender({ method, entered: amountShown, tip, due: target, whole });
  const money = (minor: number): Amount => ({ minor, currency });
  const collect = collectOf(plan, target, tip);
  const thisPayment = `${method}:${plan.amount}:${tip}`;
  const approved = approval?.for === thisPayment ? approval.override : null;
  const note = plan.problem === null ? (tipNote(tip, plan.amount, ceiling) ?? (asked === thisPayment ? "Needs a manager" : null)) : null;
  const needsManager = note !== null && approved === null;
  const view = payView({ paid, received: received !== null, waiting: waiting !== null, failed: error !== null, uncertain: unsettled !== null });

  const choose = (next: PaymentMethod) => {
    setMethod(next);
    setEntered(next === "cash" ? 0 : null);
    setTip(0);
    setReference("");
    setField(next !== "cash" && whole ? "tip" : "amount");
    setError(null);
  };

  const takeSplit = (split: SplitResponse, amountsKeyed: number) => {
    // The cards stay up, now with Collect on each.
    setInUse({ split, amountsKeyed, paid: new Set(), current: 0 });
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

  // Taken up from before a reload: the order is asked at once.
  useEffect(() => {
    if (held) void check(held.unsettled);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- once, as the screen appears
  }, []);

  /** A payment the server has: the bill and the split move on. */
  const recorded = (sent: Sent) => {
    const { body } = sent;
    const byCash = body.method === "cash";
    setAttempt(null);
    releaseUnsettled();
    setWaiting(null);
    // Money that leaves some owed is confirmed here; the paid panel says the rest.
    if (body.amount.minor < sent.due) {
      setReceived({
        method: body.method,
        amount: body.amount,
        tip: body.tip,
        change: money(byCash ? sent.change : 0),
        reference: body.reference ?? "",
        who: sent.who,
        left: money(sent.due - body.amount.minor),
      });
    }
    setApproval(null);
    setAsked(null);
    const paidShare = sent.share;
    if (paidShare !== null) {
      setInUse((u) => {
        if (!u) return u;
        const paidShares = new Set(u.paid).add(paidShare);
        return { ...u, paid: paidShares, current: nextShare(u.split, paidShares) };
      });
    }
    setLastChange(byCash ? sent.change : null);
    setEntered(byCash ? 0 : null);
    setTip(0);
    setReference("");
    setField(!byCash && inUse ? "tip" : "amount");
  };

  /** Reads the order to see whether a payment whose answer was lost is on it. */
  const check = async (u: Unsettled) => {
    setChecking(true);
    try {
      const read = await ticket.reread();
      if (tenderOutcome(u.before, u.sent.body, read) === "recorded") {
        setError(null);
        recorded(u.sent);
      } else {
        // Not there: the failure stands, and another method may be tried.
        releaseUnsettled();
      }
    } catch {
      // Still not reached: the card says so and offers Check again.
    } finally {
      setChecking(false);
    }
  };

  /**
   * Sends the payment, with a manager's approval when one was given. A refusal
   * of the approval itself rejects when the approval sheet sent it, for the
   * sheet to say; anything else is said here, as a payment's failure always
   * is, and an approval refused at Mark as paid is asked for again.
   */
  const send = async (override: Override | null = approved, fromSheet = false) => {
    if (plan.problem || busy || unsettled) return;
    const ref = reference.trim();
    const body: TenderRequest = {
      method,
      amount: money(plan.amount),
      tip: money(tip),
      ...(!cash && ref ? { reference: ref } : {}),
      ...(override ?? {}),
    };
    const next = attemptKey(attempt, body, () => newKey());
    const sent: Sent = {
      body,
      change: cash ? plan.change : 0,
      due,
      share: inUse?.current ?? null,
      who: inUse && inUse.current !== null ? payerLabel(inUse.split.kind, inUse.current, inUse.amountsKeyed) : null,
    };
    const before = order.tenders.map((t) => t.id);
    setAttempt(next);
    setBusy(true);
    setError(null);
    try {
      await tender(body, next.key);
      recorded(sent);
    } catch (err) {
      // A refusal lets the attempt go; one that may have been recorded keeps
      // its key, method and amount until the order is read again.
      const kept = afterFailure(next, err);
      setAttempt(kept);
      if (override && approvalRefused(err)) {
        setApproval(null);
        if (fromSheet) throw err;
        setError("The manager's approval was not accepted. Approve the tip again.");
        return;
      }
      setError(err instanceof Error ? err.message : String(err));
      if (tipApprovalAsked(err)) {
        // The till did not know this tip needed a manager: it does now.
        setAsked(thisPayment);
        setApproving(true);
      }
      if (kept) {
        const u = { before, sent };
        holdUnsettled(u, kept);
        void check(u);
      }
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
  // Cash that covers what is owed: the change is said in the foot at every
  // size, beside the button, so it is never a scroll away (ADR-381).
  const changeDue = typing && !error && plan.problem === null && short === 0 ? money(plan.change) : null;
  // A discount or a note changes the bill: not once it is paid or invoiced.
  const open = !paid && !order.invoiced;

  return (
    <div className="flex min-w-0 flex-1">
      <PaySummary
        order={order}
        title={title}
        // Back is held while a payment's answer is out: leaving would lose
        // the question, and the next Charge would be a second payment.
        busy={busy || unsettled !== null}
        onDiscount={open ? () => setPanel({ kind: "discount", lineId: null }) : null}
        onNote={open ? () => setPanel({ kind: "note" }) : null}
        onBack={onBack}
      />

      <section aria-label="Payment" className="flex min-w-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-4 pb-4 pt-4 short:pb-3 short:pt-3 xl:px-6 xl:pb-5 xl:pt-5 2xl:px-7 2xl:pt-6">
          {/* A paid bill has no step left to take: "Paid in full" heads the invoice instead, and nothing wraps beside the paid panel. */}
          {step !== "complete" && (
            <PaySteps
              step={step}
              disabled={busy || view !== "entry"}
              onPayment={() => setSplitting(false)}
              onSplit={() => {
                setSplitting(true);
                setError(null);
              }}
            />
          )}

          {step === "split" && (
            <div className="mt-5 flex min-h-0 flex-1 flex-col short:mt-3">
              {inUse ? (
                <SplitShares
                  order={order}
                  title={title}
                  split={inUse.split}
                  amountsKeyed={inUse.amountsKeyed}
                  paid={inUse.paid}
                  current={inUse.current}
                  stale={stale}
                  disabled={busy}
                  onCollect={(payer) => {
                    setInUse({ ...inUse, current: payer });
                    setEntered(cash ? 0 : null);
                    setTip(0);
                    setField(cash ? "amount" : "tip");
                    // Another share is another payment: it never reuses a failed one's key.
                    setAttempt(null);
                    setError(null);
                    setSplitting(false);
                  }}
                  onClear={() => {
                    setInUse(null);
                    setEntered(cash ? 0 : null);
                    setTip(0);
                    setField("amount");
                  }}
                  onBack={() => setSplitting(false)}
                />
              ) : (
                <SplitPanel order={order} title={title} draft={draft} onDraft={setDraft} onUse={takeSplit} onBack={() => setSplitting(false)} />
              )}
            </div>
          )}

          {step === "complete" && (
            <div className="flex min-h-0 flex-1 flex-col">
              <InvoicePanel order={order} ticket={ticket} onError={setError} />
            </div>
          )}

          {step === "payment" && view === "waiting" && waiting !== null && (
            <div className="mt-6 short:mt-3">
              <WaitingCard
                method={method}
                charge={money(plan.charge)}
                tip={money(tip)}
                reference={reference}
                since={waiting}
                busy={busy}
                onReference={setReference}
                onCancel={() => setWaiting(null)}
                onPaid={() => (needsManager ? setApproving(true) : void send())}
              />
            </div>
          )}

          {step === "payment" && view === "refused" && (
            <div className="mt-6 short:mt-3">
              <RefusedCard
                method={method}
                reason={error ?? ""}
                busy={busy}
                onOther={() => {
                  setWaiting(null);
                  setError(null);
                }}
                onRetry={() => (needsManager ? setApproving(true) : void send())}
              />
            </div>
          )}

          {step === "payment" && view === "uncertain" && unsettled && (
            <div className="mt-6 short:mt-3">
              <UncertainCard
                method={unsettled.sent.body.method}
                amount={unsettled.sent.body.amount}
                checking={checking}
                onCheck={() => void check(unsettled)}
              />
            </div>
          )}

          {step === "payment" && view === "received" && received && (
            <div className="mt-6 short:mt-3">
              <ReceivedCard
                received={received}
                next={inUse ? "Next share" : "Next payment"}
                onNext={() => {
                  setReceived(null);
                  // Back to the guests' cards, to see who has paid and collect the next.
                  if (inUse) setSplitting(true);
                }}
              />
            </div>
          )}

          {step === "payment" && view === "entry" && (
            <>
              {inUse && (
                <div className="mt-5 short:mt-3">
                  <ShareStrip
                    split={inUse.split}
                    amountsKeyed={inUse.amountsKeyed}
                    paid={inUse.paid}
                    current={inUse.current}
                    stale={stale}
                    disabled={busy}
                    onShares={() => setSplitting(true)}
                  />
                </div>
              )}
              <h2 className="mt-5 font-serif text-xl short:mt-3 2xl:mt-6 2xl:text-total">Select payment method</h2>
              <div className="mt-3 short:mt-2 2xl:mt-4">
                <MethodTiles method={method} disabled={busy} onChoose={choose} />
              </div>

              {/* The pad beside the amount and the tip under both, so an odd sum is keyed with the amount in view; all three side by side from 1536. */}
              <div className="mt-5 grid grid-cols-[minmax(0,1fr)_var(--panel)] items-start gap-4 short:mt-3 2xl:grid-cols-[minmax(0,1fr)_var(--rail)_var(--side)]">
                <AmountCard
                  method={method}
                  target={money(target)}
                  share={whole}
                  amount={money(amountShown)}
                  active={field === "amount"}
                  // The tip comes out of the cash: "exact" is the bill and the tip.
                  quick={cash ? quickTenders(target + tip, currency) : []}
                  change={change}
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
                />
                <div className="col-span-2 2xl:col-span-1">
                  <TipCard
                    bill={money(plan.problem === null ? plan.amount : target)}
                    base={target}
                    tip={money(tip)}
                    collect={money(collect)}
                    cash={cash}
                    note={note}
                    active={field === "tip"}
                    busy={busy}
                    onActivate={() => setField("tip")}
                    onKey={(k) => keyIn("tip", k)}
                    onTip={(minor) => {
                      setTip(minor);
                      setError(null);
                    }}
                  />
                </div>
                {/* The keypad the design leaves to a keyboard: a touch till has none (rule 4). */}
                <div className="col-start-2 row-start-1 rounded-card border border-line bg-raised p-4 short:p-3 2xl:col-start-3">
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

        {step === "payment" && view === "entry" && (
          <div className="flex shrink-0 items-center gap-5 border-t border-line bg-canvas px-4 py-3.5 short:py-2 xl:px-6 2xl:px-7">
            <div aria-live="polite" className="min-w-0 flex-1">
              {changeDue ? (
                <p className="flex flex-wrap items-baseline gap-x-2.5 font-serif text-total leading-tight text-success">
                  Change <Money amount={changeDue} variant="total" tone="success" />
                </p>
              ) : (
                <p
                  role={message?.tone === "error" ? "alert" : undefined}
                  className={message?.tone === "error" ? "text-base text-danger" : "text-base text-fg-muted"}
                >
                  {message?.text}
                  {short > 0 && (
                    <>
                      A part payment: <Money amount={money(short)} /> stays owed.
                    </>
                  )}
                </p>
              )}
            </div>
            <Button
              variant="primary"
              size="charge"
              disabled={busy || plan.problem !== null}
              onClick={() => {
                // A tip over the ceiling is approved first. Cash is in the
                // hand; a card or UPI payment is taken first, then marked paid.
                if (needsManager) setApproving(true);
                else if (cash) void send();
                else {
                  setError(null);
                  setWaiting(Date.now());
                }
              }}
            >
              {busy ? "Sending" : cash ? "Complete payment" : `Collect by ${methodNames[method]}`}
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

      {approving && (
        <TipApprovalSheet
          tip={money(tip)}
          amount={money(plan.amount)}
          ceiling={ceiling}
          action={cash || waiting !== null ? "Approve and record" : "Approve"}
          // Cash, or a card already taken on the machine, goes with the
          // approval now. A card not yet taken carries it to Mark as paid:
          // the manager says yes before the machine is keyed, and the server
          // checks the PIN when the payment is recorded.
          onSubmit={async (override) => {
            if (cash || waiting !== null) return send(override, true);
            setApproval({ for: thisPayment, override });
            setError(null);
            setWaiting(Date.now());
          }}
          onClose={() => setApproving(false)}
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

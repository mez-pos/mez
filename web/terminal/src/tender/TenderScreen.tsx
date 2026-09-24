// The tender screen (docs/design/screens/terminal-tender.md): cash, card or
// UPI against the ticket in front, a tip kept apart from the bill, and the
// amount still due always in view. The invoice is Task 3.
//
// Every figure is the server's but the change due and the part of a cash
// payment that settles the bill, which tender-logic.ts works out. A split
// (Task 2) comes from the server too; each share is then one tender.

import { useState } from "react";
import type { Money as Amount, OrderResponse, PaymentMethod, SplitResponse, TenderPayment } from "../../../shared/api-types";
import { Button, Money, NumPad, applyAmountKey, type NumKey } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { newKey } from "../register/ticket-key";
import type { Ticket } from "../register/use-ticket";
import { nextShare, splitStale } from "./split-logic";
import { SplitPanel } from "./SplitPanel";
import { SplitShares } from "./SplitShares";
import { attemptKey, planTender, quickTenders, type Attempt } from "./tender-logic";
import { TenderSummary, methodNames } from "./TenderSummary";

interface TenderScreenProps {
  order: OrderResponse;
  tender: Ticket["tender"];
  /** Where the ticket is: "Table 4 · 2 covers". */
  label: string;
  onBack: () => void;
}

const methods: { method: PaymentMethod; hint: string }[] = [
  { method: "cash", hint: "Drawer" },
  { method: "card", hint: "Slip reference" },
  { method: "upi", hint: "Transaction ID" },
];

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

export function TenderScreen({ order, tender, label, onBack }: TenderScreenProps) {
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

  const stale = inUse !== null && splitStale(inUse.split, inUse.paid, due);
  const share = inUse && inUse.current !== null && !stale ? inUse.split.shares[inUse.current] : undefined;
  // A share is paid whole: card and UPI take exactly it, cash at least it.
  const target = share ? share.amount.minor : due;
  const whole = share !== undefined;
  const amountShown =
    entered === null || (whole && method !== "cash") ? target : method === "cash" ? entered : Math.min(entered, target);
  const plan = stale
    ? { amount: 0, change: 0, charge: 0, problem: "Split the bill again before taking payment." }
    : planTender({ method, entered: amountShown, tip, due: target, whole });
  const money = (minor: number): Amount => ({ minor, currency });

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
    setEntered(method === "cash" ? 0 : null);
    setField(method === "cash" ? "amount" : "tip");
    setError(null);
  };

  const key = (k: NumKey) => {
    setError(null);
    if (field === "tip") {
      setTip((t) => applyAmountKey(t, k, padCeiling));
      return;
    }
    if (whole && method !== "cash") return;
    // A card amount that is following the balance starts afresh on the first
    // digit, so a part payment is keyed, not the balance backspaced away.
    const from = entered === null && k !== "back" ? 0 : amountShown;
    setEntered(applyAmountKey(from, k, method === "cash" ? padCeiling : target));
  };

  const send = async () => {
    if (plan.problem || busy) return;
    const ref = reference.trim();
    const body: TenderPayment = {
      method,
      amount: money(plan.amount),
      tip: money(tip),
      ...(method !== "cash" && ref ? { reference: ref } : {}),
    };
    const next = attemptKey(attempt, body, () => newKey());
    setAttempt(next);
    setBusy(true);
    setError(null);
    try {
      await tender(body, next.key);
      setAttempt(null);
      if (inUse && inUse.current !== null) {
        const paid = new Set(inUse.paid).add(inUse.current);
        setInUse({ ...inUse, paid, current: nextShare(inUse.split, paid) });
      }
      setLastChange(method === "cash" ? plan.change : null);
      setEntered(method === "cash" ? 0 : null);
      setTip(0);
      setReference("");
      setField(method !== "cash" && inUse ? "tip" : "amount");
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const typing = method === "cash" && amountShown > 0;
  const change = typing ? money(plan.change) : lastChange !== null ? money(lastChange) : null;
  const paid = due <= 0;

  const action = (
    <Button variant="primary" size="charge" disabled={busy || paid || plan.problem !== null} onClick={() => void send()}>
      {busy ? "Sending" : <ActionLabel method={method} amount={money(plan.charge)} />}
    </Button>
  );
  const message = splitting
    ? null
    : error
    ? { text: error, tone: "error" as const }
    : !paid && plan.problem
      ? { text: plan.problem, tone: "hint" as const }
      : null;

  return (
    <div className="flex min-w-0 flex-1">
      <section aria-label="Tender" className="flex min-w-0 flex-1 flex-col overflow-y-auto px-6 py-5">
        {splitting && !paid ? (
          <SplitPanel order={order} onUse={takeSplit} onCancel={() => setSplitting(false)} />
        ) : paid ? (
          <div className="m-auto text-center">
            <p className="font-serif text-total">Paid in full</p>
            <p className="mt-2 text-base text-fg-muted">The invoice and closing the ticket come next.</p>
          </div>
        ) : (
          <>
            {inUse ? (
              <SplitShares
                split={inUse.split}
                amountsKeyed={inUse.amountsKeyed}
                paid={inUse.paid}
                current={inUse.current}
                stale={stale}
                disabled={busy}
                onPick={(payer) => {
                  setInUse({ ...inUse, current: payer });
                  setEntered(method === "cash" ? 0 : null);
                  setError(null);
                }}
                onClear={() => {
                  setInUse(null);
                  setEntered(method === "cash" ? 0 : null);
                  setField("amount");
                }}
              />
            ) : (
              <div className="mb-4 flex justify-end">
                <Button disabled={busy} onClick={() => setSplitting(true)}>
                  Split the bill
                </Button>
              </div>
            )}
            <div role="radiogroup" aria-label="Payment method" className="grid grid-cols-3 gap-tap-gap">
              {methods.map((m) => (
                <button
                  key={m.method}
                  type="button"
                  role="radio"
                  aria-checked={method === m.method}
                  disabled={busy}
                  onClick={() => choose(m.method)}
                  className={cx(
                    "flex h-24 flex-col items-start justify-between rounded border p-3.5 text-left",
                    "outline-offset-2 outline-line-focus focus-visible:outline-2 disabled:opacity-40",
                    method === m.method ? "border-line-focus bg-raised-high" : "border-line bg-raised",
                  )}
                >
                  <span className="text-lg font-semibold">{methodNames[m.method]}</span>
                  <span className="text-sm text-fg-muted">{m.hint}</span>
                </button>
              ))}
            </div>

            <div className="mt-5 grid grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)] gap-5">
              <div className="rounded border border-line bg-chrome p-5">
                <div className="grid grid-cols-2 gap-tap-gap">
                  <AmountField
                    label={method === "cash" ? "Cash handed over" : share ? "This share" : "Amount"}
                    amount={money(amountShown)}
                    active={field === "amount"}
                    disabled={whole && method !== "cash"}
                    onClick={() => setField("amount")}
                  />
                  <AmountField label="Tip" amount={money(tip)} active={field === "tip"} onClick={() => setField("tip")} />
                </div>
                {method === "cash" && (
                  <div className="mt-3 flex flex-wrap gap-tap-gap" aria-label="Quick tender">
                    {quickTenders(target, currency).map((q, i) => (
                      <Button
                        key={q}
                        disabled={busy}
                        onClick={() => {
                          setEntered(q);
                          setField("amount");
                          setError(null);
                        }}
                        className="flex-1"
                      >
                        {i === 0 ? "Exact" : <Money amount={money(q)} />}
                      </Button>
                    ))}
                  </div>
                )}
                <NumPad
                  layout="amount"
                  size="lg"
                  label={field === "tip" ? "Tip" : method === "cash" ? "Cash handed over" : "Amount"}
                  onKey={key}
                  disabled={busy}
                  className="mt-4"
                />
              </div>

              <div className="rounded border border-line bg-chrome p-5">
                {method === "cash" ? (
                  <p className="text-base text-fg-muted">
                    Key what the guest hands over. A tip comes out of the change; it is never part of the bill.
                  </p>
                ) : (
                  <label className="flex flex-col gap-2">
                    <span className="text-meta font-semibold uppercase text-fg-muted">
                      {method === "card" ? "Slip reference" : "UPI transaction ID"}
                    </span>
                    <input
                      value={reference}
                      onChange={(e) => {
                        setReference(e.target.value);
                        setError(null);
                      }}
                      disabled={busy}
                      maxLength={40}
                      autoComplete="off"
                      spellCheck={false}
                      className="min-h-tap rounded border border-line-strong bg-raised-high px-3 font-mono text-fg outline-offset-2 outline-line-focus focus-visible:outline-2"
                    />
                    <span className="text-sm text-fg-faint">Optional. Kept with the payment for reconciling.</span>
                  </label>
                )}
              </div>
            </div>
          </>
        )}
      </section>

      <TenderSummary
        order={order}
        label={label}
        change={change}
        action={paid || splitting ? null : action}
        message={message}
        busy={busy}
        onBack={onBack}
      />
    </div>
  );
}

function ActionLabel({ method, amount }: { method: PaymentMethod; amount: Amount }) {
  const figure = <Money amount={amount} />;
  if (method === "cash") return <>Take {figure} cash</>;
  if (method === "card") return <>Charge {figure} to card</>;
  return <>Record {figure} UPI</>;
}

interface AmountFieldProps {
  label: string;
  amount: Amount;
  active: boolean;
  /** A share by card is its whole amount: nothing to key. */
  disabled?: boolean;
  onClick: () => void;
}

function AmountField({ label, amount, active, disabled = false, onClick }: AmountFieldProps) {
  return (
    <button
      type="button"
      aria-pressed={active}
      disabled={disabled}
      onClick={onClick}
      className={cx(
        "flex min-h-tap flex-col items-stretch gap-1 rounded border bg-canvas px-4 py-3 text-left",
        "outline-offset-2 outline-line-focus focus-visible:outline-2",
        active ? "border-line-focus" : "border-line",
      )}
    >
      <span className="text-meta font-semibold uppercase text-fg-muted">{label}</span>
      <Money amount={amount} className="text-xl" />
    </button>
  );
}

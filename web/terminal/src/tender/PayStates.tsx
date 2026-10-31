// What a payment looks like after it is keyed (D08b): waiting while a card or
// UPI payment is taken, refused, and received.
//
// The design draws a payment gateway: a QR made for the bill, a card terminal
// that answers. The till has neither, and must not need one: a sale finishes
// with no internet. The card machine and the UPI code are the venue's own, so
// the till waits for the cashier to say the money came, and "Mark as paid" is
// what records it. So there is no QR, no terminal name and no "Retry
// terminal" here, and a refusal is the server's, not a bank's.

import { ArrowRightIcon, CheckIcon, CreditCardIcon, RefreshCwIcon, SmartphoneIcon, XIcon } from "lucide-react";
import type { ReactNode } from "react";
import type { Money as Amount, PaymentMethod } from "../../../shared/api-types";
import { Button, Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { elapsedClock, methodNames } from "./tender-logic";
import { stroke } from "../../../shared/ui/stroke";
import { fieldClass } from "../../../shared/ui/field";

const card = "mx-auto w-full max-w-120 rounded-card border p-6 short:p-5";
const eyebrow = "text-meta font-bold uppercase";

interface WaitingCardProps {
  method: PaymentMethod;
  /** What the guest pays in all: the bill's part and the tip. */
  charge: Amount;
  tip: Amount;
  reference: string;
  /** When the wait began, in epoch milliseconds. */
  since: number;
  busy: boolean;
  onReference: (text: string) => void;
  onCancel: () => void;
  onPaid: () => void;
}

export function WaitingCard({ method, charge, tip, reference, since, busy, onReference, onCancel, onPaid }: WaitingCardProps) {
  const now = useClock(1000);
  const byCard = method === "card";
  const Icon = byCard ? CreditCardIcon : SmartphoneIcon;
  return (
    <section aria-label="Waiting for payment" className={cx(card, "flex flex-col border-line bg-raised text-center")}>
      <p className={cx(eyebrow, "text-attention-text")}>
        {methodNames[method]} · {byCard ? "Waiting on the card machine" : "Waiting"}
      </p>
      {/* The guest scans the venue's own UPI code, not this screen: the icon is a phone, not a code (TILL-23). */}
      <h2 className="mt-2.5 font-serif text-xl">{byCard ? "Insert or tap card" : "Guest pays by UPI"}</h2>
      <span
        aria-hidden="true"
        className="mx-auto my-5 flex size-32 items-center justify-center rounded-round border-8 border-chip-success-bg bg-accent-tint text-accent-text short:my-3 short:size-24"
      >
        <Icon size={52} strokeWidth={stroke.large} />
      </span>
      <p>
        <Money amount={charge} variant="total" className="leading-tight" />
      </p>
      <p className="mt-1.5 text-sm text-fg-muted">
        {byCard ? "Key this amount on the card machine." : "The guest scans the venue's UPI code and pays this amount."}
        {tip.minor > 0 && (
          <>
            {" "}
            It has a tip of <Money amount={tip} /> in it.
          </>
        )}
      </p>
      <p className="mt-3 flex items-center justify-center gap-2 text-sm font-semibold text-chip-warning-fg">
        <span aria-hidden="true" className="size-2 rounded-round bg-highlight" />
        Waiting for payment
        <span aria-hidden="true">· {elapsedClock(now - since)}</span>
      </p>

      <label className="mt-5 flex flex-col gap-2 text-left">
        <span className="text-base font-semibold text-fg">{byCard ? "Slip reference" : "UPI transaction ID"}</span>
        <input
          value={reference}
          onChange={(e) => onReference(e.target.value)}
          disabled={busy}
          maxLength={40}
          autoComplete="off"
          spellCheck={false}
          className={cx(fieldClass, "min-h-tap px-4 font-mono text-lg")}
        />
        <span className="text-sm text-fg-muted">Optional. Kept with the payment for reconciling.</span>
      </label>

      <div className="mt-5 flex gap-2.5">
        <Button disabled={busy} onClick={onCancel} className="flex-1">
          Cancel
        </Button>
        <Button variant="primary" disabled={busy} onClick={onPaid} className="flex-1">
          {busy ? "Recording" : "Mark as paid"}
        </Button>
      </div>
    </section>
  );
}

interface RefusedCardProps {
  method: PaymentMethod;
  /** Why, in the server's words or the network's. */
  reason: string;
  busy: boolean;
  onOther: () => void;
  onRetry: () => void;
}

/** The payment was not recorded. Trying again reuses its key, so it cannot be recorded twice. */
export function RefusedCard({ method, reason, busy, onOther, onRetry }: RefusedCardProps) {
  return (
    <section role="alert" aria-label="Payment not recorded" className={cx(card, "border-danger-line bg-danger-tint")}>
      <p className={cx(eyebrow, "text-danger")}>{methodNames[method]} · Not recorded</p>
      <Outcome
        mark={
          <Mark className="bg-chip-danger-bg text-danger">
            <XIcon size={24} strokeWidth={stroke.bold} />
          </Mark>
        }
        title="Payment not recorded"
      >
        {reason} Trying again sends the same payment, so it cannot be recorded twice.
      </Outcome>
      <div className="mt-5 grid gap-2.5">
        <Button variant="primary" disabled={busy} onClick={onOther}>
          Try another method
        </Button>
        <Button disabled={busy} onClick={onRetry} className="border-danger-line">
          {busy ? "Recording" : "Try again"}
        </Button>
      </div>
    </section>
  );
}

interface UncertainCardProps {
  method: PaymentMethod;
  /** What was sent: the bill's part. */
  amount: Amount;
  /** The order is being read again. */
  checking: boolean;
  onCheck: () => void;
}

/**
 * A payment was sent and the answer lost: the server may have recorded it.
 * Nothing else can be taken until the order says whether it did, so the
 * method and the amount stay as they were sent and no other is offered.
 */
export function UncertainCard({ method, amount, checking, onCheck }: UncertainCardProps) {
  return (
    <section role="alert" aria-label="Payment may have been recorded" className={cx(card, "border-chip-warning-border bg-chip-warning-bg")}>
      <p className={cx(eyebrow, "text-chip-warning-fg")}>
        {methodNames[method]} · <Money amount={amount} />
      </p>
      <Outcome
        mark={
          <Mark className="bg-highlight text-highlight-fg">
            <RefreshCwIcon size={22} strokeWidth={stroke.bold} className={checking ? "motion-safe:animate-spin" : undefined} />
          </Mark>
        }
        title="The payment may have been recorded"
      >
        {checking ? "Checking..." : "The server could not be reached to check. Take nothing else for this bill until it answers."}
      </Outcome>
      <Button variant="primary" disabled={checking} onClick={onCheck} className="mt-5 w-full">
        {checking ? "Checking" : "Check again"}
      </Button>
    </section>
  );
}

/** A payment just taken that leaves some of the bill owed. */
export interface Received {
  method: PaymentMethod;
  /** What it settled of the bill. */
  amount: Amount;
  tip: Amount;
  /** Cash to hand back. */
  change: Amount;
  reference: string;
  /** Whose share it was, in a split. */
  who: string | null;
  /** What the bill still owes after it. */
  left: Amount;
}

interface ReceivedCardProps {
  received: Received;
  /** "Next share", or "Next payment". */
  next: string;
  onNext: () => void;
}

export function ReceivedCard({ received, next, onNext }: ReceivedCardProps) {
  const { method, amount, tip, change, reference, who, left } = received;
  const facts = [who ? `${who}'s share` : null, reference ? `${method === "upi" ? "UPI ref" : "Ref"} ${reference}` : null].filter(Boolean);
  return (
    <section aria-label="Payment received" aria-live="polite" className={cx(card, "border-line bg-raised")}>
      <p className={cx(eyebrow, "text-chip-success-fg")}>{methodNames[method]} · Confirmed</p>
      <Outcome
        mark={
          <Mark className="bg-accent text-accent-fg">
            <CheckIcon size={24} strokeWidth={stroke.bold} />
          </Mark>
        }
        title={
          <>
            <Money amount={amount} variant="heading" /> received
          </>
        }
      >
        {facts.join(" · ")}
        {tip.minor > 0 && (
          <span className="block">
            And a tip of <Money amount={tip} />, apart from the bill.
          </span>
        )}
      </Outcome>
      {change.minor > 0 && (
        <div className="mt-4 rounded-card border border-chip-success-border bg-chip-success-bg/50 px-5 py-3.5">
          <p className="text-base text-fg-2">Change to return</p>
          <Money amount={change} variant="total" tone="accent" className="block text-left leading-tight" />
        </div>
      )}
      <p className="mt-4 rounded bg-accent-tint p-3.5 text-sm text-fg">
        <Money amount={left} className="font-bold" /> is still owed on this bill.
      </p>
      <Button variant="attention" onClick={onNext} className="mt-4 w-full">
        {next}
        <ArrowRightIcon aria-hidden="true" size={17} />
      </Button>
    </section>
  );
}

function Mark({ className, children }: { className: string; children: ReactNode }) {
  return (
    <span aria-hidden="true" className={cx("flex size-12 shrink-0 items-center justify-center rounded-round", className)}>
      {children}
    </span>
  );
}

function Outcome({ mark, title, children }: { mark: ReactNode; title: ReactNode; children: ReactNode }) {
  return (
    <div className="mt-4 flex gap-3.5">
      {mark}
      <div className="min-w-0">
        <h2 className="font-serif text-xl">{title}</h2>
        <p className="mt-1 text-base text-fg">{children}</p>
      </div>
    </div>
  );
}

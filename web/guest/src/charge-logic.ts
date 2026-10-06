// Paying online, without React (docs/design/screens/guest-bill.md,
// ADR-222). The phone starts a charge, goes to the gateway's page, and when
// it is back asks the venue how the charge stands. It remembers one thing:
// which charge it started, so it can ask. Whether it was paid is never the
// phone's to say.

import { ApiError, isUnreachable } from "../../shared/api";
import type { ChargeStatus, GuestRoundStatus } from "../../shared/api-types";

/** How often an open charge is asked about while the bill is on screen. */
export const CHARGE_ASK_MS = 4_000;

/** How long the phone remembers a charge it started: longer than a gateway's page lasts. */
export const CHARGE_REMEMBER_MS = 45 * 60 * 1000;

export interface StartedCharge {
  readonly id: string;
  /** When it was started, by the phone's clock. */
  readonly at: number;
  /** The order it pays for, when it is an order's charge and not the bill's (ADR-227). */
  readonly round?: string;
}

export function chargeStorageKey(code: string): string {
  return `mez.charge.${code}`;
}

/** Where the charge of an order held until paid is kept, apart from the bill's. */
export function roundChargeStorageKey(code: string): string {
  return `mez.round-charge.${code}`;
}

/** A charge id as the server draws one. What is in storage is not trusted. */
const CHARGE_ID = /^chg_[0-9A-Z]{4,32}$/;
const ROUND_ID = /^rnd_[0-9A-Z]{4,32}$/;

export function parseStarted(text: string | null, now: number): StartedCharge | null {
  if (!text) return null;
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof raw !== "object" || raw === null) return null;
  const { id, at, round } = raw as { id?: unknown; at?: unknown; round?: unknown };
  if (typeof id !== "string" || !CHARGE_ID.test(id) || typeof at !== "number" || !Number.isFinite(at)) return null;
  if (now - at > CHARGE_REMEMBER_MS || at - now > CHARGE_REMEMBER_MS) return null;
  if (round === undefined) return { id, at };
  return typeof round === "string" && ROUND_ID.test(round) ? { id, at, round } : null;
}

/** The stored charge of this order, or null: another order's is not this one's. */
export function parseStartedFor(text: string | null, now: number, round: string): StartedCharge | null {
  const started = parseStarted(text, now);
  return started !== null && started.round === round ? started : null;
}

/**
 * Whether an address from the server is a page to send a guest's money to:
 * https, and nothing else. The app follows no other.
 */
export function checkoutUrlOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    const u = new URL(url);
    return u.protocol === "https:" && u.username === "" && u.password === "" ? u.href : null;
  } catch {
    return null;
  }
}

export interface ChargeWords {
  readonly tone: "waiting" | "good" | "bad";
  readonly title: string;
  readonly line: string;
}

/** What the bill says of a charge the phone started. */
export function chargeWords(status: ChargeStatus): ChargeWords {
  switch (status) {
    case "open":
      return {
        tone: "waiting",
        title: "Waiting for your payment",
        line: "If you have paid, this will say so in a moment. If you closed the payment page, you can open it again.",
      };
    case "paid":
      return { tone: "good", title: "Payment received", line: "It is on this bill. Thank you." };
    case "failed":
      return { tone: "bad", title: "That payment did not go through", line: "Nothing was taken. Try again, or pay a member of staff." };
    case "unapplied":
      return {
        tone: "bad",
        title: "Your payment was taken, and is not on this bill",
        line: "Please show this to a member of staff. They will put it right.",
      };
  }
}

/**
 * What the order's screen says of the charge the phone started for it. Paid
 * is said only until the order itself reads sent, a moment later.
 */
export function roundChargeWords(status: ChargeStatus): ChargeWords {
  switch (status) {
    case "open":
      return chargeWords("open");
    case "paid":
      return { tone: "good", title: "Payment received", line: "Your order is on its way to the kitchen." };
    case "failed":
      return { tone: "bad", title: "That payment did not go through", line: "Nothing was taken, and nothing was sent to the kitchen. You can try again." };
    case "unapplied":
      return {
        tone: "bad",
        title: "Your payment was taken, and something went wrong",
        line: "Please show this to a member of staff. They will put it right. Do not pay again.",
      };
  }
}

/**
 * Whether an order is held with its money taken: the gateway has the
 * payment and the venue could not use it. The screen stops asking the guest
 * to pay and sends them to staff.
 */
export function paymentStuck(round: GuestRoundStatus, charge: ChargeStatus | undefined): boolean {
  return round === "unpaid" && charge === "unapplied";
}

/** What the order's screen says at its head when the payment is stuck. */
export const stuckWords = {
  title: "There is a problem with your payment",
  line: "Your order has not been sent to the kitchen. Please show this to a member of staff.",
} as const;

/**
 * Whether the order's screen offers paying: the order is still unpaid, and
 * the phone has no charge for it that might yet be, or already was, paid.
 */
export function canPayRound(round: GuestRoundStatus, charge: ChargeStatus | undefined): boolean {
  return round === "unpaid" && (charge === undefined || charge === "failed");
}

/** Why paying for an order could not be started, in words a guest can act on. */
export function roundStartFailure(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case "nothing_to_pay":
        return "This order is already paid for.";
      case "request_not_found":
        return "This order can no longer be paid for. Place it again from the menu.";
      case "request_lapsed":
      case "no_open_shift":
      case "gateway_unavailable":
        return error.message;
    }
    if (isUnreachable(error)) {
      return "Can't reach the kitchen to start the payment. Check your connection and try again.";
    }
  }
  return "Paying could not be started. Ask a member of staff.";
}

/** Whether a charge still has an answer to wait for. */
export function chargeOpen(status: ChargeStatus | undefined): boolean {
  return status === "open";
}

/**
 * Whether a charge the phone started is one to forget: the table gave its
 * page up (ADR-234) and it has not been paid. It holds nothing and is not
 * waited for. Paid after all, it is kept, to say so.
 */
export function chargeGivenUp(charge: { status: ChargeStatus; let_go?: boolean } | undefined): boolean {
  return charge !== undefined && charge.let_go === true && chargeOpen(charge.status);
}

/** Why a payment page could not be given up. Nothing was taken by trying. */
export const letGoFailure = "That could not be done just now. Try again, or pay a member of staff.";

/** Why paying online could not be started, in words a guest can act on. */
export function startFailure(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "nothing_to_pay") return "There is nothing left to pay on this bill.";
    if (error.code === "bill_claimed") return "Others at the table are paying that part of the bill. Look at what is left, and choose again.";
    if (error.code === "bill_moved") return "The bill changed just now. Look at it again, then pay.";
    if (error.code === "gateway_unavailable") return error.message;
    if (isUnreachable(error)) {
      return "Can't reach the kitchen to start the payment. Check your connection, or pay a member of staff.";
    }
  }
  return "Paying online could not be started. Pay a member of staff, or at the counter.";
}

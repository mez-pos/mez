// Paying online, without React (docs/design/screens/guest-bill.md,
// ADR-222). The phone starts a charge, goes to the gateway's page, and when
// it is back asks the venue how the charge stands. It remembers one thing:
// which charge it started, so it can ask. Whether it was paid is never the
// phone's to say.

import { ApiError } from "../../shared/api";
import type { ChargeStatus } from "../../shared/api-types";

/** How often an open charge is asked about while the bill is on screen. */
export const CHARGE_ASK_MS = 4_000;

/** How long the phone remembers a charge it started: longer than a gateway's page lasts. */
export const CHARGE_REMEMBER_MS = 45 * 60 * 1000;

export interface StartedCharge {
  readonly id: string;
  /** When it was started, by the phone's clock. */
  readonly at: number;
}

export function chargeStorageKey(code: string): string {
  return `mez.charge.${code}`;
}

/** A charge id as the server draws one. What is in storage is not trusted. */
const CHARGE_ID = /^chg_[0-9A-Z]{4,32}$/;

export function parseStarted(text: string | null, now: number): StartedCharge | null {
  if (!text) return null;
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return null;
  }
  if (typeof raw !== "object" || raw === null) return null;
  const { id, at } = raw as { id?: unknown; at?: unknown };
  if (typeof id !== "string" || !CHARGE_ID.test(id) || typeof at !== "number" || !Number.isFinite(at)) return null;
  if (now - at > CHARGE_REMEMBER_MS || at - now > CHARGE_REMEMBER_MS) return null;
  return { id, at };
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

/** Whether a charge still has an answer to wait for. */
export function chargeOpen(status: ChargeStatus | undefined): boolean {
  return status === "open";
}

/** Why paying online could not be started, in words a guest can act on. */
export function startFailure(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "nothing_to_pay") return "There is nothing left to pay on this bill.";
    if (error.code === "gateway_unavailable") return error.message;
    if (error.unreachable || (error.status !== null && error.status >= 502 && error.status <= 504)) {
      return "Can't reach the kitchen to start the payment. Check your connection, or pay a member of staff.";
    }
  }
  return "Paying online could not be started. Pay a member of staff, or at the counter.";
}

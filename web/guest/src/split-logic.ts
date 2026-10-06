// Splitting the bill, without React (docs/design/screens/guest-bill.md,
// ADR-232 to ADR-235). A guest pays the rest of the bill, an equal part of
// it, or a figure they type. Every figure shown is the server's, from the
// bill's `split`; the phone only reads what a guest typed.

import type { GuestBillResponse, GuestShareRequest, Money } from "../../shared/api-types";
import { formatMoney, parseAmount } from "../../shared/money";

/** Where the phone keeps the name it gave itself. */
export const PHONE_STORAGE_KEY = "mez.phone";

/** A name the server takes: what is in storage is not trusted. */
const PHONE_NAME = /^[A-Za-z0-9_-]{8,64}$/;

export function phoneNameOf(text: string | null): string | null {
  return text !== null && PHONE_NAME.test(text) ? text : null;
}

/** A new name from random bytes. It says nothing of who holds the phone. */
export function newPhoneName(bytes: Uint8Array): string {
  return "p_" + Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export type SplitWay = "rest" | "even" | "amount";

/** What a guest has chosen to pay. */
export interface SplitChoice {
  readonly way: SplitWay;
  /** Among how many, when it is split evenly. */
  readonly people: number;
  /** The figure as typed, when it is an amount. */
  readonly typed: string;
}

export const startingChoice: SplitChoice = { way: "rest", people: 2, typed: "" };

/** The fewest and the most people the server splits a bill among. */
export function peopleRange(bill: Pick<GuestBillResponse, "split">): { min: number; max: number } | null {
  const parts = bill.split?.parts ?? [];
  const first = parts[0];
  const last = parts[parts.length - 1];
  return first && last ? { min: first.people, max: last.people } : null;
}

/** What paying the rest asks for: what is owed, less what others are paying in shares. */
export function restOf(bill: Pick<GuestBillResponse, "split" | "balance_due">): Money {
  return bill.split?.rest ?? bill.balance_due;
}

/** What the first choice is called: all of the bill, or the rest once some of it is paid or being paid. */
export function restLabel(bill: Pick<GuestBillResponse, "split" | "balance_due" | "total">): string {
  return restOf(bill).minor < bill.total.minor ? "The rest" : "All of it";
}

/** Whether a bill can be paid in parts from this phone: it has somewhere to pay and something it may take. */
export function canSplit(bill: Pick<GuestBillResponse, "split" | "online">, phone: string | null): boolean {
  return bill.online === true && phone !== null && bill.split !== undefined && bill.split.unclaimed.minor > 0 && bill.split.parts.length > 0;
}

export type Asking = { readonly ok: true; readonly amount: Money; readonly request: GuestShareRequest | null } | { readonly ok: false; readonly why: string };

/**
 * What a choice asks for: the figure on the button, and the request that
 * asks for it. The rest is asked with no body. An equal part is asked by
 * how many people, and the server works the figure out again.
 */
export function askingOf(bill: Pick<GuestBillResponse, "split" | "balance_due">, choice: SplitChoice, phone: string | null): Asking {
  const split = bill.split;
  if (choice.way === "rest" || split === undefined || phone === null) {
    const rest = restOf(bill);
    return rest.minor > 0 ? { ok: true, amount: rest, request: null } : { ok: false, why: "Others at the table are paying the rest of this bill." };
  }
  if (choice.way === "even") {
    const part = split.parts.find((p) => p.people === choice.people);
    if (part === undefined || part.amount.minor <= 0) return { ok: false, why: "Others at the table are paying the rest of this bill." };
    return { ok: true, amount: part.amount, request: { phone, people: part.people } };
  }
  const currency = split.unclaimed.currency;
  if (choice.typed.trim() === "") return { ok: false, why: "Type the amount you are paying." };
  const minor = parseAmount(choice.typed, currency);
  if (minor === null || minor <= 0) return { ok: false, why: "That is not an amount. Type it in figures, like 250 or 250.50." };
  if (minor > split.unclaimed.minor) return { ok: false, why: `That is more than the ${formatMoney(split.unclaimed)} left to pay.` };
  return { ok: true, amount: { minor, currency }, request: { phone, amount: { minor, currency } } };
}

/** One more or one fewer person, kept within what the server offers. */
export function stepPeople(bill: Pick<GuestBillResponse, "split">, people: number, by: 1 | -1): number {
  const range = peopleRange(bill);
  if (range === null) return people;
  return Math.min(range.max, Math.max(range.min, people + by));
}

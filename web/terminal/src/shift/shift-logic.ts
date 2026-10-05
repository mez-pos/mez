// The arithmetic and words behind the till's open and close
// (terminal-close.md), apart from the components so they are tested
// without rendering. The only sum here is the counter's, for display: the
// server adds the notes up again and records its own total.

import type { CountRequest, Denomination, Money, ReconciliationResponse } from "../../../shared/api-types";
import { formatMoney } from "../../../shared/money";
import { applyAmountKey, type NumKey } from "../../../shared/ui/numpad-keys";

/**
 * The notes and coins counted, by face value in minor units. A face value
 * shared by a note and a coin (Rs. 10, Rs. 20) is one row: a drawer is
 * counted by value, not by what the money is made of.
 */
const faces: Readonly<Record<string, readonly number[]>> = {
  INR: [50000, 20000, 10000, 5000, 2000, 1000, 500, 200, 100],
  USD: [10000, 5000, 2000, 1000, 500, 100, 25, 10, 5, 1],
  EUR: [50000, 20000, 10000, 5000, 2000, 1000, 500, 200, 100, 50, 20, 10, 5, 2, 1],
  GBP: [5000, 2000, 1000, 500, 200, 100, 50, 20, 10, 5, 2, 1],
};

/** The rows of the counter for a currency, largest first; empty when there is no table for it. */
export function facesFor(currency: string): readonly number[] {
  return faces[currency] ?? [];
}

/** More pieces of one note than any drawer holds; the pad stops there. */
export const maxPieces = 99_999;

/** How many of each face have been counted. A face not in it is none. */
export type Tally = Readonly<Record<number, number>>;

/** The tally after a pad key on one row. */
export function keyCount(tally: Tally, face: number, key: NumKey): Tally {
  return { ...tally, [face]: applyAmountKey(tally[face] ?? 0, key, maxPieces) };
}

/** The row after `face`, or the same row at the bottom. */
export function nextFace(rows: readonly number[], face: number): number {
  const i = rows.indexOf(face);
  return i < 0 || i === rows.length - 1 ? face : (rows[i + 1] ?? face);
}

/** What the tally adds up to, in minor units. */
export function tallyTotal(tally: Tally): number {
  return Object.entries(tally).reduce((sum, [face, count]) => sum + Number(face) * count, 0);
}

/**
 * The request for a count: the tally's rows with anything in them, largest
 * first, and their total. With no denominations for the currency, `typed`
 * is the whole count.
 */
export function countRequest(currency: string, tally: Tally, typed: number): CountRequest {
  const denominations: Denomination[] = facesFor(currency)
    .filter((face) => (tally[face] ?? 0) > 0)
    .map((face) => ({ face, count: tally[face] ?? 0 }));
  const minor = facesFor(currency).length > 0 ? tallyTotal(tally) : typed;
  return denominations.length > 0 ? { total: { minor, currency }, denominations } : { total: { minor, currency } };
}

/** How the drawer came out, in a word, and the tone that goes with it. */
export interface VarianceWords {
  text: string;
  tone: "success" | "danger" | "warning";
}

export function varianceWords(rec: ReconciliationResponse): VarianceWords {
  const by = formatMoney(absolute(rec.variance));
  if (rec.short) return { text: `Short ${by}`, tone: "danger" };
  if (rec.over) return { text: `Over ${by}`, tone: "warning" };
  return { text: "Balanced", tone: "success" };
}

/** Why the reason is owed, or null when it is not. */
export function reasonOwed(rec: ReconciliationResponse, explained: boolean): string | null {
  if (!rec.reason_required || explained) return null;
  return `${formatMoney(absolute(rec.variance))} is more than the ${formatMoney(rec.threshold)} allowed. Say why before closing.`;
}

/**
 * What to say under a variance when no reason is still owed: that it
 * balanced, that it is within what the venue allows, or, once a reason for a
 * larger one is saved, that it is over and explained.
 */
export function toleranceText(rec: ReconciliationResponse, explained: boolean): string {
  if (!rec.short && !rec.over) return "The drawer is what was expected.";
  const allowed = formatMoney(rec.threshold);
  if (rec.reason_required) return `More than the ${allowed} allowed. The reason is saved with the count.`;
  return explained
    ? `Within the ${allowed} allowed. The note is saved with the count.`
    : `Within the ${allowed} allowed. Add a note for the manager if there is one.`;
}

function absolute(m: Money): Money {
  return { ...m, minor: Math.abs(m.minor) };
}

/** The longest reason the server keeps. */
export const maxReason = 500;

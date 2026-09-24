// What the split panel needs besides the server (terminal-tender.md, Task 2).
// The shares themselves always come from POST /orders/{id}/split, which runs
// the domain's split package: nothing here divides money. This turns the
// panel's choices into that request, names the payers, and tells when a
// split made earlier no longer matches the bill.

import type { LineResponse, SplitKind, SplitRequest, SplitResponse } from "../../../shared/api-types";

/** The most payers the server accepts in one split. */
export const maxWays = 20;

/** What the panel has chosen so far. Payers are 0-based; seats 1-based, 0 is the table. */
export interface SplitDraft {
  kind: SplitKind;
  ways: number;
  /** By amount: each payer's amount in minor units, in order. */
  amounts: number[];
  /** By item: line id to the payers sharing it. A line not here is payer 1's. */
  payers: Readonly<Record<number, number[]>>;
  /** By guest: line id to its seat. A line not here is the table's. */
  seats: Readonly<Record<number, number>>;
}

/** The lines that cost something: a voided or comped line needs no payer. */
export function billed(lines: readonly LineResponse[]): LineResponse[] {
  return lines.filter((l) => l.due.minor > 0);
}

/** The request for a draft, or null while there is nothing to ask. */
export function requestFor(d: SplitDraft, lines: readonly LineResponse[], currency: string): SplitRequest | null {
  switch (d.kind) {
    case "evenly":
      return { kind: "evenly", ways: d.ways };
    case "amount":
      if (d.amounts.length === 0) return null;
      return { kind: "amount", amounts: d.amounts.map((minor) => ({ minor, currency })) };
    case "item":
      return {
        kind: "item",
        ways: d.ways,
        items: billed(lines).map((l) => {
          // Payers past the count went when the count came down.
          const kept = (d.payers[l.id] ?? []).filter((p) => p < d.ways);
          return { line_id: l.id, payers: kept.length > 0 ? kept : [0] };
        }),
      };
    case "guest":
      return {
        kind: "guest",
        ways: d.ways,
        seats: billed(lines).map((l) => {
          const seat = d.seats[l.id] ?? 0;
          return { line_id: l.id, guest: seat <= d.ways ? seat : 0 };
        }),
      };
  }
}

/** Adds or removes a payer on a line. A line always keeps at least one. */
export function togglePayer(current: readonly number[], payer: number): number[] {
  if (current.includes(payer)) {
    return current.length > 1 ? current.filter((p) => p !== payer) : [...current];
  }
  return [...current, payer].sort((a, b) => a - b);
}

/**
 * What the till calls a share. By guest, payer i is seat i+1. By amount,
 * the server adds one payer after those keyed for whatever they leave.
 */
export function payerLabel(kind: SplitKind, payer: number, amountsKeyed: number): string {
  if (kind === "guest") return `Guest ${payer + 1}`;
  if (kind === "amount" && payer === amountsKeyed) return "The rest";
  return `Payer ${payer + 1}`;
}

/** The first share not yet paid, or null when every one is. */
export function nextShare(split: SplitResponse, paid: ReadonlySet<number>): number | null {
  return split.shares.find((s) => !paid.has(s.payer))?.payer ?? null;
}

/**
 * True when the shares left to pay no longer add up to what is owed: the
 * bill changed, or money was taken, somewhere other than this split. The
 * split is then wrong and must be made again.
 */
export function splitStale(split: SplitResponse, paid: ReadonlySet<number>, balance: number): boolean {
  const left = split.shares.filter((s) => !paid.has(s.payer)).reduce((sum, s) => sum + s.amount.minor, 0);
  return left !== balance;
}

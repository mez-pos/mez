// What the split step needs besides the server (D08a). The shares themselves
// always come from POST /orders/{id}/split, which runs the domain's split
// package: nothing here divides money. This turns the panel's choices into
// that request, names the guests, says why a split cannot be completed yet,
// and tells when a split made earlier no longer matches the bill.

import type { LineResponse, ShareResponse, SplitKind, SplitRequest, SplitResponse } from "../../../shared/api-types";

/** The most payers the server accepts in one split. */
export const maxWays = 20;

/**
 * The ways the till splits (D08a): by item, equally, custom amounts. The
 * server's by guest is not offered: a line here can have several guests,
 * which is what a seat's shared plate was.
 */
export type TillSplit = Exclude<SplitKind, "guest">;

/** A split's mode, as the till names it. By guest is named for a split made elsewhere. */
export const splitNames: Readonly<Record<SplitKind, string>> = {
  item: "By item",
  evenly: "Equally",
  amount: "Custom amounts",
  guest: "By guest",
};

/** What the panel has chosen so far. Guests are 0-based. */
export interface SplitDraft {
  kind: TillSplit;
  ways: number;
  /** Custom amounts: each guest's amount in minor units, in order. */
  amounts: readonly number[];
  /** By item: line id to the guests sharing it. A line not here is nobody's yet. */
  payers: Readonly<Record<number, readonly number[]>>;
}

/** The lines that cost something: a voided or comped line needs no payer. */
export function billed(lines: readonly LineResponse[]): LineResponse[] {
  return lines.filter((l) => l.due.minor > 0);
}

/** A line's guests. Guests past the count went when the count came down. */
export function payersOf(d: SplitDraft, lineId: number): number[] {
  return (d.payers[lineId] ?? []).filter((p) => p < d.ways);
}

/** The billed lines no guest has yet. */
export function unassigned(d: SplitDraft, lines: readonly LineResponse[]): LineResponse[] {
  return billed(lines).filter((l) => payersOf(d, l.id).length === 0);
}

/** Adds guests to a line. */
export function assign(d: SplitDraft, lineId: number, guests: readonly number[]): SplitDraft {
  const next = [...new Set([...payersOf(d, lineId), ...guests])].sort((a, b) => a - b);
  return { ...d, payers: { ...d.payers, [lineId]: next } };
}

/** Takes a guest off a line. With nobody left, the line is unassigned again. */
export function unassign(d: SplitDraft, lineId: number, guest: number): SplitDraft {
  return { ...d, payers: { ...d.payers, [lineId]: payersOf(d, lineId).filter((p) => p !== guest) } };
}

/**
 * The request for a draft, or null while there is nothing to ask. By item,
 * the server refuses a line with no payer, so the lines nobody has yet are
 * held on one payer past the guests: each guest's share is then the server's
 * figure while the split is still being made, and so is what is left over.
 */
export function requestFor(d: SplitDraft, lines: readonly LineResponse[], currency: string): SplitRequest | null {
  switch (d.kind) {
    case "evenly":
      return { kind: "evenly", ways: d.ways };
    case "amount":
      if (d.amounts.length === 0) return null;
      return { kind: "amount", amounts: d.amounts.map((minor) => ({ minor, currency })) };
    case "item": {
      const loose = unassigned(d, lines).length > 0;
      return {
        kind: "item",
        ways: loose ? d.ways + 1 : d.ways,
        items: billed(lines).map((l) => {
          const kept = payersOf(d, l.id);
          return { line_id: l.id, payers: kept.length > 0 ? kept : [d.ways] };
        }),
      };
    }
  }
}

/** The share holding the lines nobody has yet, when there are any. */
export function looseShare(d: SplitDraft, lines: readonly LineResponse[], split: SplitResponse): ShareResponse | undefined {
  if (d.kind !== "item" || unassigned(d, lines).length === 0) return undefined;
  return split.shares.find((s) => s.payer === d.ways);
}

/**
 * What the till calls a share. By amount, the server adds one payer after
 * those keyed for whatever they leave.
 */
export function payerLabel(kind: SplitKind, payer: number, amountsKeyed: number): string {
  if (kind === "amount" && payer === amountsKeyed) return "The rest";
  return `Guest ${payer + 1}`;
}

/** Why a draft cannot be completed yet, in the till's words; null when it can. */
export function splitBlock(d: SplitDraft, lines: readonly LineResponse[], split: SplitResponse | undefined): string | null {
  if (d.kind === "amount" && d.amounts.length === 0) return "Key what a guest pays, then add them.";
  if (d.kind === "item" && unassigned(d, lines).length > 0) return "Assign every item before completing the split.";
  if (!split) return "Working it out.";
  // A share of nothing cannot be paid, and would never read as paid.
  const empty = split.shares.find((s) => s.amount.minor <= 0);
  if (empty) return `${payerLabel(d.kind, empty.payer, d.amounts.length)} has nothing to pay. Give them an item or take a guest off.`;
  return null;
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

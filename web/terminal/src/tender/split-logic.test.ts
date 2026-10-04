import { describe, expect, it } from "vitest";
import type { LineResponse, SplitResponse } from "../../../shared/api-types";
import { assign, billed, looseShare, nextShare, payerLabel, requestFor, splitBlock, splitStale, unassign, unassigned, type SplitDraft } from "./split-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const line = (id: number, due: number): LineResponse => ({ id, name: `l${id}`, due: inr(due) }) as LineResponse;
const lines = [line(2, 12600), line(3, 0), line(4, 4200)];

const draft = (extra: Partial<SplitDraft>): SplitDraft => ({ kind: "evenly", ways: 3, amounts: [], payers: {}, ...extra });

describe("billed", () => {
  it("keeps only lines that cost something", () => {
    expect(billed(lines).map((l) => l.id)).toEqual([2, 4]);
  });
});

describe("requestFor", () => {
  it("evenly sends the ways", () => {
    expect(requestFor(draft({}), lines, "INR")).toEqual({ kind: "evenly", ways: 3 });
  });
  it("by amount sends each amount, and nothing until there is one", () => {
    expect(requestFor(draft({ kind: "amount", amounts: [] }), lines, "INR")).toBeNull();
    expect(requestFor(draft({ kind: "amount", amounts: [50000, 1000] }), lines, "INR")).toEqual({
      kind: "amount",
      amounts: [inr(50000), inr(1000)],
    });
  });
  it("by item sends each billed line's guests, dropping guests past the count", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [0, 1], 4: [1, 2] } });
    expect(requestFor(d, lines, "INR")).toEqual({
      kind: "item",
      ways: 2,
      items: [
        { line_id: 2, payers: [0, 1] },
        { line_id: 4, payers: [1] },
      ],
    });
  });
  it("by item holds the lines nobody has yet on one more payer, so the server still answers", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [0], 4: [2] } });
    expect(requestFor(d, lines, "INR")).toEqual({
      kind: "item",
      ways: 3,
      items: [
        { line_id: 2, payers: [0] },
        { line_id: 4, payers: [2] },
      ],
    });
  });
});

describe("unassigned", () => {
  it("is the billed lines with no guest left on them", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [1], 4: [5] } });
    expect(unassigned(d, lines).map((l) => l.id)).toEqual([4]);
  });
});

describe("assign and unassign", () => {
  const d = draft({ kind: "item", ways: 3, payers: { 2: [2] } });
  it("adds guests to a line, in order and once each", () => {
    expect(assign(d, 2, [0, 2]).payers[2]).toEqual([0, 2]);
    expect(assign(d, 4, [1]).payers[4]).toEqual([1]);
  });
  it("takes a guest off a line, which may leave it with nobody", () => {
    expect(unassign(assign(d, 2, [0]), 2, 2).payers[2]).toEqual([0]);
    expect(unassign(d, 2, 2).payers[2]).toEqual([]);
  });
  it("leaves the draft it was given alone", () => {
    assign(d, 2, [0]);
    expect(d.payers[2]).toEqual([2]);
  });
});

const shares = (kind: SplitResponse["kind"], amounts: number[]): SplitResponse => ({
  kind,
  total: inr(amounts.reduce((a, b) => a + b, 0)),
  shares: amounts.map((a, payer) => ({ payer, amount: inr(a), line_ids: [] })),
});

describe("looseShare", () => {
  it("is the holding payer's share while lines are unassigned", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [0] } });
    expect(looseShare(d, lines, shares("item", [12600, 0, 4200]))?.amount.minor).toBe(4200);
  });
  it("is nothing once every line has a guest, or for another kind", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [0], 4: [1] } });
    expect(looseShare(d, lines, shares("item", [12600, 4200]))).toBeUndefined();
    expect(looseShare(draft({}), lines, shares("evenly", [5600, 5600, 5600]))).toBeUndefined();
  });
});

describe("splitBlock", () => {
  const all = draft({ kind: "item", ways: 2, payers: { 2: [0], 4: [1] } });
  const cases: [string, SplitDraft, SplitResponse | undefined, string | null][] = [
    [
      "a line nobody has",
      draft({ kind: "item", ways: 2, payers: { 2: [0] } }),
      shares("item", [12600, 0, 4200]),
      "Assign every item before completing the split.",
    ],
    [
      "a guest with nothing",
      draft({ kind: "item", ways: 2, payers: { 2: [0], 4: [0] } }),
      shares("item", [16800, 0]),
      "Guest 2 has nothing to pay. Give them an item or take a guest off.",
    ],
    ["no amount keyed yet", draft({ kind: "amount" }), undefined, "Key what a guest pays, then add them."],
    ["no answer yet", all, undefined, "Working it out."],
    ["ready", all, shares("item", [12600, 4200]), null],
    ["evenly, ready", draft({}), shares("evenly", [5600, 5600, 5600]), null],
  ];
  it.each(cases)("%s", (_n, d, split, want) => {
    expect(splitBlock(d, lines, split)).toBe(want);
  });
});

describe("payerLabel", () => {
  const cases: [string, Parameters<typeof payerLabel>, string][] = [
    ["a guest is counted from one", ["evenly", 0, 0], "Guest 1"],
    ["by item too", ["item", 1, 0], "Guest 2"],
    ["by amount, the payer the server added", ["amount", 2, 2], "The rest"],
    ["by amount, an amount keyed", ["amount", 1, 2], "Guest 2"],
  ];
  it.each(cases)("%s", (_n, args, want) => {
    expect(payerLabel(...args)).toBe(want);
  });
});

const split = shares("evenly", [351, 350, 350]);

describe("nextShare", () => {
  it("is the first share not paid", () => {
    expect(nextShare(split, new Set())).toBe(0);
    expect(nextShare(split, new Set([0, 2]))).toBe(1);
    expect(nextShare(split, new Set([0, 1, 2]))).toBeNull();
  });
});

describe("splitStale", () => {
  const cases: [string, number[], number, boolean][] = [
    ["fresh", [], 1051, false],
    ["after paying a share here", [0], 700, false],
    ["the bill grew on another till", [], 1200, true],
    ["a payment on another till", [0], 500, true],
  ];
  it.each(cases)("%s", (_n, paid, balance, want) => {
    expect(splitStale(split, new Set(paid), balance)).toBe(want);
  });
});

import { describe, expect, it } from "vitest";
import type { LineResponse, SplitResponse } from "../../../shared/api-types";
import { billed, nextShare, payerLabel, requestFor, splitStale, togglePayer, type SplitDraft } from "./split-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const line = (id: number, due: number): LineResponse => ({ id, name: `l${id}`, due: inr(due) }) as LineResponse;
const lines = [line(2, 12600), line(3, 0), line(4, 4200)];

const draft = (extra: Partial<SplitDraft>): SplitDraft => ({ kind: "evenly", ways: 3, amounts: [], payers: {}, seats: {}, ...extra });

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
  it("by item gives every billed line a payer, the first by default, dropping payers past the count", () => {
    const d = draft({ kind: "item", ways: 2, payers: { 2: [0, 1], 4: [2] } });
    expect(requestFor(d, lines, "INR")).toEqual({
      kind: "item",
      ways: 2,
      items: [
        { line_id: 2, payers: [0, 1] },
        { line_id: 4, payers: [0] },
      ],
    });
  });
  it("by guest seats every billed line, the table by default, and a seat past the count goes to the table", () => {
    const d = draft({ kind: "guest", ways: 2, seats: { 2: 3, 4: 1 } });
    expect(requestFor(d, lines, "INR")).toEqual({
      kind: "guest",
      ways: 2,
      seats: [
        { line_id: 2, guest: 0 },
        { line_id: 4, guest: 1 },
      ],
    });
  });
});

describe("togglePayer", () => {
  const cases: [string, number[], number, number[]][] = [
    ["adds a payer, in order", [2], 0, [0, 2]],
    ["removes one", [0, 2], 2, [0]],
    ["never leaves a line with nobody", [1], 1, [1]],
  ];
  it.each(cases)("%s", (_n, current, p, want) => {
    expect(togglePayer(current, p)).toEqual(want);
  });
});

describe("payerLabel", () => {
  const cases: [string, Parameters<typeof payerLabel>, string][] = [
    ["a payer is counted from one", ["evenly", 0, 0], "Payer 1"],
    ["a guest is their seat", ["guest", 1, 0], "Guest 2"],
    ["by amount, the payer the server added", ["amount", 2, 2], "The rest"],
    ["by amount, an amount keyed", ["amount", 1, 2], "Payer 2"],
  ];
  it.each(cases)("%s", (_n, args, want) => {
    expect(payerLabel(...args)).toBe(want);
  });
});

const split: SplitResponse = {
  kind: "evenly",
  total: inr(1051),
  shares: [
    { payer: 0, amount: inr(351), line_ids: [] },
    { payer: 1, amount: inr(350), line_ids: [] },
    { payer: 2, amount: inr(350), line_ids: [] },
  ],
};

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

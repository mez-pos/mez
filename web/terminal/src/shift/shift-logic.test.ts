import { describe, expect, it } from "vitest";
import type { ReconciliationResponse } from "../../../shared/api-types";
import { countRequest, facesFor, keyCount, maxPieces, nextFace, reasonOwed, tallyTotal, varianceWords } from "./shift-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const rec = (extra: Partial<ReconciliationResponse>): ReconciliationResponse => ({
  expected: inr(252950),
  counted: inr(252950),
  variance: inr(0),
  threshold: inr(0),
  over: false,
  short: false,
  reason_required: false,
  counted_by: 1,
  counted_at: "2026-09-28T20:00:00Z",
  ...extra,
});

describe("facesFor", () => {
  it("lists a currency's notes and coins largest first, each value once", () => {
    for (const currency of ["INR", "USD", "EUR", "GBP"]) {
      const faces = facesFor(currency);
      expect(faces.length).toBeGreaterThan(0);
      expect([...faces].sort((a, b) => b - a)).toEqual(faces);
      expect(new Set(faces).size).toBe(faces.length);
    }
  });
  it("starts the rupee drawer at Rs. 500 and ends at Rs. 1", () => {
    expect(facesFor("INR").at(0)).toBe(50000);
    expect(facesFor("INR").at(-1)).toBe(100);
  });
  it("has no table for a currency it does not know", () => {
    expect(facesFor("JPY")).toEqual([]);
  });
});

describe("keyCount", () => {
  it("types a count into one row and leaves the others", () => {
    let tally = keyCount({ 50000: 2 }, 1000, "4");
    tally = keyCount(tally, 1000, "7");
    expect(tally).toEqual({ 50000: 2, 1000: 47 });
  });
  it("takes a digit off and clears", () => {
    expect(keyCount({ 1000: 47 }, 1000, "back")).toEqual({ 1000: 4 });
    expect(keyCount({ 1000: 47 }, 1000, "clear")).toEqual({ 1000: 0 });
  });
  it("stops at more than any drawer holds", () => {
    expect(keyCount({ 1000: maxPieces }, 1000, "9")).toEqual({ 1000: maxPieces });
  });
  it("does not change the tally it was given", () => {
    const tally = { 1000: 4 };
    keyCount(tally, 1000, "7");
    expect(tally).toEqual({ 1000: 4 });
  });
});

describe("nextFace", () => {
  const rows = [50000, 20000, 10000];
  it("moves down a row, and stays on the last", () => {
    expect(nextFace(rows, 50000)).toBe(20000);
    expect(nextFace(rows, 10000)).toBe(10000);
    expect(nextFace(rows, 7)).toBe(7);
  });
});

describe("tallyTotal and countRequest", () => {
  it("adds the drawer up and sends only the rows counted", () => {
    const tally = { 50000: 5, 2000: 1, 1000: 0 };
    expect(tallyTotal(tally)).toBe(252000);
    expect(countRequest("INR", tally, 0)).toEqual({
      total: inr(252000),
      denominations: [
        { face: 50000, count: 5 },
        { face: 2000, count: 1 },
      ],
    });
  });
  it("sends an empty drawer as nothing, with no rows", () => {
    expect(countRequest("INR", {}, 0)).toEqual({ total: inr(0) });
  });
  it("sends the typed total for a currency with no table", () => {
    expect(countRequest("JPY", { 1000: 3 }, 5000)).toEqual({ total: { minor: 5000, currency: "JPY" } });
  });
});

describe("varianceWords", () => {
  it("says balanced, short or over, with a tone, never colour alone", () => {
    expect(varianceWords(rec({}))).toEqual({ text: "Balanced", tone: "success" });
    const short = varianceWords(rec({ variance: inr(-950), short: true }));
    expect(short.tone).toBe("danger");
    expect(short.text).toMatch(/^Short .*9\.50$/);
    const over = varianceWords(rec({ variance: inr(500), over: true }));
    expect(over.tone).toBe("warning");
    expect(over.text).toMatch(/^Over .*5\.00$/);
  });
});

describe("reasonOwed", () => {
  it("asks for a reason past the threshold, until one is given", () => {
    const past = rec({ variance: inr(-950), short: true, reason_required: true });
    expect(reasonOwed(past, false)).toMatch(/9\.50 is more than the .*0\.00 allowed/);
    expect(reasonOwed(past, true)).toBeNull();
  });
  it("asks for none within it", () => {
    expect(reasonOwed(rec({ variance: inr(-100), short: true, threshold: inr(500) }), false)).toBeNull();
  });
});

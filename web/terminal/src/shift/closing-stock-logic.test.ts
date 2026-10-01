import { describe, expect, it } from "vitest";
import type { ClosingStockSheet } from "../../../shared/api-types";
import { stockCountOwed, toCounts } from "./closing-stock-logic";

const items = [
  { id: 1, name: "Beans", unit: "g" as const },
  { id: 2, name: "Milk", unit: "ml" as const },
];

describe("toCounts", () => {
  it("sends every item's count in thousandths", () => {
    expect(toCounts(items, { 1: "4800", 2: "2000.5" })).toEqual({
      counts: [
        { stock_item_id: 1, qty: 4_800_000 },
        { stock_item_id: 2, qty: 2_000_500 },
      ],
    });
  });
  it("accepts a count of nothing", () => {
    expect(toCounts(items, { 1: "0", 2: "0" })).toMatchObject({ counts: [{ qty: 0 }, { qty: 0 }] });
  });
  it("names an item left uncounted, or counted in words", () => {
    expect(toCounts(items, { 1: "4800" })).toEqual({ problem: { itemId: 2, message: "Count the milk too." } });
    expect(toCounts(items, { 1: "lots", 2: "1" })).toMatchObject({ problem: { itemId: 1, message: /250/ } });
    expect(toCounts(items, { 1: "-5", 2: "1" })).toMatchObject({ problem: { itemId: 1 } });
  });
});

describe("stockCountOwed", () => {
  const sheet = (over: Partial<ClosingStockSheet>): ClosingStockSheet => ({ done: false, items: [], results: [], ...over });
  it("is owed while a list waits to be counted", () => {
    expect(stockCountOwed(sheet({ items }))).toBe(true);
  });
  it("is not owed once counted, with nothing to count, or before the sheet is read", () => {
    expect(stockCountOwed(sheet({ done: true }))).toBe(false);
    expect(stockCountOwed(sheet({}))).toBe(false);
    expect(stockCountOwed(undefined)).toBe(false);
  });
});

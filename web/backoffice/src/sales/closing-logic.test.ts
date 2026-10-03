import { describe, expect, it } from "vitest";
import type { ClosingStockLine, ClosingStockReport, Money } from "../../../shared/api-types";
import { closingCsvPath, closingPath, closingRows, countsIn, uncostedText, varianceWords } from "./closing-logic";

const inr = (minor: number): Money => ({ minor, currency: "INR" });
const line = (id: number, name: string, over: Partial<ClosingStockLine> = {}): ClosingStockLine => ({
  stock_item_id: id,
  name,
  unit: "ml",
  expected: 5_000_000,
  counted: 4_500_000,
  variance: -500_000,
  ...over,
});

// Two nights: milk costed, beans not.
const report: ClosingStockReport = {
  from: "2026-09-29",
  to: "2026-09-30",
  currency: "INR",
  shifts: [
    {
      shift_id: "shf_A",
      trading_day: "2026-09-29",
      lines: [line(2, "Beans", { unit: "g" }), line(1, "Milk", { cost: inr(6000), value: inr(-3000) })],
      value: inr(-3000),
      uncosted: 1,
    },
    {
      shift_id: "shf_B",
      trading_day: "2026-09-30",
      lines: [line(2, "Beans", { unit: "g" }), line(1, "Milk", { cost: inr(6400), value: inr(640) })],
      value: inr(640),
      uncosted: 1,
    },
  ],
  items: [],
  value: inr(-2360),
  uncosted: 2,
};

describe("the paths", () => {
  const range = { from: "2026-09-27", to: "2026-10-03" };
  it("reads the report and its file", () => {
    expect(closingPath(range)).toBe("/api/v1/reports/closing-stock?from=2026-09-27&to=2026-10-03");
    expect(closingCsvPath(range)).toBe("/api/v1/reports/closing-stock/csv?from=2026-09-27&to=2026-10-03");
  });
});

describe("the counts as rows", () => {
  it("is a row per item per shift, in the server's order, each with its day", () => {
    const rows = closingRows(report);
    expect(rows.map((r) => `${r.tradingDay} ${r.line.name}`)).toEqual(["2026-09-29 Beans", "2026-09-29 Milk", "2026-09-30 Beans", "2026-09-30 Milk"]);
    expect(new Set(rows.map((r) => r.key)).size).toBe(4);
    expect(countsIn(report)).toBe(4);
  });
  it("is nothing for a range with no counts", () => {
    const quiet = { ...report, shifts: [] };
    expect(closingRows(quiet)).toEqual([]);
    expect(countsIn(quiet)).toBe(0);
  });
});

describe("a variance in words", () => {
  const tests: { name: string; value: number; counts: number; uncosted: number; text: RegExp; tone: string }[] = [
    { name: "short", value: -2360, counts: 4, uncosted: 2, text: /^Short by .*23\.60$/, tone: "danger" },
    { name: "over", value: 640, counts: 2, uncosted: 0, text: /^Over by .*6\.40$/, tone: "warning" },
    { name: "as expected", value: 0, counts: 2, uncosted: 0, text: /^Counted as expected$/, tone: "success" },
    { name: "as expected with one not costed", value: 0, counts: 2, uncosted: 1, text: /^Counted as expected$/, tone: "success" },
    { name: "nothing costed", value: 0, counts: 2, uncosted: 2, text: /^No cost on what was counted$/, tone: "muted" },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      const got = varianceWords(inr(tt.value), tt.counts, tt.uncosted);
      expect(got.text).toMatch(tt.text);
      expect(got.tone).toBe(tt.tone);
    });
  }
  it("never shows a minus sign: the word says which way", () => {
    expect(varianceWords(inr(-2360), 4, 0).text).not.toMatch(/-|−/);
  });
});

describe("what a total leaves out", () => {
  it("says how many counts had no cost", () => {
    expect(uncostedText(0)).toBe("");
    expect(uncostedText(1)).toBe("1 count had no cost and is not in this.");
    expect(uncostedText(2)).toBe("2 counts had no cost and are not in this.");
  });
});

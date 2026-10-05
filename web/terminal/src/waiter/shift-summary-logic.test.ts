import { describe, expect, it } from "vitest";
import type { MyShiftResponse } from "../../../shared/api-types";
import { onShiftText, shiftSpan, tipBars } from "./shift-summary-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });
const mine = (extra: Partial<MyShiftResponse> = {}): MyShiftResponse => ({
  shift_id: 1,
  opened_at: "2026-09-19T10:30:00Z",
  orders: 0,
  tables: 0,
  sales: inr(0),
  tips: inr(0),
  tips_by_table: [],
  ...extra,
});
const at = (iso: string) => Date.parse(iso);

describe("shiftSpan", () => {
  it("is the day and the hours from the shift's opening until now, in the venue's zone", () => {
    expect(shiftSpan("2026-09-19T10:30:00Z", at("2026-09-19T18:00:00Z"), "Asia/Kolkata")).toBe("Sat, 19 Sep 2026 · 4:00 PM – 11:30 PM");
  });
});

describe("onShiftText", () => {
  it("counts from the first order they touched", () => {
    expect(onShiftText(mine({ first_at: "2026-09-19T10:34:00Z" }), at("2026-09-19T18:00:00Z"))).toBe("On shift · 7h 26m");
  });
  it("counts from the shift's opening for someone who has done nothing yet", () => {
    expect(onShiftText(mine(), at("2026-09-19T10:50:00Z"))).toBe("On shift · 20m");
  });
});

describe("tipBars", () => {
  const names = new Map([
    [4, "T4"],
    [7, "T7"],
  ]);
  it("names each table and draws its bar against the largest", () => {
    const bars = tipBars(
      [
        { table_id: 4, tips: inr(38000) },
        { table_id: 7, tips: inr(19000) },
        { table_id: 0, tips: inr(9500) },
      ],
      names,
    );
    expect(bars.map((b) => [b.label, b.percent])).toEqual([
      ["T4", 100],
      ["T7", 50],
      ["No table", 25],
    ]);
  });
  it("names a table gone from the floor by its number", () => {
    expect(tipBars([{ table_id: 12, tips: inr(100) }], names)[0]?.label).toBe("Table 12");
  });
  it("draws no bar for tips handed back", () => {
    const bars = tipBars(
      [
        { table_id: 4, tips: inr(1000) },
        { table_id: 7, tips: inr(-500) },
      ],
      names,
    );
    expect(bars.map((b) => b.percent)).toEqual([100, 0]);
  });
  it("is empty with no tips", () => {
    expect(tipBars([], names)).toEqual([]);
  });
});

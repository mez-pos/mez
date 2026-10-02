import { describe, expect, it } from "vitest";
import type { Money, SalesPoint } from "../../../shared/api-types";
import { averageBill, barHeights, csvPath, daysIn, everyFor, periodText, quickRange, quickRangeOf, quickRanges, salesPath, shares } from "./sales-logic";

const inr = (minor: number): Money => ({ minor, currency: "INR" });
const point = (start: string, end: string, total: number): SalesPoint => ({
  start,
  end,
  sales: { bills: 0, gross: inr(0), discounts: inr(0), net: inr(0), taxable: inr(0), tax: inr(0), total: inr(total), net_of_credits: inr(0) },
});

// Noon on Saturday 3 October 2026.
const now = new Date(2026, 9, 3, 12, 0);

describe("quick ranges", () => {
  const tests: { id: (typeof quickRanges)[number]["id"]; want: { from: string; to: string } }[] = [
    { id: "today", want: { from: "2026-10-03", to: "2026-10-03" } },
    { id: "week", want: { from: "2026-09-27", to: "2026-10-03" } },
    { id: "month", want: { from: "2026-09-04", to: "2026-10-03" } },
    { id: "this_month", want: { from: "2026-10-01", to: "2026-10-03" } },
  ];
  for (const tt of tests) {
    it(`${tt.id} ends today`, () => {
      expect(quickRange(tt.id, now)).toEqual(tt.want);
      expect(quickRangeOf(tt.want, now)).toBe(tt.id);
    });
  }
  it("counts the small hours as the day before", () => {
    expect(quickRange("today", new Date(2026, 9, 1, 2, 0))).toEqual({ from: "2026-09-30", to: "2026-09-30" });
    expect(quickRange("this_month", new Date(2026, 9, 1, 2, 0))).toEqual({ from: "2026-09-01", to: "2026-09-30" });
  });
  it("is none of them for a range typed by hand", () => {
    expect(quickRangeOf({ from: "2026-08-01", to: "2026-08-15" }, now)).toBeNull();
  });
});

describe("how a range is cut", () => {
  const tests: { name: string; from: string; to: string; days: number; every: string }[] = [
    { name: "one day", from: "2026-10-03", to: "2026-10-03", days: 1, every: "day" },
    { name: "a month of 31", from: "2026-10-01", to: "2026-10-31", days: 31, every: "day" },
    { name: "32 days", from: "2026-10-01", to: "2026-11-01", days: 32, every: "week" },
    { name: "26 weeks", from: "2026-01-01", to: "2026-07-01", days: 182, every: "week" },
    { name: "a day over 26 weeks", from: "2026-01-01", to: "2026-07-02", days: 183, every: "month" },
    { name: "a year across a clock change", from: "2025-10-04", to: "2026-10-03", days: 365, every: "month" },
    { name: "backwards", from: "2026-10-03", to: "2026-10-01", days: 0, every: "day" },
    { name: "not a day", from: "", to: "2026-10-01", days: 0, every: "day" },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(daysIn(tt)).toBe(tt.days);
      expect(everyFor(tt)).toBe(tt.every);
    });
  }
});

describe("the paths", () => {
  const range = { from: "2026-09-27", to: "2026-10-03" };
  it("reads the reports", () => {
    expect(salesPath(range, "day")).toBe("/api/v1/reports/sales?from=2026-09-27&to=2026-10-03&every=day");
  });
  it("names the sheet to download", () => {
    expect(csvPath(range, "week", "over_time")).toBe("/api/v1/reports/sales/csv?from=2026-09-27&to=2026-10-03&every=week&report=over_time");
  });
});

describe("periods", () => {
  it("words a day and a longer period", () => {
    expect(periodText({ start: "2026-10-01", end: "2026-10-01" })).toMatch(/^1 Oct$/);
    expect(periodText({ start: "2026-09-28", end: "2026-10-04" })).toMatch(/^28 Sep\w* to 4 Oct$/);
  });
});

describe("bars", () => {
  it("are a share of the tallest", () => {
    expect(barHeights([point("a", "a", 20000), point("b", "b", 0), point("c", "c", 5000)])).toEqual([100, 0, 25]);
  });
  it("show a small period as a sliver, not as nothing", () => {
    expect(barHeights([point("a", "a", 1000000), point("b", "b", 1)])).toEqual([100, 1]);
  });
  it("are flat for a quiet range and for none", () => {
    expect(barHeights([point("a", "a", 0), point("b", "b", 0)])).toEqual([0, 0]);
    expect(barHeights([])).toEqual([]);
  });
});

describe("shares", () => {
  const tests: { name: string; minors: number[]; want: number[] | null }[] = [
    { name: "split three ways", minors: [5000, 3000, 2000], want: [50, 30, 20] },
    { name: "one row is all of it", minors: [4200], want: [100] },
    { name: "a zero row", minors: [1000, 0], want: [100, 0] },
    { name: "nothing to divide", minors: [0, 0], want: null },
    { name: "no rows", minors: [], want: null },
    { name: "a method that refunded more than it took", minors: [5000, -1000], want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(shares(tt.minors)).toEqual(tt.want);
    });
  }
});

describe("the average bill", () => {
  it("is the total over the bills, to the nearest minor unit", () => {
    expect(averageBill(inr(36000), 3)).toEqual(inr(12000));
    expect(averageBill(inr(10000), 3)).toEqual(inr(3333));
    expect(averageBill(inr(20000), 3)).toEqual(inr(6667));
  });
  it("is nothing with no bills", () => {
    expect(averageBill(inr(0), 0)).toBeNull();
  });
});

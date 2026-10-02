import { describe, expect, it } from "vitest";
import type { Group, Money, SalesHour, SalesPoint } from "../../../shared/api-types";
import type { Sort } from "../../../shared/ui/table-state";
import {
  analysisCsvPath,
  analysisPath,
  averageBill,
  barHeights,
  cellText,
  csvPath,
  daysIn,
  defaultGroupSort,
  everyFor,
  heatGrid,
  hourText,
  periodText,
  quickRange,
  quickRangeOf,
  quickRanges,
  salesPath,
  shares,
  sortGroups,
  tint,
  weekdayName,
} from "./sales-logic";

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

const group = (name: string, qty: number, total: number): Group => ({
  key: name,
  name,
  qty,
  gross: inr(total),
  discounts: inr(0),
  net: inr(total),
  tax: inr(0),
  total: inr(total),
});
const cell = (weekday: number, hour: number, bills: number, total: number): SalesHour => ({ weekday, hour, bills, total: inr(total) });

describe("sorting a table of groups", () => {
  const rows = [group("Dosa", 5, 60000), group("Chai", 9, 27000), group("Vada", 5, 25000)];
  const names = (sort: Sort) => sortGroups(rows, sort).map((g) => g.name);
  const tests: { name: string; sort: Sort; want: string[] }[] = [
    { name: "opens by total, largest first", sort: defaultGroupSort, want: ["Dosa", "Chai", "Vada"] },
    { name: "by total, smallest first", sort: { column: "total", direction: "asc" }, want: ["Vada", "Chai", "Dosa"] },
    { name: "by name", sort: { column: "name", direction: "asc" }, want: ["Chai", "Dosa", "Vada"] },
    { name: "by quantity, ties as the server gave them", sort: { column: "qty", direction: "desc" }, want: ["Chai", "Dosa", "Vada"] },
    { name: "a column it does not sort by falls back to total", sort: { column: "tax", direction: "desc" }, want: ["Dosa", "Chai", "Vada"] },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(names(tt.sort)).toEqual(tt.want);
    });
  }
  it("leaves the server's rows as they were", () => {
    sortGroups(rows, { column: "name", direction: "asc" });
    expect(rows.map((g) => g.name)).toEqual(["Dosa", "Chai", "Vada"]);
  });
});

describe("the heatmap", () => {
  it("runs the hours as the trading day does, the small hours last", () => {
    // Monday lunch, Friday evening, and 01:00 of Friday night's trade.
    const g = heatGrid([cell(1, 13, 2, 42000), cell(5, 22, 1, 20000), cell(5, 1, 3, 63000)]);
    expect(g.hours).toEqual([13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 0, 1]);
    expect(g.rows.map((r) => r.weekday)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    expect(g.rows[0]?.cells[0]?.bills).toBe(2);
    expect(g.rows[4]?.cells[9]?.bills).toBe(1);
    expect(g.rows[4]?.cells[12]?.bills).toBe(3);
    expect(g.rows[1]?.cells.every((c) => c === null)).toBe(true);
    expect(g.busiest).toEqual(cell(5, 1, 3, 63000));
  });
  it("starts at 04:00 and ends at 03:00 at the widest", () => {
    const g = heatGrid([cell(3, 4, 1, 100), cell(3, 3, 1, 100)]);
    expect(g.hours).toHaveLength(24);
    expect(g.hours[0]).toBe(4);
    expect(g.hours[23]).toBe(3);
  });
  it("is one column for one hour", () => {
    expect(heatGrid([cell(7, 9, 1, 100)]).hours).toEqual([9]);
  });
  it("keeps the earliest of two cells that tie as the busiest", () => {
    expect(heatGrid([cell(2, 10, 1, 500), cell(1, 10, 1, 500)]).busiest?.weekday).toBe(1);
  });
  it("is nothing with no bills", () => {
    expect(heatGrid([])).toEqual({ hours: [], rows: [], busiest: null });
    expect(heatGrid([cell(1, 9, 0, 0)])).toEqual({ hours: [], rows: [], busiest: null });
  });
  it("tints in four steps", () => {
    const tests: [number, number, number][] = [
      [1, 100, 0.25],
      [25, 100, 0.25],
      [26, 100, 0.5],
      [50, 100, 0.5],
      [75, 100, 0.75],
      [76, 100, 1],
      [100, 100, 1],
      [0, 100, 0.25],
      [50, 0, 0.25],
    ];
    for (const [total, top, want] of tests) expect(tint(total, top)).toBe(want);
  });
  it("words a weekday, an hour and a cell", () => {
    expect(weekdayName(1)).toBe("Monday");
    expect(weekdayName(7)).toBe("Sunday");
    expect(hourText(4)).toBe("04:00");
    expect(cellText(cell(5, 13, 3, 63000))).toMatch(/^Friday 13:00, 3 bills, .*630\.00$/);
    expect(cellText(cell(7, 0, 1, 10000))).toMatch(/^Sunday 00:00, 1 bill, /);
  });
});

describe("the corrections' paths", () => {
  const range = { from: "2026-09-27", to: "2026-10-03" };
  it("reads the analysis and names the grouping to download", () => {
    expect(analysisPath(range)).toBe("/api/v1/audit/analysis?from=2026-09-27&to=2026-10-03");
    expect(analysisCsvPath(range, "authorised_by")).toBe("/api/v1/audit/analysis/csv?from=2026-09-27&to=2026-10-03&by=authorised_by");
  });
});

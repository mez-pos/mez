import { describe, expect, it } from "vitest";
import type { Group, SalesPoint } from "../../../shared/api-types";
import {
  beforeText,
  byBills,
  byTotal,
  insightText,
  namedBars,
  othersKey,
  previousRange,
  rangeText,
  ringGradient,
  serviceName,
  slices,
  trend,
  usable,
} from "./overview-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

function group(key: string, qty: number, total: number, name = key): Group {
  const z = inr(0);
  return { key, name, qty, gross: z, discounts: z, net: z, tax: z, total: inr(total) };
}

function point(start: string, total: number, by: Group[]): SalesPoint {
  const z = inr(0);
  return {
    start,
    end: start,
    sales: {
      bills: by.reduce((n, g) => n + g.qty, 0),
      gross: z,
      discounts: z,
      net: z,
      taxable: z,
      tax: z,
      total: inr(total),
      net_of_credits: inr(total),
      covers: 0,
    },
    by_service: by,
  };
}

describe("previousRange", () => {
  const tests: { name: string; from: string; to: string; want: { from: string; to: string } | null }[] = [
    { name: "a week", from: "2026-09-29", to: "2026-10-05", want: { from: "2026-09-22", to: "2026-09-28" } },
    { name: "one day", from: "2026-10-05", to: "2026-10-05", want: { from: "2026-10-04", to: "2026-10-04" } },
    { name: "across a month and a year", from: "2026-01-01", to: "2026-01-31", want: { from: "2025-12-01", to: "2025-12-31" } },
    { name: "over a leap day", from: "2028-03-01", to: "2028-03-02", want: { from: "2028-02-28", to: "2028-02-29" } },
    { name: "a range that runs backwards", from: "2026-10-05", to: "2026-10-01", want: null },
    { name: "no range", from: "", to: "", want: null },
    { name: "a year half typed", from: "0002-10-05", to: "2026-10-05", want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(previousRange({ from: tt.from, to: tt.to })).toEqual(tt.want));
  }
});

describe("usable", () => {
  const tests: [string, string, boolean][] = [
    ["2026-10-01", "2026-10-05", true],
    ["2026-10-05", "2026-10-05", true],
    ["2026-10-05", "2026-10-01", false],
    ["", "2026-10-05", false],
    ["0002-10-05", "2026-10-05", false],
  ];
  for (const [from, to, want] of tests) {
    it(`${from || "nothing"} to ${to} is ${want ? "a range" : "not one"}`, () => expect(usable({ from, to })).toBe(want));
  }
});

describe("the range in words", () => {
  it("names both ends and the year once", () => {
    expect(rangeText({ from: "2026-10-01", to: "2026-10-05" })).toBe("1 Oct – 5 Oct 2026");
  });
  it("names one day once", () => {
    expect(rangeText({ from: "2026-10-05", to: "2026-10-05" })).toBe("5 Oct 2026");
  });
  it("gives each end its year when they differ", () => {
    expect(rangeText({ from: "2025-12-30", to: "2026-01-02" })).toBe("30 Dec 2025 – 2 Jan 2026");
  });
  it("counts the days under the title", () => {
    expect(insightText({ from: "2026-10-01", to: "2026-10-19" })).toBe("Insight into 19 days of trading");
    expect(insightText({ from: "2026-10-05", to: "2026-10-05" })).toBe("Insight into one day of trading");
    expect(insightText({ from: "", to: "2026-10-05" })).toBe("Pick a first and a last day");
  });
  it("says what the figures are compared with", () => {
    expect(beforeText({ from: "2026-09-29", to: "2026-10-05" })).toBe("the 7 days before");
    expect(beforeText({ from: "2026-10-05", to: "2026-10-05" })).toBe("the day before");
  });
});

describe("serviceName", () => {
  it("names the three ways, and leaves another as it came", () => {
    expect(serviceName("dine_in")).toBe("Dine-in");
    expect(serviceName("takeaway")).toBe("Takeaway");
    expect(serviceName("delivery")).toBe("Delivery");
    expect(serviceName("drive_through")).toBe("drive_through");
  });
});

describe("trend", () => {
  const series = [
    point("2026-10-01", 340_000, [group("dine_in", 3, 200_000), group("takeaway", 2, 140_000)]),
    point("2026-10-02", 0, []),
    point("2026-10-03", 100_000, [group("delivery", 1, 100_000)]),
  ];

  it("stacks each period by how it was served, against a round top", () => {
    const got = trend(series, "INR");
    expect(got.top).toBe(400_000);
    expect(got.bars[0]?.parts.map((p) => [p.id, p.percent])).toEqual([
      ["dine_in", 50],
      ["takeaway", 35],
      ["delivery", 0],
    ]);
    expect(got.bars[2]?.parts.map((p) => p.percent)).toEqual([0, 0, 25]);
  });
  it("gives a quiet period no bar", () => {
    const quiet = trend(series, "INR").bars[1];
    expect(quiet?.parts.every((p) => p.percent === 0 && p.total === null)).toBe(true);
  });
  it("gives a small part a height that can be seen", () => {
    const got = trend([point("2026-10-01", 400_100, [group("dine_in", 1, 400_000), group("takeaway", 1, 100)])], "INR");
    expect(got.bars[0]?.parts[1]?.percent).toBe(1);
  });
  it("has an axis even with nothing sold", () => {
    expect(trend([], "INR")).toEqual({ bars: [], top: 400 });
  });
});

describe("namedBars", () => {
  it("names every bar when there are few", () => {
    expect(namedBars(7)).toEqual([true, true, true, true, true, true, true]);
  });
  it("names about seven of many, the first always", () => {
    const got = namedBars(30);
    expect(got[0]).toBe(true);
    expect(got.filter(Boolean)).toHaveLength(6);
    expect(got[5]).toBe(true);
    expect(got[1]).toBe(false);
  });
  it("has nothing to name with no bars", () => {
    expect(namedBars(0)).toEqual([]);
  });
});

describe("slices", () => {
  const rows = (values: number[]) => values.map((value, i) => ({ key: `k${i}`, name: `Row ${i}`, value }));

  it("gives whole percentages that come to exactly 100", () => {
    const got = slices(rows([1, 1, 1]), 6);
    expect(got.map((s) => s.percent)).toEqual([34, 33, 33]);
    expect(got.reduce((n, s) => n + s.percent, 0)).toBe(100);
  });
  it("orders by size, whatever order the rows came in", () => {
    expect(slices(rows([10, 60, 30]), 6).map((s) => [s.key, s.percent])).toEqual([
      ["k1", 60],
      ["k2", 30],
      ["k0", 10],
    ]);
  });
  it("folds the smallest into Others once there are more than its limit", () => {
    const got = slices(rows([40, 30, 15, 10, 5]), 4);
    expect(got.map((s) => [s.key, s.percent])).toEqual([
      ["k0", 40],
      ["k1", 30],
      ["k2", 15],
      [othersKey, 15],
    ]);
    expect(got[3]?.name).toBe("Others");
  });
  it("keeps every row when they fit", () => {
    expect(slices(rows([40, 30, 20, 10]), 4).map((s) => s.key)).toEqual(["k0", "k1", "k2", "k3"]);
  });
  it("leaves out a row with nothing, or less", () => {
    expect(slices(rows([100, 0, -50]), 6).map((s) => [s.key, s.percent])).toEqual([["k0", 100]]);
  });
  it("is empty with nothing to share out", () => {
    expect(slices(rows([0, 0]), 6)).toEqual([]);
    expect(slices([], 6)).toEqual([]);
  });
  it("reads sales groups by total or by bills", () => {
    const groups = [group("dine_in", 3, 20_000), group("takeaway", 9, 10_000)];
    expect(slices(byTotal(groups), 3).map((s) => s.key)).toEqual(["dine_in", "takeaway"]);
    expect(
      slices(
        byBills(groups, (g) => g.key.toUpperCase()),
        3,
      ).map((s) => [s.name, s.percent]),
    ).toEqual([
      ["TAKEAWAY", 75],
      ["DINE_IN", 25],
    ]);
  });
});

describe("ringGradient", () => {
  it("runs each slice from where the last ended", () => {
    expect(ringGradient([{ percent: 60 }, { percent: 30 }, { percent: 10 }], ["a", "b", "c"])).toBe("conic-gradient(a 0% 60%, b 60% 90%, c 90% 100%)");
  });
  it("repeats the last colour when it runs out", () => {
    expect(ringGradient([{ percent: 50 }, { percent: 50 }], ["a"])).toBe("conic-gradient(a 0% 50%, a 50% 100%)");
  });
});

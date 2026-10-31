import { describe, expect, it } from "vitest";
import type { Group, LiveTableResponse, SalesHour, StockItem, TableStatus } from "../../../shared/api-types";
import {
  activeTables,
  axisTicks,
  axisTop,
  change,
  changeText,
  guestsText,
  hourChart,
  hourLabel,
  lowRows,
  occupancy,
  sinceText,
  staffRows,
  topItems,
} from "./dashboard-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

function group(key: string, name: string, qty: number, total: number): Group {
  const z = inr(0);
  return { key, name, qty, gross: z, discounts: z, net: z, tax: z, total: inr(total) };
}

function table(id: number, status: TableStatus, over: Partial<LiveTableResponse> = {}): LiveTableResponse {
  return { id, area_id: 1, name: `T${id}`, seats: 4, shape: "square", x: 0, y: 0, status, covers: 2, orders: [], ...over };
}

describe("change", () => {
  const tests: { name: string; today: number; yesterday: number; want: ReturnType<typeof change>; text: string }[] = [
    { name: "more than yesterday", today: 112, yesterday: 100, want: { direction: "up", percent: 12 }, text: "Up 12% on all of yesterday" },
    { name: "less than yesterday", today: 75, yesterday: 100, want: { direction: "down", percent: 25 }, text: "Down 25% on all of yesterday" },
    { name: "the same", today: 100, yesterday: 100, want: { direction: "same", percent: 0 }, text: "Same as all of yesterday" },
    { name: "a change too small to show", today: 1001, yesterday: 1000, want: { direction: "same", percent: 0 }, text: "Same as all of yesterday" },
    { name: "nothing yet today", today: 0, yesterday: 100, want: { direction: "down", percent: 100 }, text: "Down 100% on all of yesterday" },
    { name: "nothing yesterday", today: 100, yesterday: 0, want: null, text: "Nothing before to compare" },
    { name: "more than double", today: 350, yesterday: 100, want: { direction: "up", percent: 250 }, text: "Up 250% on all of yesterday" },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      const got = change(tt.today, tt.yesterday);
      expect(got).toEqual(tt.want);
      expect(changeText(got)).toBe(tt.text);
    });
  }
  it("names what a range is compared with", () => {
    expect(changeText({ direction: "down", percent: 4 }, "the 7 days before")).toBe("Down 4% on the 7 days before");
    expect(changeText({ direction: "same", percent: 0 }, "the day before")).toBe("Same as the day before");
  });
});

describe("occupancy", () => {
  it("counts every table that is not free", () => {
    expect(occupancy([table(1, "free"), table(2, "seated"), table(3, "dirty")])).toEqual({ active: 2, total: 3, percent: 67 });
  });
  it("is nothing of nothing for a venue with no tables", () => {
    expect(occupancy([])).toEqual({ active: 0, total: 0, percent: 0 });
  });
});

describe("activeTables", () => {
  const now = Date.parse("2026-10-05T12:00:00Z");
  const floor = [
    table(1, "free"),
    table(2, "ordered", { since: "2026-10-05T11:15:00Z", covers: 4 }),
    table(3, "seated", { since: "2026-10-05T10:55:00Z" }),
    table(4, "dirty"),
    table(5, "ordered", { since: "2026-10-05T11:15:00Z", joined_to: 2 }),
    table(6, "settling", { since: "2026-10-05T11:59:40Z" }),
  ];

  it("lists the tables in use, the longest sat first, and one with no start last", () => {
    const got = activeTables(floor, now, 5);
    expect(got.rows.map((r) => [r.name, r.since])).toEqual([
      ["T3", "1 h 05 min"],
      ["T2", "45 min"],
      ["T6", "Just now"],
      ["T4", ""],
    ]);
    expect(got.more).toBe(0);
  });

  it("leaves out a free table and one joined to another", () => {
    const names = activeTables(floor, now, 5).rows.map((r) => r.name);
    expect(names).not.toContain("T1");
    expect(names).not.toContain("T5");
  });

  it("says how many it had no room for", () => {
    const got = activeTables(floor, now, 2);
    expect(got.rows.map((r) => r.name)).toEqual(["T3", "T2"]);
    expect(got.more).toBe(2);
  });
});

describe("sinceText", () => {
  const tests: [number, string][] = [
    [0, "Just now"],
    [59_000, "Just now"],
    [60_000, "1 min"],
    [59 * 60_000, "59 min"],
    [60 * 60_000, "1 h 00 min"],
    [125 * 60_000, "2 h 05 min"],
    [-5000, "Just now"],
  ];
  for (const [ms, want] of tests) {
    it(`${ms} ms is ${want}`, () => expect(sinceText(ms)).toBe(want));
  }
});

describe("guestsText", () => {
  it("counts guests, and says when nobody did", () => {
    expect(guestsText(4)).toBe("4 guests");
    expect(guestsText(1)).toBe("1 guest");
    expect(guestsText(0)).toBe("No guest count");
  });
});

describe("topItems", () => {
  const items = [
    group("1", "Masala chai", 12, 36000),
    group("2", "Butter chicken", 12, 420000),
    group("3", "Naan", 30, 150000),
    group("4", "Comped water", 0, 0),
  ];

  it("orders by how many sold, then by what they took", () => {
    expect(topItems(items, 5).map((g) => g.name)).toEqual(["Naan", "Butter chicken", "Masala chai"]);
  });
  it("keeps to its limit and leaves the server's rows alone", () => {
    expect(topItems(items, 2).map((g) => g.name)).toEqual(["Naan", "Butter chicken"]);
    expect(items[0]?.name).toBe("Masala chai");
  });
});

describe("staffRows", () => {
  const groups = [group("5", "Kabir", 9, 540000), group("2", "Arjun", 3, 120000), group("7", "Someone no longer on record", 1, 30000)];
  const roles = { "5": "waiter", "2": "manager", "7": "" };

  it("gives each person the role the report says they hold", () => {
    expect(staffRows(groups, roles, 4)).toEqual([
      { id: "5", name: "Kabir", role: "waiter", bills: 9, total: inr(540000) },
      { id: "2", name: "Arjun", role: "manager", bills: 3, total: inr(120000) },
      { id: "7", name: "Someone no longer on record", role: "", bills: 1, total: inr(30000) },
    ]);
  });
  it("has no role for someone the report does not list", () => {
    expect(staffRows([group("9", "Asha", 1, 100)], {}, 4)[0]?.role).toBe("");
  });
  it("keeps to its limit", () => {
    expect(staffRows(groups, roles, 2)).toHaveLength(2);
  });
});

describe("axisTop", () => {
  const tests: { name: string; max: number; currency: string; want: number }[] = [
    { name: "nothing sold", max: 0, currency: "INR", want: 400 },
    { name: "3,400 rupees reaches 4,000", max: 340_000, currency: "INR", want: 400_000 },
    { name: "exactly on a step", max: 400_000, currency: "INR", want: 400_000 },
    { name: "just over a step", max: 400_001, currency: "INR", want: 800_000 },
    { name: "8,100 rupees reaches 10,000", max: 810_000, currency: "INR", want: 1_000_000 },
    { name: "a small sale", max: 550, currency: "INR", want: 800 },
    { name: "a currency with no minor unit", max: 3400, currency: "JPY", want: 4000 },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(axisTop(tt.max, tt.currency)).toBe(tt.want));
  }
  it("gives five ticks down to nothing", () => {
    expect(axisTicks(400_000)).toEqual([400_000, 300_000, 200_000, 100_000, 0]);
  });
});

describe("hourChart", () => {
  const cell = (hour: number, bills: number, total: number): SalesHour => ({ weekday: 1, hour, bills, total: inr(total) });

  it("runs from 8 AM to 10 PM on a day with nothing sold", () => {
    const chart = hourChart([], "INR", 4);
    expect(chart.bars.map((b) => b.hour)).toEqual([8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22]);
    expect(chart.bars.every((b) => b.percent === 0 && b.total === null)).toBe(true);
  });

  it("sets each hour's bar against a round top", () => {
    const chart = hourChart([cell(13, 3, 340_000), cell(20, 1, 100_000)], "INR", 4);
    expect(chart.top).toBe(400_000);
    expect(chart.bars.find((b) => b.hour === 13)).toEqual({ hour: 13, bills: 3, total: inr(340_000), percent: 85 });
    expect(chart.bars.find((b) => b.hour === 20)?.percent).toBe(25);
    expect(chart.bars.find((b) => b.hour === 9)?.percent).toBe(0);
  });

  it("widens for a bill before 8 AM or after midnight, in the trading day's order", () => {
    const hours = hourChart([cell(6, 1, 100), cell(1, 1, 100)], "INR", 4).bars.map((b) => b.hour);
    expect(hours[0]).toBe(6);
    expect(hours.slice(-4)).toEqual([22, 23, 0, 1]);
  });

  it("gives a small sale a bar that can be seen", () => {
    expect(hourChart([cell(9, 1, 100), cell(13, 1, 400_000)], "INR", 4).bars.find((b) => b.hour === 9)?.percent).toBe(1);
  });

  it("ignores a cell with no bills", () => {
    expect(hourChart([cell(5, 0, 0)], "INR", 4).bars[0]?.hour).toBe(8);
  });
});

describe("hourLabel", () => {
  const tests: [number, string][] = [
    [0, "12 AM"],
    [8, "8 AM"],
    [12, "12 PM"],
    [13, "1 PM"],
    [23, "11 PM"],
  ];
  for (const [hour, want] of tests) {
    it(`${hour} is ${want}`, () => expect(hourLabel(hour)).toBe(want));
  }
});

describe("lowRows", () => {
  const item = (id: number, on_hand: number, over: Partial<StockItem> = {}): StockItem => ({
    id,
    name: `Item ${id}`,
    unit: "g",
    low_level: 1000,
    active: true,
    on_hand,
    count_at_close: false,
    low: true,
    cost: inr(0),
    ...over,
  });

  it("puts what has run out first and keeps the rest in the server's order", () => {
    const got = lowRows([item(1, 500), item(2, 0), item(3, 200), item(4, -50)], 5);
    expect(got.rows.map((r) => [r.item.id, r.level])).toEqual([
      [2, "out"],
      [4, "out"],
      [1, "low"],
      [3, "low"],
    ]);
  });
  it("lists what has run out even with no low level set", () => {
    expect(lowRows([item(1, 0, { low: false, low_level: 0 })], 5).rows.map((r) => r.level)).toEqual(["out"]);
  });
  it("leaves out what is not low and what is retired", () => {
    expect(lowRows([item(1, 5000, { low: false }), item(2, 0, { active: false })], 5).rows).toEqual([]);
  });
  it("says how many it had no room for", () => {
    const got = lowRows([item(1, 1), item(2, 1), item(3, 1)], 2);
    expect(got.rows).toHaveLength(2);
    expect(got.more).toBe(1);
  });
});

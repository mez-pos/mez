import { describe, expect, it } from "vitest";
import type { StockItem } from "../../../shared/api-types";
import { levelOf, levelPercent, retiredCount, summary, summaryText, tableRows } from "./levels-logic";

const item = (name: string, on_hand: number, low_level: number, over: Partial<StockItem> = {}): StockItem => ({
  id: name.length,
  name,
  unit: "g",
  low_level,
  active: true,
  on_hand,
  // As the server says it: at or under a level that is set.
  low: low_level > 0 && on_hand <= low_level,
  count_at_close: false,
  cost: { minor: 0, currency: "INR" },
  ...over,
});

describe("levelOf", () => {
  const tests: { name: string; item: StockItem; want: string }[] = [
    { name: "well above its level", item: item("Beans", 5000, 1000), want: "ok" },
    { name: "at its level", item: item("Milk", 1000, 1000), want: "low" },
    { name: "under its level with some left", item: item("Milk", 200, 1000), want: "low" },
    { name: "none left", item: item("Fries", 0, 500), want: "out" },
    { name: "none left and no level set", item: item("Napkins", 0, 0), want: "out" },
    { name: "counted below nothing", item: item("Syrup", -50, 0), want: "out" },
    { name: "some left and no level set", item: item("Salt", 10, 0), want: "ok" },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(levelOf(tt.item)).toBe(tt.want));
  }
});

describe("levelPercent", () => {
  const tests: { name: string; on_hand: number; low_level: number; want: number | null }[] = [
    { name: "three times its level is full", on_hand: 3000, low_level: 1000, want: 100 },
    { name: "more than that is still full", on_hand: 9000, low_level: 1000, want: 100 },
    { name: "at its level is a third", on_hand: 1000, low_level: 1000, want: 33 },
    { name: "half its level", on_hand: 500, low_level: 1000, want: 17 },
    { name: "a little left can still be seen", on_hand: 1, low_level: 1000, want: 2 },
    { name: "none left", on_hand: 0, low_level: 1000, want: 0 },
    { name: "below nothing", on_hand: -5, low_level: 1000, want: 0 },
    { name: "no level set", on_hand: 500, low_level: 0, want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(levelPercent({ on_hand: tt.on_hand, low_level: tt.low_level })).toBe(tt.want));
  }
});

describe("the line under the title", () => {
  const items = [item("Beans", 5000, 1000), item("Milk", 500, 1000), item("Fries", 0, 500), item("Old syrup", 0, 100, { active: false })];

  it("counts the active ingredients, the low and the out", () => {
    expect(summary(items)).toEqual({ tracked: 3, low: 1, out: 1 });
    expect(summaryText(summary(items))).toBe("3 tracked ingredients · 1 low · 1 out of stock");
  });
  it("words one ingredient and none", () => {
    expect(summaryText({ tracked: 1, low: 0, out: 0 })).toBe("1 tracked ingredient · 0 low · 0 out of stock");
    // With none it says what to add, and where nothing can be added it does not.
    expect(summaryText({ tracked: 0, low: 0, out: 0 })).toBe("No ingredients yet. Add what you keep: beans, milk, batter.");
    expect(summaryText({ tracked: 0, low: 0, out: 0 }, true)).toBe("No ingredients yet.");
    expect(summaryText({ tracked: 1, low: 0, out: 0 }, true)).toBe("1 tracked ingredient · 0 low · 0 out of stock");
    expect(summaryText(summary([]))).toBe("No ingredients yet. Add what you keep: beans, milk, batter.");
  });
});

describe("tableRows", () => {
  const items = [item("Beans", 5000, 1000), item("Old syrup", 0, 100, { active: false }), item("Milk", 500, 1000)];

  it("leaves the retired out unless asked, and puts them last when it is", () => {
    expect(tableRows(items, false).map((i) => i.name)).toEqual(["Beans", "Milk"]);
    expect(tableRows(items, true).map((i) => i.name)).toEqual(["Beans", "Milk", "Old syrup"]);
    expect(retiredCount(items)).toBe(1);
  });
});

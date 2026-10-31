import { describe, expect, it } from "vitest";
import type { Group, ItemMargin, StockItem, WasteItem, WasteReasonTotal } from "../../../shared/api-types";
import {
  entriesText,
  itemRows,
  itemsText,
  namesText,
  reasonName,
  stockHealth,
  wasteBars,
  wasteLeftOut,
  wastePath,
  worstReason,
  costingNote,
  itemsPath,
  marginText,
} from "./items-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

function group(key: string, name: string, qty: number, net: number): Group {
  const z = inr(0);
  return { key, name, qty, gross: inr(net), discounts: z, net: inr(net), tax: z, total: inr(net) };
}

describe("itemRows", () => {
  const items = [group("1", "Chai", 20, 60_000), group("2", "Butter chicken", 5, 210_000), group("3", "Naan", 10, 50_000), group("4", "Comped water", 0, 0)];
  const menu = [
    { id: 1, diet: "veg" as const },
    { id: 2, diet: "non_veg" as const },
  ];

  it("orders by net sales and gives each its mark from the menu", () => {
    expect(itemRows(items, null, menu, null, 8).map((r) => [r.name, r.diet, r.net.minor])).toEqual([
      ["Butter chicken", "non_veg", 210_000],
      ["Chai", "veg", 60_000],
      ["Naan", "", 50_000],
    ]);
  });
  it("sets how many sold beside the days before", () => {
    const before = [group("1", "Chai", 10, 30_000), group("2", "Butter chicken", 10, 420_000)];
    const rows = itemRows(items, before, menu, null, 8);
    expect(rows.map((r) => r.trend)).toEqual([{ direction: "down", percent: 50 }, { direction: "up", percent: 100 }, null]);
  });
  it("has no trend while the days before are not read", () => {
    expect(itemRows(items, null, menu, null, 8).every((r) => r.trend === undefined)).toBe(true);
  });
  it("gives each item the cost and margin the server worked out", () => {
    const margins: ItemMargin[] = [
      { key: "2", name: "Butter chicken", qty: 5, net: inr(210_000), costing: "full", cost: inr(90_000), costed_qty: 5, margin: inr(120_000), margin_bp: 5714 },
      { key: "3", name: "Naan", qty: 10, net: inr(50_000), costing: "none", costed_qty: 0 },
    ];
    const rows = itemRows(items, null, menu, margins, 8);
    expect(rows.map((r) => r.margin?.costing)).toEqual(["full", undefined, "none"]);
    expect(rows[0]?.margin?.margin).toEqual(inr(120_000));
  });
  it("keeps to its limit and leaves the server's rows alone", () => {
    expect(itemRows(items, null, menu, null, 2)).toHaveLength(2);
    expect(items[0]?.name).toBe("Chai");
  });
});

describe("cost and margin in words", () => {
  const tests: [number, string][] = [
    [8433, "84.3%"],
    [10000, "100.0%"],
    [0, "0.0%"],
    [5, "0.1%"],
    [4, "0.0%"],
    [-2500, "−25.0%"],
    [-4, "0.0%"],
    [9999, "100.0%"],
  ];
  for (const [bp, want] of tests) {
    it(`${bp} is ${want}`, () => expect(marginText(bp)).toBe(want));
  }
  it("says how far the costs can be trusted", () => {
    expect(costingNote({ kept_from: "2026-10-05", short: false })).toBe("");
    expect(costingNote({ short: false })).toMatch(/^No dish has a cost yet\./);
    expect(costingNote({ kept_from: "2026-10-05", short: true })).toMatch(/^Costs by dish are kept from 5 Oct 2026\. .* margins are not shown\.$/);
  });
  it("builds the path for a range", () => {
    expect(itemsPath({ from: "2026-10-01", to: "2026-10-05" })).toBe("/api/v1/reports/items?from=2026-10-01&to=2026-10-05");
  });
});

describe("wasteBars", () => {
  const item = (id: number, value: number, entries: number, uncosted: number): WasteItem => ({
    stock_item_id: id,
    name: `Item ${id}`,
    unit: "g",
    qty: 1000,
    entries,
    value: inr(value),
    uncosted,
  });

  it("sets each loss against the largest", () => {
    const got = wasteBars([item(1, 10_000, 2, 0), item(2, 2_500, 1, 0)], 4);
    expect(got.bars.map((b) => b.percent)).toEqual([100, 25]);
    expect(got.more).toBe(0);
  });
  it("gives a small loss a bar that can be seen", () => {
    expect(wasteBars([item(1, 100_000, 1, 0), item(2, 10, 1, 0)], 4).bars[1]?.percent).toBe(2);
  });
  it("marks an item none of whose entries had a cost", () => {
    const got = wasteBars([item(1, 5_000, 2, 1), item(2, 0, 1, 1)], 4);
    expect(got.bars.map((b) => [b.notCosted, b.percent])).toEqual([
      [false, 100],
      [true, 0],
    ]);
  });
  it("says how many it had no room for", () => {
    const got = wasteBars([item(1, 3, 1, 0), item(2, 2, 1, 0), item(3, 1, 1, 0)], 2);
    expect(got.bars).toHaveLength(2);
    expect(got.more).toBe(1);
  });
  it("is empty with nothing written off", () => {
    expect(wasteBars([], 4)).toEqual({ bars: [], more: 0 });
  });
});

describe("the write-offs in words", () => {
  const total = (reason: WasteReasonTotal["reason"], value: number): WasteReasonTotal => ({ reason, entries: 1, value: inr(value), uncosted: 0 });

  it("names a reason", () => {
    expect(reasonName("spoiled")).toBe("Spoiled");
    expect(reasonName("stolen")).toBe("stolen");
  });
  it("picks the reason that cost most, when anything was costed", () => {
    expect(worstReason([total("dropped", 6000), total("spilled", 3000)])?.reason).toBe("dropped");
    expect(worstReason([total("spoiled", 0)])).toBeNull();
    expect(worstReason([])).toBeNull();
  });
  it("counts entries and says what the total leaves out", () => {
    expect(entriesText(1)).toBe("1 entry");
    expect(entriesText(4)).toBe("4 entries");
    expect(wasteLeftOut({ uncosted: 0 })).toBe("");
    expect(wasteLeftOut({ uncosted: 1 })).toBe("1 entry had no cost and is not in the total.");
    expect(wasteLeftOut({ uncosted: 3 })).toBe("3 entries had no cost and are not in the total.");
  });
  it("builds the path for a range", () => {
    expect(wastePath({ from: "2026-10-01", to: "2026-10-05" })).toBe("/api/v1/reports/wastage?from=2026-10-01&to=2026-10-05");
  });
});

describe("stockHealth", () => {
  const item = (name: string, on_hand: number, low: boolean, active = true): StockItem => ({
    id: name.length,
    name,
    unit: "g",
    low_level: 1000,
    active,
    on_hand,
    count_at_close: false,
    low,
    cost: inr(0),
  });

  it("counts what is healthy, low and out among the active items", () => {
    const got = stockHealth([item("Beans", 5000, false), item("Milk", 500, true), item("Fries", 0, true), item("Old syrup", 0, true, false)]);
    expect(got).toEqual({ healthy: 1, low: 1, out: 1, outNames: ["Fries"] });
  });
  it("counts an item with none left as out even with no low level set", () => {
    expect(stockHealth([item("Napkins", 0, false)])).toEqual({ healthy: 0, low: 0, out: 1, outNames: ["Napkins"] });
  });
  it("is all zero with no stock items", () => {
    expect(stockHealth([])).toEqual({ healthy: 0, low: 0, out: 0, outNames: [] });
  });
  it("words counts and names", () => {
    expect(itemsText(1)).toBe("1 item");
    expect(itemsText(48)).toBe("48 items");
    expect(namesText(["Fries"])).toBe("Fries");
    expect(namesText(["Fries", "Milk", "Beans"])).toBe("Fries, Milk, Beans");
    expect(namesText(["Fries", "Milk", "Beans", "Tea"])).toBe("Fries, Milk and 2 more");
  });
});

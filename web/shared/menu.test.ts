import { describe, expect, it } from "vitest";
import type { MenuItemResponse, MenuResponse } from "./api-types";
import { indexMenu, isMenuStale, isSoldOut, photoUrl } from "./menu";

const rupees = (r: number) => ({ minor: r * 100, currency: "INR" });

const item = (id: number, category: number, groups: number[] = []): MenuItemResponse => ({
  id,
  category_id: category,
  name: `item ${id}`,
  price: rupees(100),
  station: "bar",
  course: 1,
  tax_rate: 5000,
  modifier_groups: groups,
});

const menu: MenuResponse = {
  version: 7,
  currency: "INR",
  stations: [{ id: 1, code: "bar", name: "Bar" }],
  courses: [{ id: 1, number: 1, name: "First" }],
  categories: [
    { id: 10, name: "Coffee" },
    { id: 20, name: "Cakes" },
  ],
  items: [item(1, 10, [300, 100]), item(2, 20), item(3, 10, [999])],
  modifier_groups: [
    { id: 100, name: "Milk", min_select: 1, max_select: 1, modifiers: [] },
    { id: 300, name: "Extras", min_select: 0, max_select: 3, modifiers: [] },
  ],
};

describe("isMenuStale", () => {
  const cases: [string, Parameters<typeof isMenuStale>[0], { version: number } | undefined, boolean][] = [
    ["another version", { kind: "menu", version: 8 }, { version: 7 }, true],
    ["an older version too", { kind: "menu", version: 6 }, { version: 7 }, true],
    ["the version it has", { kind: "menu", version: 7 }, { version: 7 }, false],
    ["nothing cached", { kind: "menu", version: 8 }, undefined, false],
    ["a resync is not about the menu", { kind: "resync", from: 0 }, { version: 7 }, false],
  ];
  it.each(cases)("%s", (_, notice, cached, stale) => {
    expect(isMenuStale(notice, cached)).toBe(stale);
  });
});

describe("indexMenu", () => {
  const index = indexMenu(menu);

  it("files items under their category, in menu order", () => {
    expect(index.itemsIn(10).map((i) => i.id)).toEqual([1, 3]);
    expect(index.itemsIn(20).map((i) => i.id)).toEqual([2]);
    expect(index.itemsIn(99)).toEqual([]);
  });

  it("finds an item by id", () => {
    expect(index.item(2)?.category_id).toBe(20);
    expect(index.item(42)).toBeUndefined();
  });

  it("gives an item's groups in the item's order, dropping any it cannot find", () => {
    expect(index.groupsFor(menu.items[0]!).map((g) => g.name)).toEqual(["Extras", "Milk"]);
    expect(index.groupsFor(menu.items[1]!)).toEqual([]);
    expect(index.groupsFor(menu.items[2]!)).toEqual([]);
  });
});

describe("isSoldOut", () => {
  const now = new Date("2026-09-30T12:00:00Z");
  const tests: { name: string; until: string | undefined; want: boolean }[] = [
    { name: "on sale", until: undefined, want: false },
    { name: "until tomorrow morning", until: "2026-09-30T22:30:00Z", want: true },
    { name: "a time already passed: back on sale", until: "2026-09-30T11:59:59Z", want: false },
    { name: "exactly now: back on sale", until: "2026-09-30T12:00:00Z", want: false },
    { name: "an empty time", until: "", want: false },
    { name: "a time it cannot read", until: "tomorrow", want: false },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(isSoldOut(tt.until === undefined ? {} : { sold_out_until: tt.until }, now)).toBe(tt.want);
    });
  }
});

describe("photoUrl", () => {
  it("serves a photo by its name", () => {
    expect(photoUrl("ab12.jpg")).toBe("/api/v1/menu/photos/ab12.jpg");
    expect(photoUrl("../x")).toBe("/api/v1/menu/photos/..%2Fx");
  });
});

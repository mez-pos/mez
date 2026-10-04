import { describe, expect, it } from "vitest";
import type { MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import {
  addLineFor,
  categoryIconKey,
  choiceCount,
  defaultModifiers,
  dietFilterActive,
  filterItems,
  fold,
  itemsForView,
  noDietFilter,
  popularItems,
  searchItems,
  sidebarEntries,
} from "./register-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const item = (id: number, name: string, extra: Partial<MenuItemResponse> = {}): MenuItemResponse => ({
  id,
  category_id: 1,
  name,
  price: inr(18000),
  station: "bar",
  course: 1,
  tax_rate: 5000,
  modifier_groups: [],
  ...extra,
});

const group = (min: number, max: number, defaults: boolean[]): ModifierGroupResponse => ({
  id: 1,
  name: "G",
  min_select: min,
  max_select: max,
  modifiers: defaults.map((d, i) => ({ id: 10 + i, name: `m${i}`, price: inr(i * 1000), is_default: d })),
});

describe("fold", () => {
  it("drops case and accents", () => {
    expect(fold("Café CRÈME")).toBe("cafe creme");
  });
});

describe("searchItems", () => {
  const items = [item(1, "Flat white"), item(2, "Filter coffee"), item(3, "Café latte"), item(4, "Iced tea")];
  const cases: [string, string, number[]][] = [
    ["a word prefix", "fla", [1]],
    ["a later word", "white", [1]],
    ["every word must match", "fl wh", [1]],
    ["shared prefix, menu order", "f", [1, 2]],
    ["accents ignored", "cafe", [3]],
    ["case ignored", "ICED", [4]],
    ["not mid-word", "ite", []],
    ["nothing typed", "   ", []],
    ["no match", "burger", []],
  ];
  it.each(cases)("%s", (_, query, ids) => {
    expect(searchItems(items, query).map((i) => i.id)).toEqual(ids);
  });
});

describe("defaultModifiers", () => {
  it("rings up with no modifiers when the item has no groups", () => {
    expect(defaultModifiers([])).toEqual([]);
  });

  it("takes each group's defaults", () => {
    expect(defaultModifiers([group(1, 1, [true, false]), group(0, 3, [false, false])])).toEqual([{ modifier_id: 10, name: "m0", price: inr(0) }]);
  });

  const refused: [string, ModifierGroupResponse][] = [
    ["a required group with no default", group(1, 1, [false, false])],
    ["too few defaults", group(2, 3, [true, false, false])],
    ["too many defaults", group(0, 1, [true, true])],
  ];
  it.each(refused)("needs a choice for %s", (_, g) => {
    expect(defaultModifiers([g])).toBeNull();
  });
});

describe("addLineFor", () => {
  it("copies the snapshot facts, one of it", () => {
    const line = addLineFor(item(1, "Flat white", { hsn: "996331", course: 2, station: "kitchen" }), [{ modifier_id: 3, name: "Oat", price: inr(5000) }]);
    expect(line).toEqual({
      item_id: 1,
      name: "Flat white",
      qty: 1,
      unit_price: inr(18000),
      modifiers: [{ modifier_id: 3, name: "Oat", price: inr(5000) }],
      course: 2,
      station: "kitchen",
      tax_rate: 5000,
      hsn: "996331",
    });
  });

  it("marks an exempt item, and leaves out what is not there", () => {
    const line = addLineFor(item(2, "Water", { tax_exempt: true, tax_rate: 0 }), []);
    expect(line.tax_exempt).toBe(true);
    expect("hsn" in line).toBe(false);
  });
});

describe("choiceCount", () => {
  it("counts every modifier across groups", () => {
    expect(choiceCount([group(1, 1, [true, false]), group(0, 3, [false, false, false])])).toBe(5);
  });
});

describe("filterItems", () => {
  const menu = [
    item(1, "Paneer", { diet: "veg" }),
    item(2, "Chicken", { diet: "non_veg" }),
    item(3, "Tofu bowl", { diet: "vegan", gluten_free: true }),
    item(4, "Plain toast"),
    item(5, "Veg rice", { diet: "veg", gluten_free: true }),
  ];
  const names = (f: Partial<typeof noDietFilter>) => filterItems(menu, { ...noDietFilter, ...f }).map((i) => i.name);

  it("shows everything with nothing on", () => {
    expect(names({})).toHaveLength(5);
    expect(dietFilterActive(noDietFilter)).toBe(false);
  });
  it("adds the diets up", () => {
    expect(names({ veg: true })).toEqual(["Paneer", "Veg rice"]);
    expect(names({ veg: true, vegan: true })).toEqual(["Paneer", "Tofu bowl", "Veg rice"]);
    expect(names({ nonVeg: true })).toEqual(["Chicken"]);
  });
  it("narrows by gluten free", () => {
    expect(names({ glutenFree: true })).toEqual(["Tofu bowl", "Veg rice"]);
    expect(names({ glutenFree: true, veg: true })).toEqual(["Veg rice"]);
  });
  it("leaves an item with no diet out of any diet filter, but not out of none", () => {
    expect(names({ veg: true, nonVeg: true, vegan: true })).not.toContain("Plain toast");
    expect(dietFilterActive({ ...noDietFilter, glutenFree: true })).toBe(true);
  });
});

describe("popularItems", () => {
  it("keeps the popular ones in menu order", () => {
    const menu = [item(1, "A", { popular: true }), item(2, "B"), item(3, "C", { popular: true })];
    expect(popularItems(menu).map((i) => i.name)).toEqual(["A", "C"]);
    expect(popularItems([item(1, "A")])).toEqual([]);
  });
});

describe("categoryIconKey", () => {
  const tests = [
    { name: "Pizzas", want: "pizza" },
    { name: "Burgers & Sandwiches", want: "burger" },
    { name: "Salads", want: "salad" },
    { name: "Hot Beverages", want: "coffee" },
    { name: "Tea", want: "coffee" },
    { name: "Steak", want: "utensils" },
    { name: "Desserts", want: "dessert" },
    { name: "Ice Cream", want: "dessert" },
    { name: "Main Course", want: "soup" },
    { name: "Starters", want: "leaf" },
    { name: "Café Specials", want: "utensils" },
    { name: "", want: "utensils" },
  ];
  for (const tt of tests) {
    it(`${JSON.stringify(tt.name)} is ${tt.want}`, () => {
      expect(categoryIconKey(tt.name)).toBe(tt.want);
    });
  }
});

describe("sidebarEntries and itemsForView", () => {
  const categories = [
    { id: 1, name: "Starters" },
    { id: 2, name: "Beverages" },
  ];
  const menu = [item(1, "Fries", { category_id: 1, popular: true }), item(2, "Chai", { category_id: 2 }), item(3, "Soup", { category_id: 1 })];

  it("lists All Items, Popular and the categories with their counts", () => {
    expect(sidebarEntries(categories, menu).map((e) => [e.view, e.label, e.count])).toEqual([
      ["all", "All Items", 3],
      ["popular", "Popular", 1],
      [1, "Starters", 2],
      [2, "Beverages", 1],
    ]);
  });
  it("leaves Popular out when nothing is popular", () => {
    const plain = menu.map((i) => ({ ...i, popular: false }));
    expect(sidebarEntries(categories, plain).map((e) => e.view)).toEqual(["all", 1, 2]);
  });
  it("shows each view's items in menu order", () => {
    expect(itemsForView("all", menu).map((i) => i.id)).toEqual([1, 2, 3]);
    expect(itemsForView("popular", menu).map((i) => i.id)).toEqual([1]);
    expect(itemsForView(1, menu).map((i) => i.id)).toEqual([1, 3]);
    expect(itemsForView(9, menu)).toEqual([]);
  });
});

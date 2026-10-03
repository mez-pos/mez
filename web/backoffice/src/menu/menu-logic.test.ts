import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { EditorItem, MenuEditor } from "../../../shared/api-types";
import {
  choices,
  itemFormFrom,
  itemFromSearch,
  itemsOf,
  moved,
  namesById,
  newItemForm,
  priceChange,
  problemOf,
  rateText,
  selectedCategory,
  taxClassText,
  toItemInput,
  unreadablePrice,
} from "./menu-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const item = (id: number, category: number, name: string, price: number, extra: Partial<EditorItem> = {}): EditorItem => ({
  id,
  category_id: category,
  name,
  price: inr(price),
  hsn: "",
  tax_class_id: 1,
  station_id: 2,
  course_id: 1,
  active: true,
  modifier_groups: [],
  ...extra,
});

const menu: MenuEditor = {
  version: 7,
  currency: "INR",
  needs_hsn: false,
  stations: [
    { id: 1, code: "bar", name: "Bar", active: false },
    { id: 2, code: "kitchen", name: "Kitchen", active: true },
  ],
  courses: [{ id: 1, number: 1, name: "Mains", active: true }],
  tax_classes: [
    { id: 1, name: "Food", rate: 5000, exempt: false, active: true },
    { id: 2, name: "Water", rate: 0, exempt: true, active: true },
    { id: 3, name: "Old rate", rate: 12000, exempt: false, active: false },
  ],
  categories: [
    { id: 10, name: "Coffee", active: true },
    { id: 11, name: "Breakfast", active: false },
  ],
  items: [item(100, 10, "Latte", 18000), item(101, 10, "Mocha", 20000), item(102, 11, "Dosa", 12000, { station_id: 1 })],
  modifier_groups: [],
};

describe("itemsOf and selectedCategory", () => {
  it("lists a category's items in order", () => {
    expect(itemsOf(menu, 10).map((i) => i.name)).toEqual(["Latte", "Mocha"]);
    expect(itemsOf(menu, 99)).toEqual([]);
  });
  it("keeps the chosen category, falls back to the first, and to none", () => {
    expect(selectedCategory(menu, 11)?.name).toBe("Breakfast");
    expect(selectedCategory(menu, 99)?.name).toBe("Coffee");
    expect(selectedCategory(menu, null)?.name).toBe("Coffee");
    expect(selectedCategory({ ...menu, categories: [] }, 10)).toBeNull();
  });
});

describe("moved", () => {
  const tests: { name: string; index: number; delta: -1 | 1; want: number[] | null }[] = [
    { name: "up", index: 1, delta: -1, want: [2, 1, 3] },
    { name: "down", index: 1, delta: 1, want: [1, 3, 2] },
    { name: "the first cannot go up", index: 0, delta: -1, want: null },
    { name: "the last cannot go down", index: 2, delta: 1, want: null },
    { name: "an index past the end", index: 5, delta: -1, want: null },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      const ids = [1, 2, 3];
      expect(moved(ids, tt.index, tt.delta)).toEqual(tt.want);
      expect(ids).toEqual([1, 2, 3]);
    });
  }
});

describe("rateText and taxClassText", () => {
  const tests: [number, string][] = [
    [5000, "5%"],
    [18000, "18%"],
    [2500, "2.5%"],
    [12375, "12.375%"],
    [50, "0.05%"],
    [0, "0%"],
  ];
  for (const [rate, want] of tests) {
    it(`words ${rate} as ${want}`, () => expect(rateText(rate)).toBe(want));
  }
  it("names a class with its rate, or as exempt", () => {
    expect(taxClassText(menu.tax_classes[0]!)).toBe("Food, 5%");
    expect(taxClassText(menu.tax_classes[1]!)).toBe("Water, exempt");
    expect(taxClassText({ id: 3, name: "Restaurant 5%", rate: 5000, exempt: false, active: true })).toBe("Restaurant 5%");
    expect(taxClassText({ id: 4, name: "Exempt", rate: 0, exempt: true, active: true })).toBe("Exempt");
    expect(taxClassText({ id: 5, name: "Food 5%", rate: 18000, exempt: false, active: true })).toBe("Food 5%, 18%");
  });
});

describe("choices", () => {
  it("offers the active rows and keeps a retired one already chosen", () => {
    expect(choices(menu.stations, 2).map((s) => s.name)).toEqual(["Kitchen"]);
    expect(choices(menu.stations, 1).map((s) => s.name)).toEqual(["Bar", "Kitchen"]);
  });
});

describe("the item form", () => {
  it("starts a new item in the category, with the first usable choices", () => {
    expect(newItemForm(menu, 11)).toEqual({ name: "", categoryId: 11, price: "", hsn: "", taxClassId: 1, stationId: 2, courseId: 1 });
  });
  it("offers the courses in service, and a retired one only to an item that has it", () => {
    const withRetired = { ...menu, courses: [{ id: 1, number: 1, name: "Mains", active: false }, { id: 2, number: 2, name: "Desserts", active: true }] };
    expect(newItemForm(withRetired, 10).courseId).toBe(2);
    expect(choices(withRetired.courses, 2).map((c) => c.name)).toEqual(["Desserts"]);
    expect(choices(withRetired.courses, 1).map((c) => c.name)).toEqual(["Mains", "Desserts"]);
  });
  it("starts with nothing chosen on an empty menu, for the server to refuse", () => {
    const empty = { ...menu, stations: [], courses: [], tax_classes: [] };
    expect(newItemForm(empty, 10)).toMatchObject({ taxClassId: 0, stationId: 0, courseId: 0 });
    const firstRetired = { ...menu, tax_classes: [{ ...menu.tax_classes[0]!, active: false }, menu.tax_classes[1]!] };
    expect(newItemForm(firstRetired, 10).taxClassId).toBe(2);
  });
  it("reads an item into the form and back unchanged", () => {
    const latte = menu.items[0]!;
    const form = itemFormFrom(latte);
    expect(form.price).toBe("180.00");
    expect(toItemInput(form, "INR")).toEqual({
      input: { category_id: 10, name: "Latte", price: inr(18000), hsn: "", tax_class_id: 1, station_id: 2, course_id: 1 },
    });
  });
  it("trims the name and the HSN", () => {
    const form = { ...newItemForm(menu, 10), name: "  Chai ", price: "60", hsn: " 996331 " };
    expect(toItemInput(form, "INR")).toMatchObject({ input: { name: "Chai", hsn: "996331", price: inr(6000) } });
  });
  it("refuses a price it cannot read before sending anything", () => {
    for (const price of ["", "abc", "12.345", "-5"]) {
      expect(toItemInput({ ...newItemForm(menu, 10), price }, "INR")).toEqual({ problem: { field: "price", message: unreadablePrice } });
    }
  });
});

describe("priceChange", () => {
  const latte = menu.items[0]!;
  it("says what open tickets keep and what new lines cost", () => {
    expect(priceChange(latte, "200")).toEqual({ was: inr(18000), now: inr(20000) });
  });
  it("says nothing for a new item, the same price, or one it cannot read", () => {
    expect(priceChange(null, "200")).toBeNull();
    expect(priceChange(latte, "180.00")).toBeNull();
    expect(priceChange(latte, "180")).toBeNull();
    expect(priceChange(latte, "18o")).toBeNull();
  });
  it("counts going down to free as a change", () => {
    expect(priceChange(latte, "0")).toEqual({ was: inr(18000), now: inr(0) });
  });
});

describe("problemOf", () => {
  const refusal = (code: string, field: string | null = null) =>
    new ApiError(code as ConstructorParameters<typeof ApiError>[0], "The server's words.", 422, field);

  it("puts a field's refusal under that field", () => {
    expect(problemOf(refusal("invalid_menu", "hsn"))).toEqual({ field: "hsn", message: "The server's words." });
  });
  it("puts a refusal of the item's groups under them", () => {
    expect(problemOf(refusal("invalid_menu", "ids")).field).toBe("ids");
  });
  it("puts a refusal about no field over the form", () => {
    expect(problemOf(refusal("item_not_found"))).toEqual({ field: null, message: "The server's words." });
  });
  it("words a stale order, a refusal of permission and anything else", () => {
    expect(problemOf(refusal("menu_order_stale")).message).toMatch(/changed on another screen/);
    expect(problemOf(refusal("forbidden", "menu.edit"))).toEqual({ field: null, message: "Only someone who can edit the menu can do this." });
    expect(problemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});

describe("namesById", () => {
  it("names each column's id, marking a retired station", () => {
    const n = namesById(menu);
    expect([n.taxClass(1), n.station(1), n.station(2), n.course(1), n.course(9)]).toEqual([
      "Food, 5%",
      "Bar (retired)",
      "Kitchen",
      "1 Mains",
      "Unknown",
    ]);
  });
});

describe("a link to an item", () => {
  const item = (id: number, category: number) => ({ ...menu.items[0]!, id, category_id: category });
  const withItems = { ...menu, items: [item(7, 10), item(12, 11)] };
  it("names it by id", () => {
    expect(itemFromSearch("?item=12", withItems)?.id).toBe(12);
    expect(itemFromSearch("item=7", withItems)?.category_id).toBe(10);
    expect(itemFromSearch("?x=1&item=7", withItems)?.id).toBe(7);
  });
  it("is no item when it names none, or one the menu lacks", () => {
    for (const bad of ["", "?item=", "?item=99", "?item=0", "?item=-1", "?item=1.5", "?item=abc", "?item=7x", "?other=7", "?item=12345678901234567890"]) {
      expect(itemFromSearch(bad, withItems), bad).toBeNull();
    }
  });
});

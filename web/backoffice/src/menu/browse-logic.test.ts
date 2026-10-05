import { describe, expect, it } from "vitest";
import type { EditorCategory, EditorItem, MenuEditor } from "../../../shared/api-types";
import { canReorder, emptyLine, itemState, listItems, listTitle, scopeRows, validScope, type ListFilter } from "./browse-logic";

const now = new Date("2026-10-05T12:00:00Z");
const later = "2026-10-05T22:30:00Z";

function item(id: number, name: string, category_id: number, price: number, over: Partial<EditorItem> = {}): EditorItem {
  return {
    id,
    category_id,
    name,
    price: { minor: price, currency: "INR" },
    hsn: "",
    tax_class_id: 1,
    station_id: 1,
    course_id: 1,
    active: true,
    modifier_groups: [],
    photo: "",
    description: "",
    diet: "",
    gluten_free: false,
    popular: false,
    ...over,
  } as EditorItem;
}

const category = (id: number, name: string, active = true): EditorCategory => ({ id, name, active }) as EditorCategory;

// Coffee comes before Cakes on the till; the server sends items by category, then in each category's order.
const menu = {
  currency: "INR",
  categories: [category(2, "Coffee"), category(1, "Cakes"), category(3, "Old specials", false)],
  items: [
    item(10, "Latte", 2, 18000, { popular: true }),
    item(11, "Americano", 2, 15000),
    item(12, "Espresso", 2, 12000, { sold_out_until: later }),
    item(20, "Lemon tart", 1, 19000, { popular: true }),
    item(21, "Brownie", 1, 15000, { active: false }),
    item(30, "Pumpkin latte", 3, 22000),
  ],
} as unknown as MenuEditor;

const filter = (over: Partial<ListFilter> = {}): ListFilter => ({ scope: "all", search: "", show: "all", order: "menu", ...over });
const names = (f: Partial<ListFilter>) => listItems(menu, filter(f), now).map((i) => i.name);

describe("scopeRows", () => {
  it("lists All items, Popular, then each category in register order with its count", () => {
    expect(scopeRows(menu).map((r) => [r.scope, r.name, r.count, r.retired])).toEqual([
      ["all", "All items", 6, false],
      ["popular", "Popular", 2, false],
      [2, "Coffee", 3, false],
      [1, "Cakes", 2, false],
      [3, "Old specials", 1, true],
    ]);
  });
  it("falls back to All items when the category asked for has gone", () => {
    expect(validScope(menu, 2)).toBe(2);
    expect(validScope(menu, 99)).toBe("all");
    expect(validScope(menu, "popular")).toBe("popular");
  });
});

describe("itemState", () => {
  it("is active, sold out for now, or retired; retired wins", () => {
    expect(itemState({ active: true }, now)).toBe("active");
    expect(itemState({ active: true, sold_out_until: later }, now)).toBe("sold_out");
    expect(itemState({ active: true, sold_out_until: "2026-10-05T06:00:00Z" }, now)).toBe("active");
    expect(itemState({ active: false, sold_out_until: later }, now)).toBe("retired");
  });
});

describe("listItems", () => {
  it("shows everything in menu order: by category as the till has them, then each category's own order", () => {
    expect(names({})).toEqual(["Latte", "Americano", "Espresso", "Lemon tart", "Brownie", "Pumpkin latte"]);
  });
  it("shows one category, or the popular items", () => {
    expect(names({ scope: 1 })).toEqual(["Lemon tart", "Brownie"]);
    expect(names({ scope: "popular" })).toEqual(["Latte", "Lemon tart"]);
  });
  it("searches every category, whatever the case, whichever is selected", () => {
    expect(names({ scope: 1, search: " LATTE " })).toEqual(["Latte", "Pumpkin latte"]);
    expect(names({ search: "zzz" })).toEqual([]);
  });
  it("filters by where an item stands", () => {
    expect(names({ show: "sold_out" })).toEqual(["Espresso"]);
    expect(names({ show: "retired" })).toEqual(["Brownie"]);
    expect(names({ show: "active", scope: 2 })).toEqual(["Latte", "Americano"]);
  });
  it("orders by name or by price", () => {
    expect(names({ scope: 2, order: "name" })).toEqual(["Americano", "Espresso", "Latte"]);
    expect(names({ order: "price" })).toEqual(["Espresso", "Americano", "Brownie", "Latte", "Lemon tart", "Pumpkin latte"]);
  });
  it("leaves the menu's own list alone", () => {
    names({ order: "name" });
    expect(menu.items[0]?.name).toBe("Latte");
  });
});

describe("canReorder", () => {
  it("only for one category in menu order with nothing filtered out", () => {
    expect(canReorder(filter({ scope: 2 }))).toBe(true);
    expect(canReorder(filter({ scope: "all" }))).toBe(false);
    expect(canReorder(filter({ scope: 2, order: "name" }))).toBe(false);
    expect(canReorder(filter({ scope: 2, show: "active" }))).toBe(false);
    expect(canReorder(filter({ scope: 2, search: "la" }))).toBe(false);
  });
});

describe("the list's words", () => {
  it("counts its items in the title", () => {
    expect(listTitle(68)).toBe("Menu items (68)");
  });
  it("says why it is empty", () => {
    const none = { ...menu, items: [] } as MenuEditor;
    expect(emptyLine(menu, filter({ search: "zzz" }))).toBe('Nothing on the menu matches "zzz".');
    expect(emptyLine({ ...none, categories: [] } as MenuEditor, filter())).toBe("No menu yet. Add a category, then the items in it.");
    expect(emptyLine(none, filter())).toBe("No items yet. Add the first one.");
    expect(emptyLine(menu, filter({ show: "retired", scope: 2 }))).toBe("No items like that here.");
    expect(emptyLine(menu, filter({ scope: "popular" }))).toBe("No item is marked popular yet.");
    expect(emptyLine(menu, filter({ scope: 3 }))).toBe("No items here yet.");
  });
});

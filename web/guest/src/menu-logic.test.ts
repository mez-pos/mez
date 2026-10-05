import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { GuestItemResponse, GuestMenuResponse } from "../../shared/api-types";
import { categoriesWithItems, countText, headingFor, noFilter, offeredDietFilters, photoUrl, problemOf, toggleDiet, visibleItems } from "./menu-logic";

const item = (id: number, name: string, patch: Partial<GuestItemResponse> = {}): GuestItemResponse => ({
  id,
  category_id: 1,
  name,
  price: { minor: 10000, currency: "INR" },
  modifier_groups: [],
  ...patch,
});

const menu: GuestMenuResponse = {
  version: 3,
  currency: "INR",
  categories: [
    { id: 1, name: "Coffee" },
    { id: 2, name: "Plates" },
    { id: 3, name: "Nothing in it" },
  ],
  items: [
    item(1, "Filter coffee", { diet: "veg", description: "Slow and dark." }),
    item(2, "Oat flat white", { diet: "vegan", sold_out: true }),
    item(3, "Masala dosa", { category_id: 2, diet: "vegan", gluten_free: true, description: "Crisp, with potato." }),
    item(4, "Chicken sandwich", { category_id: 2, diet: "non_veg" }),
    item(5, "Egg bhurji", { category_id: 2 }),
  ],
  modifier_groups: [],
};

const names = (items: GuestItemResponse[]) => items.map((it) => it.name);

describe("visibleItems", () => {
  it("shows everything with no filter, what is sold out last", () => {
    expect(names(visibleItems(menu, noFilter))).toEqual(["Filter coffee", "Masala dosa", "Chicken sandwich", "Egg bhurji", "Oat flat white"]);
  });

  it("narrows to a category", () => {
    expect(names(visibleItems(menu, { ...noFilter, category: 2 }))).toEqual(["Masala dosa", "Chicken sandwich", "Egg bhurji"]);
    expect(visibleItems(menu, { ...noFilter, category: 3 })).toEqual([]);
  });

  it.each([
    ["one word in a name", "dosa", ["Masala dosa"]],
    ["whatever the case", "DOSA", ["Masala dosa"]],
    ["part of a word", "sand", ["Chicken sandwich"]],
    ["a word in the description", "dark", ["Filter coffee"]],
    ["every word, in any order", "white oat", ["Oat flat white"]],
    ["one word in the name and one in the description", "dosa potato", ["Masala dosa"]],
    ["a word that is nowhere", "pizza", []],
    ["two words of which one is nowhere", "dosa pizza", []],
    ["nothing but spaces is no search", "   ", ["Filter coffee", "Masala dosa", "Chicken sandwich", "Egg bhurji", "Oat flat white"]],
  ])("searches for %s", (_, search, want) => {
    expect(names(visibleItems(menu, { ...noFilter, search }))).toEqual(want);
  });

  it("counts a vegan dish as vegetarian, and not the other way", () => {
    expect(names(visibleItems(menu, { ...noFilter, diets: ["veg"] }))).toEqual(["Filter coffee", "Masala dosa", "Oat flat white"]);
    expect(names(visibleItems(menu, { ...noFilter, diets: ["vegan"] }))).toEqual(["Masala dosa", "Oat flat white"]);
  });

  it("leaves out a dish that does not say what it is", () => {
    expect(names(visibleItems(menu, { ...noFilter, diets: ["veg"] }))).not.toContain("Egg bhurji");
  });

  it("narrows by every filter together", () => {
    expect(names(visibleItems(menu, { ...noFilter, diets: ["veg", "gluten_free"] }))).toEqual(["Masala dosa"]);
    expect(names(visibleItems(menu, { category: 1, search: "flat", diets: ["vegan"] }))).toEqual(["Oat flat white"]);
    expect(visibleItems(menu, { category: 1, search: "dosa", diets: [] })).toEqual([]);
  });

  it("does not change the menu it was given", () => {
    const before = names(menu.items);
    visibleItems(menu, noFilter);
    expect(names(menu.items)).toEqual(before);
  });
});

describe("what is offered", () => {
  it("offers only the categories with a dish in them", () => {
    expect(categoriesWithItems(menu).map((c) => c.name)).toEqual(["Coffee", "Plates"]);
  });

  it("offers only the diet filters a dish satisfies", () => {
    expect(offeredDietFilters(menu)).toEqual(["veg", "vegan", "gluten_free"]);
    expect(offeredDietFilters({ items: [item(1, "Tea", { diet: "veg" })] })).toEqual(["veg"]);
    expect(offeredDietFilters({ items: [item(1, "Tea")] })).toEqual([]);
  });

  it("switches a diet filter on and off without touching the rest", () => {
    const on = toggleDiet({ category: 2, search: "x", diets: ["veg"] }, "vegan");
    expect(on).toEqual({ category: 2, search: "x", diets: ["veg", "vegan"] });
    expect(toggleDiet(on, "veg").diets).toEqual(["vegan"]);
  });
});

describe("the words over the dishes", () => {
  it("names the search first, then the category, then everything", () => {
    expect(headingFor(menu, noFilter)).toBe("All dishes");
    expect(headingFor(menu, { ...noFilter, category: 2 })).toBe("Plates");
    expect(headingFor(menu, { ...noFilter, category: 2, search: " dosa " })).toBe('Results for "dosa"');
    expect(headingFor(menu, { ...noFilter, category: 99 })).toBe("All dishes");
  });

  it("counts dishes in words", () => {
    expect(countText(0)).toBe("0 dishes");
    expect(countText(1)).toBe("1 dish");
    expect(countText(18)).toBe("18 dishes");
  });

  it("points a photo at the guest's own API", () => {
    expect(photoUrl("ab12.jpg")).toBe("/order/api/photos/ab12.jpg");
    expect(photoUrl("a b/c")).toBe("/order/api/photos/a%20b%2Fc");
  });
});

describe("problemOf", () => {
  it.each([
    ["a code that leads nowhere", new ApiError("table_code_unknown", "x", 404), "unknown_code"],
    ["no network", new ApiError("network", "x"), "unreachable"],
    ["a request that timed out", new ApiError("timeout", "x"), "unreachable"],
    ["the relay saying the venue is offline", new ApiError("bad_response", "x", 503), "unreachable"],
    ["the relay timing out on the venue", new ApiError("bad_response", "x", 504), "unreachable"],
    ["the relay cut off by the venue", new ApiError("bad_response", "x", 502), "unreachable"],
    ["too many requests", new ApiError("rate_limited", "x", 429), "unreachable"],
    ["the server failing", new ApiError("internal", "x", 500), "failed"],
    ["something that is not an API error", new Error("boom"), "failed"],
    ["nothing", null, "failed"],
  ] as const)("reads %s", (_, error, want) => {
    expect(problemOf(error)).toBe(want);
  });
});

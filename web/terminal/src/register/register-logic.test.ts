import { describe, expect, it } from "vitest";
import type { MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { addLineFor, choiceCount, defaultModifiers, fold, searchItems } from "./register-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const item = (id: number, name: string, extra: Partial<MenuItemResponse> = {}): MenuItemResponse => ({
  id, category_id: 1, name, price: inr(18000), station: "bar", course: 1, tax_rate: 5000, modifier_groups: [], ...extra,
});

const group = (min: number, max: number, defaults: boolean[]): ModifierGroupResponse => ({
  id: 1, name: "G", min_select: min, max_select: max,
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
    expect(defaultModifiers([group(1, 1, [true, false]), group(0, 3, [false, false])])).toEqual([
      { modifier_id: 10, name: "m0", price: inr(0) },
    ]);
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
    const line = addLineFor(item(1, "Flat white", { hsn: "996331", course: 2, station: "kitchen" }), [
      { modifier_id: 3, name: "Oat", price: inr(5000) },
    ]);
    expect(line).toEqual({
      item_id: 1, name: "Flat white", qty: 1, unit_price: inr(18000),
      modifiers: [{ modifier_id: 3, name: "Oat", price: inr(5000) }],
      course: 2, station: "kitchen", tax_rate: 5000, hsn: "996331",
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

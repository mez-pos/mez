import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { EditorGroup, EditorItem, MenuEditor } from "../../../shared/api-types";
import {
  addableGroups,
  emptyGroupForm,
  groupById,
  groupFormFrom,
  groupProblemOf,
  groupsChanged,
  newChoice,
  parseChoicePrice,
  ruleText,
  toGroupInput,
  usedBy,
} from "./group-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const milk: EditorGroup = {
  id: 1,
  name: "Milk",
  min_select: 1,
  max_select: 1,
  active: true,
  modifiers: [
    { id: 10, name: "Dairy", price: inr(0), is_default: true },
    { id: 11, name: "Oat", price: inr(4000), is_default: false },
    { id: 12, name: "No milk", price: inr(-1000), is_default: false },
  ],
};
const extras: EditorGroup = { id: 2, name: "Extras", min_select: 0, max_select: 2, active: true, modifiers: [] };
const syrups: EditorGroup = { id: 3, name: "Syrups", min_select: 0, max_select: 1, active: false, modifiers: [] };

const item = (id: number, groups: number[]): EditorItem => ({
  id,
  category_id: 1,
  name: `Item ${id}`,
  price: inr(100),
  hsn: "",
  tax_class_id: 1,
  station_id: 1,
  course_id: 1,
  active: id !== 3,
  modifier_groups: groups,
});

const menu = {
  version: 1,
  currency: "INR",
  needs_hsn: false,
  stations: [],
  courses: [],
  tax_classes: [],
  categories: [],
  items: [item(1, [1, 2]), item(2, [1]), item(3, [1])],
  modifier_groups: [extras, milk, syrups],
} satisfies MenuEditor;

describe("ruleText", () => {
  const tests: [number, number, string][] = [
    [1, 1, "Pick 1"],
    [2, 2, "Pick 2"],
    [0, 2, "Optional, up to 2"],
    [0, 1, "Optional, up to 1"],
    [1, 3, "Pick 1 to 3"],
  ];
  for (const [min, max, want] of tests) {
    it(`${min} to ${max} is "${want}"`, () => expect(ruleText({ min_select: min, max_select: max })).toBe(want));
  }
});

describe("the menu's groups", () => {
  it("counts the items offering a group, retired ones too", () => {
    expect([usedBy(menu, 1), usedBy(menu, 2), usedBy(menu, 3)]).toEqual([3, 1, 0]);
  });
  it("offers only active groups not chosen yet", () => {
    expect(addableGroups(menu, [1]).map((g) => g.name)).toEqual(["Extras"]);
    expect(addableGroups(menu, [1, 2])).toEqual([]);
  });
  it("finds a group by id", () => {
    expect(groupById(menu, 2)?.name).toBe("Extras");
    expect(groupById(menu, 9)).toBeNull();
  });
  it("sees a change of groups, order included", () => {
    expect(groupsChanged(item(1, [1, 2]), [1, 2])).toBe(false);
    expect(groupsChanged(item(1, [1, 2]), [2, 1])).toBe(true);
    expect(groupsChanged(item(1, [1, 2]), [1])).toBe(true);
    expect(groupsChanged(null, [])).toBe(false);
    expect(groupsChanged(null, [1])).toBe(true);
  });
});

describe("parseChoicePrice", () => {
  const tests: [string, number | null][] = [
    ["", 0],
    ["  ", 0],
    ["40", 4000],
    ["40.50", 4050],
    ["-20", -2000],
    ["−20", -2000],
    [" -0.5 ", -50],
    ["--20", null],
    ["20-", null],
    ["abc", null],
    ["1.234", null],
  ];
  for (const [text, want] of tests) {
    it(`reads ${JSON.stringify(text)} as ${want}`, () => expect(parseChoicePrice(text, "INR")).toBe(want));
  }
});

describe("the group form", () => {
  it("starts empty with one blank choice", () => {
    const f = emptyGroupForm();
    expect([f.name, f.min, f.max, f.choices.length]).toEqual(["", "0", "1", 1]);
  });
  it("gives every new row its own key", () => {
    expect(newChoice().key).not.toBe(newChoice().key);
  });
  it("reads a group in and back out as the same request", () => {
    const form = groupFormFrom(milk);
    expect(form.choices.map((c) => c.price)).toEqual(["", "40.00", "-10.00"]);
    expect(toGroupInput(form, "INR")).toEqual({
      input: {
        name: "Milk",
        min_select: 1,
        max_select: 1,
        modifiers: [
          { id: 10, name: "Dairy", price: inr(0), is_default: true },
          { id: 11, name: "Oat", price: inr(4000), is_default: false },
          { id: 12, name: "No milk", price: inr(-1000), is_default: false },
        ],
      },
    });
  });
  it("sends a new row with no id, and trims names", () => {
    const form = { ...emptyGroupForm(), name: " Extras ", choices: [{ ...newChoice(), name: " Shot ", price: "30" }] };
    expect(toGroupInput(form, "INR")).toEqual({
      input: { name: "Extras", min_select: 0, max_select: 1, modifiers: [{ name: "Shot", price: inr(3000), is_default: false }] },
    });
  });
  it("refuses bounds and prices it cannot read, before sending", () => {
    const base = groupFormFrom(milk);
    expect(toGroupInput({ ...base, min: "one" }, "INR")).toMatchObject({ problem: { field: "min_select" } });
    expect(toGroupInput({ ...base, max: "-1" }, "INR")).toMatchObject({ problem: { field: "max_select" } });
    expect(toGroupInput({ ...base, max: "1.5" }, "INR")).toMatchObject({ problem: { field: "max_select" } });
    const badPrice = { ...base, choices: base.choices.map((c, i) => (i === 2 ? { ...c, price: "ten" } : c)) };
    expect(toGroupInput(badPrice, "INR")).toMatchObject({ problem: { field: null, row: { index: 2, field: "price" } } });
  });
});

describe("groupProblemOf", () => {
  const refusal = (field: string | null) => new ApiError("invalid_menu", "The server's words.", 422, field);
  it("puts a choice's refusal on its row", () => {
    expect(groupProblemOf(refusal("modifiers.1.name"))).toEqual({ field: null, row: { index: 1, field: "name" }, message: "The server's words." });
    expect(groupProblemOf(refusal("modifiers.0.id")).row).toEqual({ index: 0, field: "id" });
  });
  it("puts a group's refusal on its field, and anything else over the form", () => {
    expect(groupProblemOf(refusal("min_select")).field).toBe("min_select");
    expect(groupProblemOf(refusal("modifiers")).field).toBe("modifiers");
    expect(groupProblemOf(refusal("ids"))).toEqual({ field: null, message: "The server's words." });
    expect(groupProblemOf(new ApiError("forbidden", "x", 403, "menu.edit")).message).toMatch(/Only someone/);
    expect(groupProblemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});

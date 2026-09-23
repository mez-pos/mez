import { describe, expect, it } from "vitest";
import type { ModifierGroupResponse } from "../../../shared/api-types";
import {
  firstUnmet,
  groupState,
  initialSelection,
  sameModifiers,
  toggle,
  toModifiers,
  unitPrice,
  type Selection,
} from "./modifier-selection";

const inr = (minor: number) => ({ minor, currency: "INR" });

const milk: ModifierGroupResponse = {
  id: 1, name: "Milk", min_select: 1, max_select: 1,
  modifiers: [
    { id: 10, name: "Dairy", price: inr(0), is_default: true },
    { id: 11, name: "Oat", price: inr(5000), is_default: false },
    { id: 12, name: "Almond", price: inr(5000), is_default: false },
  ],
};
const extras: ModifierGroupResponse = {
  id: 2, name: "Extras", min_select: 0, max_select: 2,
  modifiers: [
    { id: 20, name: "Shot", price: inr(3000), is_default: false },
    { id: 21, name: "Vanilla", price: inr(2500), is_default: false },
    { id: 22, name: "Hazelnut", price: inr(2500), is_default: false },
  ],
};
const ice: ModifierGroupResponse = {
  id: 3, name: "Ice cream", min_select: 0, max_select: 1,
  modifiers: [{ id: 30, name: "Scoop", price: inr(6000), is_default: false }],
};
const syrup: ModifierGroupResponse = {
  id: 4, name: "Syrup", min_select: 1, max_select: 1,
  modifiers: [{ id: 40, name: "Maple", price: inr(0), is_default: false }],
};
const groups = [milk, extras];
const pick = (s: Selection, g: ModifierGroupResponse) => s.get(g.id);

describe("initialSelection", () => {
  it("preselects each group's defaults for a new line", () => {
    const s = initialSelection(groups);
    expect(pick(s, milk)).toEqual([10]);
    expect(pick(s, extras)).toEqual([]);
  });

  it("reopens an existing line with what it has, dropping what the menu lost", () => {
    const s = initialSelection(groups, [
      { modifier_id: 11, name: "Oat", price: inr(5000) },
      { modifier_id: 22, name: "Hazelnut", price: inr(2500) },
      { modifier_id: 99, name: "Retired", price: inr(0) },
    ]);
    expect(pick(s, milk)).toEqual([11]);
    expect(pick(s, extras)).toEqual([22]);
  });

  it("reopens a line with no modifiers as empty, not as the defaults", () => {
    expect(pick(initialSelection(groups, []), milk)).toEqual([]);
  });
});

describe("toggle", () => {
  const start = initialSelection(groups);

  it("swaps a pick-one group", () => {
    expect(pick(toggle(start, milk, 11), milk)).toEqual([11]);
  });

  it("keeps a required pick-one choice when tapped again", () => {
    const s = toggle(start, milk, 10);
    expect(s).toBe(start);
  });

  it("clears an optional pick-one choice when tapped again", () => {
    const on = toggle(initialSelection([ice]), ice, 30);
    expect(pick(on, ice)).toEqual([30]);
    expect(pick(toggle(on, ice, 30), ice)).toEqual([]);
  });

  it("adds and removes in a pick-several group, in menu order", () => {
    let s = toggle(start, extras, 22);
    s = toggle(s, extras, 20);
    expect(pick(s, extras)).toEqual([20, 22]);
    expect(pick(toggle(s, extras, 20), extras)).toEqual([22]);
  });

  it("refuses a new pick when the group is full, keeping what was chosen", () => {
    const full = toggle(toggle(start, extras, 20), extras, 21);
    expect(toggle(full, extras, 22)).toBe(full);
  });

  it("never changes the selection it was given", () => {
    toggle(start, milk, 12);
    expect(pick(start, milk)).toEqual([10]);
  });
});

describe("groupState and firstUnmet", () => {
  const cases: [string, ModifierGroupResponse, string][] = [
    ["exactly one", milk, "Choose 1"],
    ["optional", extras, "Up to 2"],
    ["a range", { ...extras, min_select: 2, max_select: 3 }, "Choose 2 to 3"],
  ];
  it.each(cases)("words %s", (_, g, rule) => {
    expect(groupState(g, new Map()).rule).toBe(rule);
  });

  it("knows when a group is met and when it is full", () => {
    const s = toggle(toggle(initialSelection(groups), extras, 20), extras, 21);
    expect(groupState(milk, s)).toMatchObject({ count: 1, satisfied: true, full: true });
    expect(groupState(extras, s)).toMatchObject({ count: 2, satisfied: true, full: true });
  });

  it("names the first required group with no choice", () => {
    expect(firstUnmet([milk, syrup], initialSelection([milk, syrup]))).toBe(syrup);
    expect(firstUnmet(groups, initialSelection(groups))).toBeNull();
  });
});

describe("toModifiers and unitPrice", () => {
  it("lists choices in group then menu order, and prices one of the item", () => {
    let s = initialSelection(groups);
    s = toggle(s, extras, 21);
    s = toggle(s, milk, 11);
    s = toggle(s, extras, 20);
    const mods = toModifiers(groups, s);
    expect(mods.map((m) => m.name)).toEqual(["Oat", "Shot", "Vanilla"]);
    expect(unitPrice(inr(21000), mods)).toEqual(inr(31500));
  });
});

describe("sameModifiers", () => {
  const oat = { modifier_id: 11, name: "Oat", price: inr(5000) };
  const shot = { modifier_id: 20, name: "Shot", price: inr(3000) };
  it("compares the choice, in order", () => {
    expect(sameModifiers([oat, shot], [oat, shot])).toBe(true);
    expect(sameModifiers([oat], [oat, shot])).toBe(false);
    expect(sameModifiers([shot, oat], [oat, shot])).toBe(false);
    expect(sameModifiers(undefined, [])).toBe(true);
  });
});

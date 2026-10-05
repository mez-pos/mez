import { describe, expect, it } from "vitest";
import { arrived, choicesOf, dishCount, pillLabel, waited, wantingTables } from "./request-logic";

const ids = (...list: string[]) => list.map((id) => ({ id }));

describe("arrived", () => {
  it("tells a terminal that has not looked yet of everything waiting", () => {
    expect(arrived(null, ids("a", "b"))).toEqual(["a", "b"]);
  });
  it("names only what was not there at the last look", () => {
    expect(arrived(new Set(["a"]), ids("a", "b", "c"))).toEqual(["b", "c"]);
  });
  it("says nothing when nothing is new, or one was answered", () => {
    expect(arrived(new Set(["a", "b"]), ids("a", "b"))).toEqual([]);
    expect(arrived(new Set(["a", "b"]), ids("b"))).toEqual([]);
    expect(arrived(new Set(), [])).toEqual([]);
  });
});

describe("wantingTables", () => {
  it("is each table once, however many phones asked", () => {
    const tables = wantingTables([{ table_id: 4 }, { table_id: 4 }, { table_id: 7 }]);
    expect([...tables].sort()).toEqual([4, 7]);
    expect(wantingTables([]).size).toBe(0);
  });
});

describe("pillLabel", () => {
  it("agrees with the count", () => {
    expect(pillLabel(1)).toBe("table request");
    expect(pillLabel(2)).toBe("table requests");
  });
});

describe("waited", () => {
  const at = "2026-10-06T10:00:00Z";
  const after = (minutes: number, seconds = 0) => Date.parse(at) + minutes * 60_000 + seconds * 1000;
  const cases: [string, number, string][] = [
    ["under a minute", after(0, 59), "Just now"],
    ["a minute", after(1), "1 min"],
    ["most of an hour", after(59, 59), "59 min"],
    ["an hour", after(60), "1 h"],
    ["an hour and some", after(65), "1 h 5 min"],
    ["a clock set behind the server's", after(-3), "Just now"],
  ];
  it.each(cases)("%s", (_name, now, want) => {
    expect(waited(at, now)).toBe(want);
  });
  it("does not print a time it cannot read", () => {
    expect(waited("not a time", after(5))).toBe("Just now");
  });
});

describe("choicesOf and dishCount", () => {
  it("joins the choices, and is empty with none", () => {
    expect(choicesOf({ choices: ["Oat", "Extra shot"] })).toBe("Oat, Extra shot");
    expect(choicesOf({ choices: [] })).toBe("");
  });
  it("counts every dish, not every line", () => {
    const line = { item_id: 1, name: "x", choices: [], total: { minor: 0, currency: "INR" } };
    expect(
      dishCount({
        lines: [
          { ...line, qty: 2 },
          { ...line, qty: 1 },
        ],
      }),
    ).toBe(3);
  });
});

import { describe, expect, it } from "vitest";
import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { cardMeta, filterCounts, firstName, greeting, groupOf, hourIn, tablesShown } from "./waiter-logic";

const table = (extra: Partial<LiveTableResponse> = {}): LiveTableResponse => ({
  id: 1,
  area_id: 10,
  name: "T1",
  seats: 4,
  shape: "square",
  x: 0,
  y: 0,
  status: "free",
  covers: 0,
  orders: [],
  ...extra,
});

const floor = (tables: LiveTableResponse[]): LiveFloorResponse => ({
  grid: { columns: 6, rows: 4 },
  areas: [
    { id: 10, name: "Main", active: true },
    { id: 20, name: "Terrace", active: true },
  ],
  tables,
});

const evening = [
  table({ id: 1, name: "T1", status: "ordered", covers: 4, orders: ["a"] }),
  table({ id: 2, name: "T2" }),
  table({ id: 3, name: "T10", status: "settling", covers: 5, orders: ["b"] }),
  table({ id: 4, name: "T3", status: "seated", covers: 2, orders: ["c"] }),
  table({ id: 5, name: "P1", area_id: 20, status: "dirty" }),
];

describe("groupOf", () => {
  it("reads a table seated and a table with an order both as occupied", () => {
    expect(groupOf("seated")).toBe("occupied");
    expect(groupOf("ordered")).toBe("occupied");
  });
  it("gives the other three states a group each", () => {
    expect([groupOf("free"), groupOf("settling"), groupOf("dirty")]).toEqual(["available", "bill", "clear"]);
  });
});

describe("filterCounts", () => {
  it("counts every table under All, and each under its group", () => {
    expect(filterCounts(evening)).toEqual({ all: 5, occupied: 2, available: 1, bill: 1, clear: 1 });
  });
  it("counts nothing on an empty floor", () => {
    expect(filterCounts([])).toEqual({ all: 0, occupied: 0, available: 0, bill: 0, clear: 0 });
  });
});

describe("tablesShown", () => {
  const names = (filter: Parameters<typeof tablesShown>[1], query = "") => tablesShown(floor(evening), filter, query).map((t) => t.name);

  it("lists area by area, and within one the way people count", () => {
    expect(names("all")).toEqual(["T1", "T2", "T3", "T10", "P1"]);
  });
  it("keeps one group", () => {
    expect(names("occupied")).toEqual(["T1", "T3"]);
    expect(names("bill")).toEqual(["T10"]);
  });
  it("finds a table by its number or its name, whatever the case or the spaces", () => {
    expect(names("all", "1")).toEqual(["T1", "T10", "P1"]);
    expect(names("all", " t 10 ")).toEqual(["T10"]);
    expect(names("all", "p")).toEqual(["P1"]);
  });
  it("searches within the group picked", () => {
    expect(names("occupied", "1")).toEqual(["T1"]);
    expect(names("available", "9")).toEqual([]);
  });
});

describe("cardMeta", () => {
  const names = new Map([[1, "T1"]]);
  it("says a free table has nobody, and how many it seats", () => {
    expect(cardMeta(table({ seats: 4 }), names)).toBe("No guests · 4 seats");
    expect(cardMeta(table({ seats: 1 }), names)).toBe("No guests · 1 seat");
  });
  it("says a seated table has not ordered", () => {
    expect(cardMeta(table({ status: "seated", covers: 2 }), names)).toBe("2 guests · No order yet");
  });
  it("leaves the balance to be shown as money", () => {
    expect(cardMeta(table({ status: "ordered", covers: 1 }), names)).toBe("1 guest");
    expect(cardMeta(table({ status: "settling", covers: 5 }), names)).toBe("5 guests");
  });
  it("names the party's table for a table joined to it", () => {
    expect(cardMeta(table({ id: 2, status: "ordered", joined_to: 1 }), names)).toBe("With T1");
  });
  it("says a table to clear needs clearing", () => {
    expect(cardMeta(table({ status: "dirty" }), names)).toBe("Needs clearing");
  });
});

describe("greeting", () => {
  it("follows the clock", () => {
    expect(greeting(5)).toEqual({ hello: "Good morning", when: "this morning" });
    expect(greeting(11)).toEqual({ hello: "Good morning", when: "this morning" });
    expect(greeting(12)).toEqual({ hello: "Good afternoon", when: "this afternoon" });
    expect(greeting(16)).toEqual({ hello: "Good afternoon", when: "this afternoon" });
    expect(greeting(17)).toEqual({ hello: "Good evening", when: "tonight" });
    expect(greeting(23)).toEqual({ hello: "Good evening", when: "tonight" });
  });
  it("is still the evening for a service that runs past midnight", () => {
    expect(greeting(0)).toEqual({ hello: "Good evening", when: "tonight" });
    expect(greeting(4)).toEqual({ hello: "Good evening", when: "tonight" });
  });
});

describe("hourIn", () => {
  const at = new Date("2026-10-05T14:45:00Z");
  it("reads the hour in the venue's zone, not the tablet's", () => {
    expect(hourIn(at, "Asia/Kolkata")).toBe(20);
    expect(hourIn(at, "UTC")).toBe(14);
  });
  it("reads midnight as 0", () => {
    expect(hourIn(new Date("2026-10-05T18:30:00Z"), "Asia/Kolkata")).toBe(0);
  });
  it("falls back to the device's clock for a zone the browser does not know", () => {
    expect(hourIn(at, "Mars/Olympus")).toBe(at.getHours());
    expect(hourIn(at, null)).toBe(at.getHours());
  });
});

describe("firstName", () => {
  it("takes the first word of a name", () => {
    expect(firstName("Kabir Singh")).toBe("Kabir");
    expect(firstName("  Kabir ")).toBe("Kabir");
    expect(firstName("")).toBe("");
  });
});

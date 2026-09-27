import { describe, expect, it } from "vitest";
import type { LiveFloorResponse, LiveTableResponse } from "../../../shared/api-types";
import { areaToShow, countsText, startingCovers, tableNames, tapFor, tileText, tilesOf } from "./floor-logic";

const tile = (extra: Partial<LiveTableResponse> = {}): LiveTableResponse => ({
  id: 1, area_id: 10, name: "T1", seats: 4, shape: "square", x: 0, y: 0, status: "free", covers: 0, orders: [], ...extra,
});

const floor = (tables: LiveTableResponse[]): LiveFloorResponse => ({
  grid: { columns: 12, rows: 8 },
  areas: [
    { id: 10, name: "Inside", active: true },
    { id: 20, name: "Terrace", active: true },
  ],
  tables,
});

describe("tilesOf", () => {
  it("keeps the layout but drops the empty edges", () => {
    const rows = tilesOf(floor([tile({ id: 1, x: 2, y: 1 }), tile({ id: 2, x: 4, y: 3 }), tile({ id: 3, x: 0, y: 0, area_id: 20 })]), 10);
    expect(rows.map((r) => r.map((t) => t?.id ?? 0))).toEqual([
      [1, 0, 0],
      [0, 0, 0],
      [0, 0, 2],
    ]);
  });
  it("is empty for an area with no tables", () => {
    expect(tilesOf(floor([tile({ area_id: 20 })]), 10)).toEqual([]);
  });
});

describe("areaToShow", () => {
  it("keeps the chosen area while it is there", () => {
    expect(areaToShow(floor([]), 20)).toBe(20);
    expect(areaToShow(floor([]), 99)).toBe(10);
    expect(areaToShow({ ...floor([]), areas: [] }, null)).toBeNull();
  });
});

describe("countsText", () => {
  it("counts in turn order, in the floor's words", () => {
    const tables = [
      tile({ status: "dirty" }),
      tile({ status: "free" }),
      tile({ status: "settling" }),
      tile({ status: "free" }),
      tile({ status: "seated" }),
    ];
    expect(countsText(tables)).toBe("2 free · 1 seated · 1 paying · 1 to clear");
    expect(countsText([])).toBe("");
  });
});

describe("tapFor", () => {
  const cases: [Partial<LiveTableResponse>, ReturnType<typeof tapFor>][] = [
    [{ status: "free" }, { kind: "seat" }],
    [{ status: "ordered", orders: ["ord_1"] }, { kind: "ticket", orderId: "ord_1" }],
    [{ status: "settling", orders: ["ord_1"] }, { kind: "ticket", orderId: "ord_1" }],
    [{ status: "ordered", orders: ["ord_1", "ord_2"] }, { kind: "sheet" }],
    [{ status: "seated", orders: ["ord_1"] }, { kind: "sheet" }],
    [{ status: "dirty" }, { kind: "sheet" }],
  ];
  it.each(cases)("%o taps to %o", (extra, want) => {
    expect(tapFor(tile(extra))).toEqual(want);
  });
});

describe("tileText", () => {
  const now = Date.parse("2026-09-27T12:42:00Z");
  it("words each state", () => {
    expect(tileText(tile({ seats: 1 }), now)).toEqual({ people: "1 seat", time: null });
    expect(tileText(tile({ status: "ordered", covers: 3, since: "2026-09-27T12:00:00Z" }), now)).toEqual({ people: "3 covers", time: "42m" });
    expect(tileText(tile({ status: "seated", covers: 1, since: "2026-09-27T12:40:00Z" }), now)).toEqual({ people: "1 cover", time: "2m" });
    expect(tileText(tile({ status: "seated", covers: 0, since: "2026-09-27T12:40:00Z" }), now).people).toBe("Seated");
    expect(tileText(tile({ status: "dirty", since: "2026-09-27T12:36:00Z" }), now)).toEqual({ people: "Needs clearing", time: "6m" });
  });
});

describe("small helpers", () => {
  it("starts covers at the table's seats, within 1 to 12", () => {
    expect([0, 1, 4, 30].map(startingCovers)).toEqual([1, 1, 4, 12]);
  });
  it("names tables by id", () => {
    expect(tableNames(floor([tile({ id: 7, name: "Patio 2" })])).get(7)).toBe("Patio 2");
    expect(tableNames(undefined).size).toBe(0);
  });
});

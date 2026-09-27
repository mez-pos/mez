import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { FloorResponse, TableResponse } from "../../../shared/api-types";
import {
  areaToShow,
  cellsOf,
  emptyTableForm,
  moveTo,
  placeText,
  problemOf,
  retiredIn,
  seatsText,
  tableFormFrom,
  toTableSettings,
} from "./floor-logic";

const table = (extra: Partial<TableResponse> = {}): TableResponse => ({
  id: 1, area_id: 10, name: "T1", seats: 4, shape: "round", x: 0, y: 0, active: true, ...extra,
});

const plan = (tables: TableResponse[]): FloorResponse => ({
  grid: { columns: 3, rows: 2 },
  areas: [
    { id: 10, name: "Inside", active: true },
    { id: 20, name: "Terrace", active: true },
  ],
  tables,
});

describe("cellsOf", () => {
  const p = plan([
    table({ id: 1, x: 2, y: 1 }),
    table({ id: 2, name: "T2", x: 0, y: 0, area_id: 20 }),
    table({ id: 3, name: "T3", x: 1, y: 0, active: false }),
  ]);

  it("lays the area out row by row, with only its tables in service", () => {
    const cells = cellsOf(p, 10);
    expect(cells).toHaveLength(2);
    expect(cells[0]).toHaveLength(3);
    expect(cells[1]![2]!.table?.id).toBe(1);
    expect(cells[0]![0]!.table).toBeNull(); // T2 is on the terrace
    expect(cells[0]![1]!.table).toBeNull(); // T3 is retired
    expect(cells[1]![0]).toEqual({ x: 0, y: 1, table: null });
  });

  it("lists the retired tables of an area", () => {
    expect(retiredIn(p, 10).map((t) => t.name)).toEqual(["T3"]);
    expect(retiredIn(p, 20)).toEqual([]);
  });
});

describe("areaToShow", () => {
  it("keeps the chosen area, else the first in service, else none", () => {
    const p = plan([]);
    expect(areaToShow(p, 20)).toBe(20);
    expect(areaToShow(p, 99)).toBe(10);
    expect(areaToShow(p, null)).toBe(10);
    expect(areaToShow({ ...p, areas: [{ id: 5, name: "Old", active: false }, { id: 6, name: "New", active: true }] }, null)).toBe(6);
    expect(areaToShow({ ...p, areas: [{ id: 5, name: "Old", active: false }] }, null)).toBe(5);
    expect(areaToShow({ ...p, areas: [] }, null)).toBeNull();
  });
});

describe("words", () => {
  it("says where a table stands, counting from 1", () => {
    expect(placeText(plan([]), table({ x: 2, y: 1 }))).toBe("Inside, column 3, row 2");
    expect(placeText(plan([]), table({ area_id: 99 }))).toBe("No area, column 1, row 1");
  });
  it("counts seats", () => {
    expect(seatsText(1)).toBe("1 seat");
    expect(seatsText(6)).toBe("6 seats");
  });
});

describe("moveTo", () => {
  const t1 = table({ id: 1, x: 0, y: 0 });
  const t2 = table({ id: 2, name: "T2", x: 1, y: 0 });

  it("moves to a free cell, keeping everything else", () => {
    expect(moveTo(t1, { x: 2, y: 1, table: null }, 20)).toEqual({
      kind: "move",
      settings: { area_id: 20, name: "T1", seats: 4, shape: "round", x: 2, y: 1 },
    });
  });
  it("does nothing on its own cell", () => {
    expect(moveTo(t1, { x: 0, y: 0, table: t1 }, 10)).toEqual({ kind: "none" });
  });
  it("refuses a taken cell, or no cell, in the server's words", () => {
    expect(moveTo(t1, { x: 1, y: 0, table: t2 }, 10)).toEqual({
      kind: "refused",
      message: "Table T2 is already at column 2, row 1.",
    });
    expect(moveTo(t1, null, 10)).toEqual({ kind: "refused", message: "Drop T1 on a free cell." });
  });
});

describe("the table form", () => {
  it("starts at the cell tapped, counting from 1", () => {
    expect(emptyTableForm(10, 4, 2)).toEqual({ name: "", seats: "2", shape: "square", areaId: 10, column: "5", row: "3" });
  });
  it("round-trips a table", () => {
    const t = table({ x: 5, y: 6, seats: 8, shape: "long" });
    expect(toTableSettings(tableFormFrom(t))).toEqual({ area_id: 10, name: "T1", seats: 8, shape: "long", x: 5, y: 6 });
  });
  const bad: [string, Partial<ReturnType<typeof emptyTableForm>>, "seats" | "x" | "y"][] = [
    ["empty seats", { seats: "" }, "seats"],
    ["fractional seats", { seats: "2.5" }, "seats"],
    ["a word for a column", { column: "left" }, "x"],
    ["column 0", { column: "0" }, "x"],
    ["a negative row", { row: "-3" }, "y"],
  ];
  it.each(bad)("sends %s as -1, for the server to refuse", (_, patch, key) => {
    const s = toTableSettings({ ...emptyTableForm(10), name: " T9 ", ...patch });
    expect(s.name).toBe("T9");
    expect(s[key]).toBe(-1);
  });
});

describe("problemOf", () => {
  it("puts a refusal under its field", () => {
    expect(problemOf(new ApiError("cell_taken", "Table T2 is already there.", 409, "x"))).toEqual({
      field: "x",
      message: "Table T2 is already there.",
    });
    expect(problemOf(new ApiError("area_in_use", "Retire the tables first.", 409))).toEqual({
      field: null,
      message: "Retire the tables first.",
    });
  });
  it("words the network and permissions", () => {
    expect(problemOf(new ApiError("network", "x")).message).toMatch(/Can't reach/);
    expect(problemOf(new ApiError("forbidden", "x", 403)).message).toMatch(/Only someone/);
    expect(problemOf(new Error("boom")).message).toBe("Something went wrong. Try again.");
  });
});

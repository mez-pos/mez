import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { EditorItem, MenuEditor } from "../../../shared/api-types";
import {
  MAX_NAME,
  courseRetiredText,
  courseRows,
  destinations,
  itemsText,
  nameProblem,
  retireProblem,
  setupProblemOf,
  stationRetiredText,
  stationRows,
  type SetupRow,
} from "./setup-logic";

const item = (id: number, station: number, course: number, active = true): EditorItem =>
  ({ id, station_id: station, course_id: course, active, name: `Item ${id}` }) as EditorItem;

const menu = {
  stations: [
    { id: 1, code: "kitchen", name: "Kitchen", active: true },
    { id: 2, code: "bar", name: "Bar", active: true },
    { id: 3, code: "pastry", name: "Pastry", active: false },
  ],
  courses: [
    { id: 10, number: 1, name: "Starters", active: true },
    { id: 11, number: 2, name: "Mains", active: true },
    { id: 12, number: 3, name: "Cheese", active: false },
  ],
  items: [item(1, 1, 10), item(2, 1, 11), item(3, 2, 10), item(4, 2, 10, false), item(5, 3, 12, false)],
} as unknown as MenuEditor;

describe("the rows", () => {
  it("name each station with its code and every item made there, retired items too", () => {
    expect(stationRows(menu)).toEqual([
      { id: 1, name: "Kitchen", tag: "kitchen", active: true, items: 2 },
      { id: 2, name: "Bar", tag: "bar", active: true, items: 2 },
      { id: 3, name: "Pastry", tag: "pastry", active: false, items: 1 },
    ]);
  });
  it("name each course with its number and its items", () => {
    expect(courseRows(menu)).toEqual([
      { id: 10, name: "Starters", tag: "Course 1", active: true, items: 3 },
      { id: 11, name: "Mains", tag: "Course 2", active: true, items: 1 },
      { id: 12, name: "Cheese", tag: "Course 3", active: false, items: 1 },
    ]);
  });
  it("keep the server's order", () => {
    expect(stationRows(menu).map((r) => r.id)).toEqual([1, 2, 3]);
    expect(courseRows(menu).map((r) => r.id)).toEqual([10, 11, 12]);
  });
});

describe("where a retired row's items can go", () => {
  const rows = stationRows(menu);
  it("is any other row in service", () => {
    expect(destinations(rows, 1).map((r) => r.id)).toEqual([2]);
    expect(destinations(rows, 2).map((r) => r.id)).toEqual([1]);
  });
  it("is never a retired row, and never itself", () => {
    expect(destinations(rows, 3).map((r) => r.id)).toEqual([1, 2]);
    expect(destinations(rows, 99).map((r) => r.id)).toEqual([1, 2]);
  });
  it("is nowhere when it is the only one in service", () => {
    expect(destinations([{ id: 1, name: "K", tag: "k", active: true, items: 0 }], 1)).toEqual([]);
  });
});

describe("words for counts", () => {
  it("say none, one and many", () => {
    expect(itemsText(0)).toBe("No items");
    expect(itemsText(1)).toBe("1 item");
    expect(itemsText(7)).toBe("7 items");
  });
});

describe("retiring", () => {
  const withItems: SetupRow = { id: 1, name: "Kitchen", tag: "kitchen", active: true, items: 3 };
  const withOne: SetupRow = { ...withItems, items: 1 };
  const empty: SetupRow = { ...withItems, items: 0 };
  it("asks where items go when there are any", () => {
    expect(retireProblem(withItems, 0, "station")).toEqual({ field: "move_to", message: "Choose the station its items go to." });
    expect(retireProblem(withOne, 0, "course")).toEqual({ field: "move_to", message: "Choose the course its item goes to." });
    expect(retireProblem(withItems, 2, "station")).toBeNull();
  });
  it("asks nothing of a row that carries nothing", () => {
    expect(retireProblem(empty, 0, "station")).toBeNull();
  });
  it("says what it moved", () => {
    expect(stationRetiredText("Bar", { name: "Kitchen" }, { id: 2, items_moved: 7, printer_moved: true })).toBe(
      "Bar retired. 7 items moved to Kitchen. Its printer now serves Kitchen.",
    );
    expect(stationRetiredText("Bar", { name: "Kitchen" }, { id: 2, items_moved: 1, printer_moved: false })).toBe(
      "Bar retired. 1 item moved to Kitchen.",
    );
    expect(stationRetiredText("Pastry", null, { id: 3, items_moved: 0, printer_moved: false })).toBe("Pastry retired.");
    expect(courseRetiredText("Cheese", { name: "Mains" }, { id: 12, items_moved: 4 })).toBe("Cheese retired. 4 items moved to Mains.");
    expect(courseRetiredText("Cheese", null, { id: 12, items_moved: 0 })).toBe("Cheese retired.");
  });
});

describe("a name", () => {
  it("is refused when empty, or too long, counting characters", () => {
    expect(nameProblem("")).toEqual({ field: "name", message: "Give it a name." });
    expect(nameProblem("   ")?.field).toBe("name");
    expect(nameProblem("a".repeat(MAX_NAME + 1))?.field).toBe("name");
    expect(nameProblem("a".repeat(MAX_NAME))).toBeNull();
    expect(nameProblem("स".repeat(MAX_NAME))).toBeNull(); // 60 characters, 180 bytes
    expect(nameProblem("Juice bar")).toBeNull();
  });
});

describe("a failed request", () => {
  it("puts the server's words under the field they are about", () => {
    expect(setupProblemOf(new ApiError("invalid_menu", "A station already uses the code bar.", 422, "code"))).toEqual({
      field: "code",
      message: "A station already uses the code bar.",
    });
    expect(setupProblemOf(new ApiError("invalid_menu", "Choose the station its items and printer go to.", 422, "move_to")).field).toBe("move_to");
    expect(setupProblemOf(new ApiError("invalid_menu", "Give it a name.", 422, "name")).field).toBe("name");
  });
  it("puts a refusal about no field of the form in the footer", () => {
    expect(setupProblemOf(new ApiError("invalid_menu", "This is the only station in service.", 422, "id")).field).toBeNull();
    expect(setupProblemOf(new ApiError("tickets_open", "Tickets are still open.", 409)).field).toBeNull();
    expect(setupProblemOf(new ApiError("station_not_found", "No such station.", 404)).message).toBe("No such station.");
  });
});

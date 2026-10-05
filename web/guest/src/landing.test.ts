import { describe, expect, it } from "vitest";
import { tableLabel } from "./Landing";

describe("tableLabel", () => {
  it.each([
    ["T4", "Table T4"],
    ["12", "Table 12"],
    ["Window", "Table Window"],
    ["Table 7", "Table 7"],
    ["table 7", "table 7"],
    ["Window table", "Window table"],
    ["Tablet corner", "Table Tablet corner"],
  ])("calls a table named %s %s", (name, want) => {
    expect(tableLabel(name)).toBe(want);
  });
});

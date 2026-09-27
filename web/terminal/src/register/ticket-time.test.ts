import { describe, expect, it } from "vitest";
import { elapsed, ticketLabel } from "./ticket-time";

const opened = "2026-09-23T10:00:00Z";
const at = (minutes: number, seconds = 0) => Date.parse(opened) + minutes * 60_000 + seconds * 1000;

describe("elapsed", () => {
  it.each([
    [0, 0, "0m", false],
    [4, 59, "4m", false],
    [19, 59, "19m", false],
    [20, 0, "20m", true],
    [65, 0, "1h 05m", true],
    [600, 0, "10h 00m", true],
  ])("%im %is is %s, late %s", (m, s, text, late) => {
    expect(elapsed(opened, at(m, s), 20)).toEqual({ text, late });
  });

  it("never goes negative when the till's clock is behind the server's", () => {
    expect(elapsed(opened, at(-3), 20)).toEqual({ text: "0m", late: false });
  });

  it("is never late with no threshold, and survives a bad timestamp", () => {
    expect(elapsed(opened, at(500), 0).late).toBe(false);
    expect(elapsed("not a time", at(5), 20)).toEqual({ text: "0m", late: false });
  });
});

describe("ticketLabel", () => {
  it.each([
    [{ covers: 0 }, "Counter"],
    [{ table_id: 4, covers: 2 }, "Table 4 · 2 covers"],
    [{ table_id: 12, covers: 1 }, "Table 12 · 1 cover"],
    [{ covers: 3 }, "Counter · 3 covers"],
  ])("%o is %s", (t, label) => {
    expect(ticketLabel(t)).toBe(label);
  });
  it("names the table from the floor when it can", () => {
    const names = new Map([[4, "T4"]]);
    expect(ticketLabel({ table_id: 4, covers: 2 }, names)).toBe("Table T4 · 2 covers");
    expect(ticketLabel({ table_id: 5, covers: 0 }, names)).toBe("Table 5");
  });
});

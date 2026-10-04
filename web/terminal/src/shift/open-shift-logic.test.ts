import { describe, expect, it } from "vitest";
import { joinNames, setFaceCount, soldOutNames, soldOutNotice } from "./open-shift-logic";
import { maxPieces } from "./shift-logic";

describe("setFaceCount", () => {
  const tests = [
    { name: "a count typed", text: "12", want: 12 },
    { name: "an empty box is none", text: "", want: 0 },
    { name: "letters and signs are dropped", text: "1a2-", want: 12 },
    { name: "only letters is none", text: "abc", want: 0 },
    { name: "leading zeros", text: "007", want: 7 },
    { name: "spaces from a paste", text: " 4 5 ", want: 45 },
    { name: "a decimal point is dropped, not read as a fraction", text: "2.5", want: 25 },
    { name: "stops at the ceiling", text: "9999999999", want: maxPieces },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(setFaceCount({}, 50000, tt.text)[50000]).toBe(tt.want);
    });
  }

  it("leaves the other notes and does not change the tally it was given", () => {
    const before = { 50000: 2, 1000: 3 };
    const after = setFaceCount(before, 1000, "9");
    expect(after).toEqual({ 50000: 2, 1000: 9 });
    expect(before).toEqual({ 50000: 2, 1000: 3 });
  });
});

describe("soldOutNames", () => {
  const now = new Date("2026-10-04T10:00:00Z");
  const items = [
    { name: "Masala Chai" },
    { name: "French Fries", sold_out_until: "2026-10-05T00:00:00Z" },
    { name: "Cold Coffee", sold_out_until: "2026-10-04T09:00:00Z" },
    { name: "Brownie", sold_out_until: "2026-10-06T00:00:00Z" },
    { name: "Odd", sold_out_until: "not a time" },
  ];
  it("names what is sold out now, in order, and not what is back or unreadable", () => {
    expect(soldOutNames(items, now)).toEqual(["French Fries", "Brownie"]);
  });
  it("is empty for an empty menu", () => {
    expect(soldOutNames([], now)).toEqual([]);
  });
});

describe("soldOutNotice", () => {
  it("says nothing when nothing is sold out", () => {
    expect(soldOutNotice([])).toBeNull();
  });
  it("is singular for one", () => {
    expect(soldOutNotice(["French Fries"])).toEqual({
      title: "1 item is sold out",
      body: "French Fries is sold out and can't be ordered until back on sale.",
    });
  });
  it("names two and three", () => {
    expect(soldOutNotice(["A", "B"])?.body).toBe("A and B are sold out and can't be ordered until back on sale.");
    expect(soldOutNotice(["A", "B", "C"])?.body).toBe("A, B and C are sold out and can't be ordered until back on sale.");
  });
  it("names three and counts the rest", () => {
    expect(soldOutNotice(["A", "B", "C", "D", "E"])).toEqual({
      title: "5 items are sold out",
      body: "A, B, C and 2 more are sold out and can't be ordered until back on sale.",
    });
  });
});

describe("joinNames", () => {
  it("joins the way a person says it", () => {
    expect(joinNames([])).toBe("");
    expect(joinNames(["Arjun"])).toBe("Arjun");
    expect(joinNames(["Arjun", "Priya"])).toBe("Arjun and Priya");
    expect(joinNames(["Arjun", "Priya", "Meera"])).toBe("Arjun, Priya and Meera");
  });
});

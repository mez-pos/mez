import { describe, expect, it } from "vitest";
import { majorText, parseMajor } from "./money-text";

describe("an amount as it is spoken", () => {
  it("is written with the currency's decimals", () => {
    expect(majorText(50, 100)).toBe("0.50");
    expect(majorText(0, 100)).toBe("0.00");
    expect(majorText(100, 100)).toBe("1.00");
    expect(majorText(123456, 100)).toBe("1234.56");
    expect(majorText(5, 100)).toBe("0.05");
    expect(majorText(7, 1)).toBe("7"); // a currency with no minor unit
    expect(majorText(10, 1000)).toBe("0.010"); // three decimals
  });
  it("is read from what is typed", () => {
    expect(parseMajor("0.50", 100)).toBe(50);
    expect(parseMajor("0.5", 100)).toBe(50);
    expect(parseMajor("5", 100)).toBe(500);
    expect(parseMajor(" 12.34 ", 100)).toBe(1234);
    expect(parseMajor("0", 100)).toBe(0);
    expect(parseMajor("0.00", 100)).toBe(0);
    expect(parseMajor("7", 1)).toBe(7);
    expect(parseMajor("0.001", 1000)).toBe(1);
  });
  it("is refused when it is not an amount, or has more decimals than the currency", () => {
    for (const bad of ["", " ", "abc", "-1", "1.234", "1,5", ".5", "1.", "1e2", "NaN", "1 2"]) {
      expect(parseMajor(bad, 100), bad).toBeNull();
    }
    expect(parseMajor("0.5", 1)).toBeNull(); // no decimals where there is no minor unit
    expect(parseMajor("1.5", 1)).toBeNull();
    expect(parseMajor("1234567890", 100)).toBeNull(); // past nine digits
  });
  it("reads back what it wrote", () => {
    for (const per of [1, 100, 1000]) {
      for (const minor of [0, 1, 5, 50, 99, 100, 12345]) expect(parseMajor(majorText(minor, per), per)).toBe(minor);
    }
  });
});

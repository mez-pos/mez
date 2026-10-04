import { describe, expect, it } from "vitest";
import { formatPairingCode, formatWait, initials, isCompletePairingCode, roleText } from "./format";

describe("formatPairingCode", () => {
  const tests = [
    { input: "mw4c", want: "MW4C" },
    { input: "mw4ch", want: "MW4C-H" },
    { input: "mw4chhwy", want: "MW4C-HHWY" },
    { input: "MW4C-HHWY", want: "MW4C-HHWY" },
    { input: " mw4c hhwy ", want: "MW4C-HHWY" },
    { input: "mw4c-hhwy-extra", want: "MW4C-HHWY" },
    { input: "", want: "" },
  ];
  for (const tt of tests) {
    it(JSON.stringify(tt.input), () => expect(formatPairingCode(tt.input)).toBe(tt.want));
  }

  it("knows a whole code", () => {
    expect(isCompletePairingCode("MW4C-HHWY")).toBe(true);
    expect(isCompletePairingCode("MW4C-HHW")).toBe(false);
  });
});

describe("initials", () => {
  const tests = [
    { name: "Asha Kumar", want: "AK" },
    { name: "Owner", want: "O" },
    { name: "  ravi   m  ", want: "RM" },
    { name: "Mary Anne de la Cruz", want: "MC" },
    { name: "", want: "" },
  ];
  for (const tt of tests) {
    it(JSON.stringify(tt.name), () => expect(initials(tt.name)).toBe(tt.want));
  }
});

describe("formatWait", () => {
  const tests = [
    { ms: 300_000, want: "5:00" },
    { ms: 299_001, want: "5:00" },
    { ms: 61_000, want: "1:01" },
    { ms: 999, want: "0:01" },
    { ms: 0, want: "0:00" },
    { ms: -50, want: "0:00" },
  ];
  for (const tt of tests) {
    it(`${tt.ms}ms`, () => expect(formatWait(tt.ms)).toBe(tt.want));
  }
});

describe("roleText", () => {
  it("capitalises the first letter and leaves the rest", () => {
    expect(roleText("manager")).toBe("Manager");
    expect(roleText("head chef")).toBe("Head chef");
    expect(roleText("  ")).toBe("");
  });
});

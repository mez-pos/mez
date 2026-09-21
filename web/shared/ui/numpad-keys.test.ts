import { describe, expect, it } from "vitest";
import { applyAmountKey, applyPinKey, keyFromKeyboard, type NumKey } from "./numpad-keys";

describe("applyAmountKey", () => {
  const max = 99_999_99; // ₹99,999.99
  const tests: { name: string; minor: number; keys: NumKey[]; want: number }[] = [
    { name: "digits shift in from the right", minor: 0, keys: ["1", "2", "5", "0"], want: 1250 },
    { name: "double zero", minor: 0, keys: ["4", "00"], want: 400 },
    { name: "leading zeros do nothing", minor: 0, keys: ["0", "0", "7"], want: 7 },
    { name: "back drops the last digit", minor: 1250, keys: ["back"], want: 125 },
    { name: "back at zero stays zero", minor: 0, keys: ["back"], want: 0 },
    { name: "clear", minor: 1250, keys: ["clear"], want: 0 },
    { name: "a digit past the maximum is ignored", minor: 99_999_99, keys: ["1"], want: 99_999_99 },
    { name: "double zero past the maximum is ignored", minor: 1_000_000, keys: ["00"], want: 1_000_000 },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(tt.keys.reduce((minor, key) => applyAmountKey(minor, key, max), tt.minor)).toBe(tt.want);
    });
  }

  it("always yields an integer", () => {
    let minor = 0;
    for (const key of ["9", "back", "3", "00", "back", "7"] as NumKey[]) {
      minor = applyAmountKey(minor, key, max);
      expect(Number.isInteger(minor)).toBe(true);
    }
  });
});

describe("applyPinKey", () => {
  const tests: { name: string; pin: string; keys: NumKey[]; want: string }[] = [
    { name: "appends", pin: "", keys: ["2", "4", "6", "8"], want: "2468" },
    { name: "stops at the maximum", pin: "1234", keys: ["5", "6"], want: "12345" },
    { name: "back", pin: "246", keys: ["back"], want: "24" },
    { name: "clear", pin: "246", keys: ["clear"], want: "" },
    { name: "double zero is not a PIN key", pin: "1", keys: ["00"], want: "1" },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(tt.keys.reduce((pin, key) => applyPinKey(pin, key, 5), tt.pin)).toBe(tt.want);
    });
  }
});

describe("keyFromKeyboard", () => {
  const tests = [
    { key: "7", want: "7" },
    { key: "Backspace", want: "back" },
    { key: "Delete", want: "clear" },
    { key: "Escape", want: null },
    { key: "a", want: null },
    { key: "Enter", want: null },
  ];
  for (const tt of tests) {
    it(tt.key, () => expect(keyFromKeyboard(tt.key)).toBe(tt.want));
  }
});

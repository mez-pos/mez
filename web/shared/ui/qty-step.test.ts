import { describe, expect, it } from "vitest";
import { canStep, stepQty } from "./qty-step";

describe("stepQty", () => {
  const bounds = { min: 1, max: 5 };
  const tests = [
    { name: "up", value: 2, step: 1 as const, want: 3 },
    { name: "down", value: 2, step: -1 as const, want: 1 },
    { name: "never below the minimum", value: 1, step: -1 as const, want: 1 },
    { name: "never above the maximum", value: 5, step: 1 as const, want: 5 },
    { name: "a fraction is truncated first", value: 2.7, step: 1 as const, want: 3 },
    { name: "an out-of-range value is pulled back in", value: 9, step: -1 as const, want: 5 },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(stepQty(tt.value, tt.step, bounds)).toBe(tt.want));
  }

  it("says when a step would do nothing, so the button can be disabled", () => {
    expect(canStep(1, -1, bounds)).toBe(false);
    expect(canStep(5, 1, bounds)).toBe(false);
    expect(canStep(3, 1, bounds)).toBe(true);
  });

  it("defaults to one through 999", () => {
    expect(stepQty(1, -1)).toBe(1);
    expect(stepQty(999, 1)).toBe(999);
  });
});

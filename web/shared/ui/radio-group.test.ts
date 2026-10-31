import { describe, expect, it } from "vitest";
import { radioStep } from "./radio-group";

describe("radioStep (TECH-20)", () => {
  it("moves on with Right and Down, back with Left and Up, wrapping", () => {
    expect(radioStep("ArrowRight", 0, 3)).toBe(1);
    expect(radioStep("ArrowDown", 2, 3)).toBe(0);
    expect(radioStep("ArrowLeft", 0, 3)).toBe(2);
    expect(radioStep("ArrowUp", 1, 3)).toBe(0);
  });

  it("goes to the ends with Home and End", () => {
    expect(radioStep("Home", 2, 4)).toBe(0);
    expect(radioStep("End", 0, 4)).toBe(3);
  });

  it("passes over a radio that cannot be chosen", () => {
    const off = (i: number) => i === 1;
    expect(radioStep("ArrowRight", 0, 3, off)).toBe(2);
    expect(radioStep("ArrowLeft", 2, 3, off)).toBe(0);
    expect(radioStep("Home", 2, 3, (i) => i === 0)).toBe(1);
    expect(radioStep("End", 0, 3, (i) => i === 2)).toBe(1);
  });

  it("starts from the first or the last when nothing is chosen", () => {
    expect(radioStep("ArrowRight", -1, 3)).toBe(0);
    expect(radioStep("ArrowLeft", -1, 3)).toBe(2);
  });

  it("does nothing for another key, or with nowhere else to go", () => {
    expect(radioStep("Enter", 0, 3)).toBeNull();
    expect(radioStep("a", 0, 3)).toBeNull();
    expect(radioStep("ArrowRight", 0, 1)).toBeNull();
    expect(radioStep("ArrowRight", 0, 3, (i) => i !== 0)).toBeNull();
    expect(radioStep("Home", 0, 3)).toBeNull();
    expect(radioStep("ArrowRight", 0, 0)).toBeNull();
  });
});

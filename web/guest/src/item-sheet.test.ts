import { describe, expect, it } from "vitest";
import { deltaText } from "./ItemSheet";

describe("deltaText", () => {
  it.each([
    [0, ""],
    [4000, "+₹40.00"],
    [50, "+₹0.50"],
    [-1000, "−₹10.00"],
  ])("writes a choice costing %d as %s", (minor, want) => {
    expect(deltaText({ minor, currency: "INR" })).toBe(want);
  });
});

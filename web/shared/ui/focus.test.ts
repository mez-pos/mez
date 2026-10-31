import { describe, expect, it } from "vitest";
import { focusButton, focusField, selectedRing } from "./focus";

// Every source file of the three apps and the shared kit, as text.
const sources = import.meta.glob(
  ["../../terminal/src/**/*.{ts,tsx}", "../../backoffice/src/**/*.{ts,tsx}", "../../shared/**/*.{ts,tsx}", "../../guest/src/**/*.{ts,tsx}", "!**/*.test.*"],
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

const where = (pattern: RegExp): string[] =>
  Object.entries(sources).flatMap(([file, text]) =>
    text.split("\n").flatMap((line, i) => (pattern.test(line) ? [`${file.replace(/^\.\.\/\.\.\//, "")}:${i + 1}`] : [])),
  );

describe("the one focus rule (TECH-12, ADR-403)", () => {
  it("reads every app's sources", () => {
    expect(Object.keys(sources).length).toBeGreaterThan(200);
  });

  it("is a 3px outline in the focus colour on a button, a green border and halo on a field", () => {
    expect(focusButton).toBe("outline-offset-2 outline-focus focus-visible:outline-3");
    expect(focusField).toBe("outline-accent-tint focus-visible:border-accent focus-visible:outline-3");
  });

  it("has no second focus colour: the old green line and the bare terracotta are gone", () => {
    expect(where(/\boutline-(line-focus|attention)\b/)).toEqual([]);
  });

  it("draws every focus outline 3px wide", () => {
    // A focus variant, then any stacked ones (after:, hover:), then the width.
    expect(where(/(focus-visible|focus-within|has-focus-visible)(:[a-z-]+)*:outline-(?!3\b|offset|focus|current|accent|none)[\w-]+/)).toEqual([]);
  });

  it("never draws a choice as focus: selection is a ring, not an outline", () => {
    expect(selectedRing).not.toMatch(/outline/);
    expect(where(/&& "outline-\d/)).toEqual([]);
  });
});

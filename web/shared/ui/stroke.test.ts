import { describe, expect, it } from "vitest";
import { stroke } from "./stroke";

// Every source file of the three apps and the shared kit, as text.
const sources = import.meta.glob(
  ["../../terminal/src/**/*.tsx", "../../backoffice/src/**/*.tsx", "../../shared/**/*.tsx", "../../guest/src/**/*.tsx", "!**/*.test.*"],
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

describe("icon strokes (TECH-22, ADR-405)", () => {
  it("are the design's line, and three others each for a reason", () => {
    expect(stroke).toEqual({ base: 1.75, hairline: 1, large: 1.5, bold: 2.25 });
  });

  it("are never written as a number: every stroke is the kit's", () => {
    const numbers = Object.entries(sources).flatMap(([file, text]) =>
      text.split("\n").flatMap((line, i) => (/strokeWidth=\{[^}]*\d/.test(line) ? [`${file.replace(/^\.\.\/\.\.\//, "")}:${i + 1}`] : [])),
    );
    expect(numbers).toEqual([]);
  });
});

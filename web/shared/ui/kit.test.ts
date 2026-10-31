import { describe, expect, it } from "vitest";
import { buttonClass } from "./Button";
import { fieldClass, inputClass } from "./field";
import { focusField } from "./focus";
import { pillClass } from "./pill";

// Every source file of the three apps and the shared kit, as text.
const sources = import.meta.glob(
  ["../../terminal/src/**/*.{ts,tsx}", "../../backoffice/src/**/*.{ts,tsx}", "../../shared/**/*.{ts,tsx}", "../../guest/src/**/*.{ts,tsx}", "!**/*.test.*"],
  { query: "?raw", import: "default", eager: true },
) as Record<string, string>;

/** A source's path from web/: the glob names this folder's own files "./x". */
const named = (file: string): string => (file.startsWith("./") ? `shared/ui/${file.slice(2)}` : file.replace(/^\.\.\/\.\.\//, ""));

/** Where a pattern is, outside the kit's own file for it. */
const outside = (pattern: RegExp, kitFile: string): string[] =>
  Object.entries(sources).flatMap(([file, text]) =>
    named(file) === kitFile ? [] : text.split("\n").flatMap((line, i) => (pattern.test(line) ? [`${named(file)}:${i + 1}`] : [])),
  );

describe("one of each in the kit (TECH-23, ADR-406)", () => {
  it("one input: the field's look is written once", () => {
    expect(fieldClass).toContain(focusField);
    expect(inputClass).toContain(fieldClass);
    expect(outside(/border-line-strong bg-raised[^"]*outline-accent-tint/, "shared/ui/field.ts")).toEqual([]);
  });

  it("one switch", () => {
    expect(outside(/role="switch"/, "shared/ui/Switch.tsx")).toEqual([]);
  });

  it("one keypad", () => {
    expect(outside(/\["7", "8", "9"\]/, "shared/ui/NumPad.tsx")).toEqual([]);
  });

  it("one pill, at the back office's size or the till's, green or terracotta when chosen", () => {
    expect(pillClass(false)).toContain("min-h-10");
    expect(pillClass(true, { size: "tap" })).toMatch(/min-h-tap.*bg-accent/);
    expect(pillClass(true, { size: "tap", chosen: "attention" })).toContain("bg-attention");
  });

  it("one pill button: a link that must look like a Button wears its classes", () => {
    expect(buttonClass()).toContain("border-line-strong");
    expect(buttonClass("secondary", null)).not.toContain("min-h-tap");
  });
});

// The paper surface's text inks hold WCAG AA (4.5:1) on every light ground
// they are written on (ADR-388, TECH-5, TECH-6, TECH-7). Read from tokens.css
// itself, so a colour changed there is checked here.

import { describe, expect, it } from "vitest";
import css from "./tokens.css?raw";

/** The declarations of one block: the primitives in :root and the paper surface's roles. */
function block(selector: string): Map<string, string> {
  const start = css.indexOf(`${selector} {`);
  const body = css.slice(start, css.indexOf("\n}", start));
  return new Map([...body.matchAll(/--([a-z0-9-]+):\s*([^;]+);/g)].map((m) => [m[1]!, m[2]!.trim()]));
}

const primitives = block(":root");
const paper = block('[data-surface="paper"]');

/** A token as a hex colour, following var() through the paper roles and the primitives. */
function hex(name: string): string {
  let value = paper.get(name) ?? primitives.get(name);
  for (let hops = 0; value?.startsWith("var(") && hops < 5; hops++) {
    const next = value.slice(6, -1);
    value = paper.get(next) ?? primitives.get(next);
  }
  if (!value?.startsWith("#")) throw new Error(`--${name} is not a colour: ${value}`);
  return value;
}

function luminance(colour: string): number {
  const [r, g, b] = (colour.slice(1).match(/../g) ?? []).map((h) => {
    const v = parseInt(h, 16) / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

// Every light ground a sentence is written on: the page, cards, wells, the tints and the chip fills.
const grounds = ["canvas", "raised", "raised-high", "sunken", "accent-tint", "attention-tint", "chip-neutral-bg", "chip-warning-bg", "chip-success-bg"];

describe("the paper surface's text", () => {
  it.each(["fg-muted", "fg-3", "fg-faint", "attention-text", "fg-2"])("--%s holds 4.5:1 on every light ground", (ink) => {
    for (const ground of grounds) {
      expect(contrast(hex(ink), hex(ground)), `--${ink} on --${ground}`).toBeGreaterThanOrEqual(4.5);
    }
  });

  it("is muted where it was faint: one ink for the quietest text", () => {
    expect(hex("fg-faint")).toBe(hex("fg-muted"));
  });
});

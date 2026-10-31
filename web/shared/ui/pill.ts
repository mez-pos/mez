// A toggle or tab drawn as a pill: the chosen one is filled. For filters and
// view switches; an option inside a form is a tinted card instead. One for
// every app (TECH-23, ADR-406): the till's area tabs and a waiter's table
// pills were copies.

import { cx } from "./cx";
import { focusButton } from "./focus";

const base = `flex items-center gap-2 rounded-pill border text-base ${focusButton}`;

const sizes = {
  /** 40px, the back office's. */
  md: "min-h-10 px-4",
  /** 48px, the touch floor: the till's. */
  tap: "min-h-tap px-5",
} as const;

const fills = {
  /** Green, a view or a filter. */
  accent: "border-accent bg-accent font-semibold text-accent-fg",
  /** Terracotta, where the screen's own choices are (the till's areas, terminal-floor.md). */
  attention: "border-attention bg-attention font-semibold text-attention-fg",
} as const;

export interface PillOptions {
  size?: keyof typeof sizes;
  chosen?: keyof typeof fills;
}

export function pillClass(on: boolean, { size = "md", chosen = "accent" }: PillOptions = {}): string {
  return cx(base, sizes[size], on ? fills[chosen] : "border-line-strong bg-raised text-fg hover:bg-raised-high");
}

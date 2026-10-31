// A toggle or tab drawn as a pill: the chosen one is the green fill, as the
// register's category chips are (tokens.md). For filters and view switches;
// an option inside a form is a tinted card instead.

import { focusButton } from "./focus";
import { cx } from "./cx";

const base = `flex min-h-10 items-center gap-2 rounded-pill border px-4 text-base ${focusButton}`;

export function pillClass(on: boolean): string {
  return cx(base, on ? "border-accent bg-accent font-semibold text-accent-fg" : "border-line-strong bg-raised text-fg hover:bg-raised-high");
}

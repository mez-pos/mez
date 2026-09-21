// A raised surface: --ink-raised on the terminal, --paper-raised elsewhere.
// Elevation on ink is lightness, not shadow (tokens.md), so a card is a fill
// and a hairline, on both grounds.

import type { HTMLAttributes } from "react";
import { cx } from "./cx";

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: "none" | "md" | "lg";
}

const paddings = { none: "", md: "p-4", lg: "p-6" } as const;

export function Card({ padding = "md", className, ...rest }: CardProps) {
  return <div className={cx("rounded border border-line bg-raised text-fg", paddings[padding], className)} {...rest} />;
}

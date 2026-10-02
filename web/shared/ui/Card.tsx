// A raised surface: an ivory card on parchment, floated on a hairline and a
// warm shadow (tokens.md). On espresso night the shadow role resolves to the
// ticket's own, so the same card works on both grounds.

import type { HTMLAttributes } from "react";
import { cx } from "./cx";

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  padding?: "none" | "md" | "lg";
}

const paddings = { none: "", md: "p-4", lg: "p-6" } as const;

export function Card({ padding = "md", className, ...rest }: CardProps) {
  return <div className={cx("rounded-card border border-line bg-raised text-fg shadow-card", paddings[padding], className)} {...rest} />;
}

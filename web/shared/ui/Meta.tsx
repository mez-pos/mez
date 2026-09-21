// The small-caps trail under a title: TERMINAL · REGISTER (tokens.md).
// Items are written in ordinary case and uppercased by CSS, so a screen
// reader says "Terminal, Register" rather than spelling out capitals.

import { cx } from "./cx";

export interface MetaProps {
  items: readonly string[];
  className?: string;
}

export function Meta({ items, className }: MetaProps) {
  return (
    <p className={cx("text-meta font-semibold uppercase text-fg-faint", className)}>
      {items.map((item, i) => (
        <span key={`${i}-${item}`}>
          {i > 0 && <span aria-hidden="true"> · </span>}
          {item}
        </span>
      ))}
    </p>
  );
}

// The category chips across the top of the register. They wrap onto a
// second row rather than scrolling sideways: a chip off the edge of the
// screen is a category nobody finds mid-service.

import type { CategoryResponse } from "../../../shared/api-types";
import { cx } from "../../../shared/ui/cx";

interface CategoryRailProps {
  categories: readonly CategoryResponse[];
  counts: ReadonlyMap<number, number>;
  /** Null while a search is showing results from every category. */
  chosen: number | null;
  onChoose: (id: number) => void;
}

export function CategoryRail({ categories, counts, chosen, onChoose }: CategoryRailProps) {
  return (
    <nav aria-label="Categories" className="flex flex-wrap gap-tap-gap">
      {categories.map((c) => {
        const active = c.id === chosen;
        return (
          <button
            key={c.id}
            type="button"
            aria-pressed={active}
            onClick={() => onChoose(c.id)}
            className={cx(
              "flex min-h-tap items-center gap-2.5 rounded-pill border px-5 text-base",
              "outline-offset-2 outline-attention focus-visible:outline-3",
              active ? "border-accent bg-accent font-semibold text-accent-fg" : "border-line-strong bg-raised font-medium text-fg hover:bg-raised-high",
            )}
          >
            <span>{c.name}</span>
            <span className={cx("font-mono text-sm tabular-nums", !active && "text-fg-muted")}>{counts.get(c.id) ?? 0}</span>
          </button>
        );
      })}
    </nav>
  );
}

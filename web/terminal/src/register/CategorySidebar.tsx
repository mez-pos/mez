// The 240px category sidebar (D03): All Items, Popular, then each category, an
// icon and a name each. The one chosen is terracotta, as the navigation is.
// They stack, never scroll sideways: a category off the edge of the screen is
// one nobody finds mid-service.

import { cx } from "../../../shared/ui/cx";
import { CategoryIcon } from "./CategoryIcon";
import type { MenuView, SidebarEntry } from "./register-logic";

interface CategorySidebarProps {
  entries: readonly SidebarEntry[];
  /** Null while a search shows results from every category. */
  chosen: MenuView | null;
  onChoose: (view: MenuView) => void;
}

export function CategorySidebar({ entries, chosen, onChoose }: CategorySidebarProps) {
  return (
    <nav aria-label="Categories" className="flex w-60 shrink-0 flex-col gap-1 overflow-y-auto border-r border-line bg-chrome px-3.5 py-4">
      {entries.map((entry) => {
        const active = entry.view === chosen;
        return (
          <button
            key={String(entry.view)}
            type="button"
            aria-pressed={active}
            onClick={() => onChoose(entry.view)}
            className={cx(
              "flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base",
              "outline-offset-2 outline-attention focus-visible:outline-3",
              active ? "bg-attention font-semibold text-attention-fg" : "text-fg hover:bg-raised-high",
            )}
          >
            <CategoryIcon name={entry.icon} className="shrink-0" />
            <span className="min-w-0 flex-1 truncate">{entry.label}</span>
            <span className={cx("font-mono text-sm tabular-nums", active ? "text-attention-fg" : "text-fg-muted")}>{entry.count}</span>
          </button>
        );
      })}
    </nav>
  );
}

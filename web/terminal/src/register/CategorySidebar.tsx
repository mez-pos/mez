// The category sidebar (D03): All Items, Popular, then each category, an icon
// and a name each. The one chosen is terracotta, as the navigation is. They
// stack, never scroll sideways: a category off the edge of the screen is one
// nobody finds mid-service. Its width is the `side` token (tokens.md,
// "Layout"): the drawing's 240px on a wide screen, less on a small one.
//
// A waiter's (T02) has the search over the categories and the one chosen in
// green.

import type { ReactNode } from "react";
import { cx } from "../../../shared/ui/cx";
import { CategoryIcon } from "./CategoryIcon";
import type { MenuView, SidebarEntry } from "./register-logic";

interface CategorySidebarProps {
  entries: readonly SidebarEntry[];
  /** Null while a search shows results from every category. */
  chosen: MenuView | null;
  onChoose: (view: MenuView) => void;
  /** The waiter's sidebar (T02): narrower, green, with the search on top. */
  search?: ReactNode;
}

export function CategorySidebar({ entries, chosen, onChoose, search }: CategorySidebarProps) {
  const waiter = search !== undefined;
  const pressed = waiter ? "bg-accent font-semibold text-accent-fg" : "bg-attention font-semibold text-attention-fg";
  return (
    <nav
      aria-label="Categories"
      className={cx("flex w-side shrink-0 flex-col gap-1 overflow-y-auto border-r border-line bg-chrome py-4 short:py-3", waiter ? "px-3" : "px-3 2xl:px-3.5")}
    >
      {search && <div className="mb-2">{search}</div>}
      {entries.map((entry) => {
        const active = entry.view === chosen;
        return (
          <button
            key={String(entry.view)}
            type="button"
            aria-pressed={active}
            onClick={() => onChoose(entry.view)}
            className={cx(
              "flex min-h-tap w-full items-center gap-3 rounded px-3 text-left text-base 2xl:px-4",
              "outline-offset-2 outline-attention focus-visible:outline-3",
              active ? pressed : "text-fg hover:bg-raised-high",
            )}
          >
            <CategoryIcon name={entry.icon} className="shrink-0" />
            <span className="min-w-0 flex-1 truncate">{entry.label}</span>
            <span className={cx("font-mono text-sm tabular-nums", active ? "text-current" : "text-fg-muted")}>{entry.count}</span>
          </button>
        );
      })}
    </nav>
  );
}

// The 240px category sidebar (D03): All Items, Popular, then each category, an
// icon and a name each. The one chosen is terracotta, as the navigation is.
// They stack, never scroll sideways: a category off the edge of the screen is
// one nobody finds mid-service.
//
// A waiter's (T02) is 224px, with the search over the categories, the one chosen
// in green, and a script line at the foot.

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
      className={cx("flex shrink-0 flex-col gap-1 overflow-y-auto border-r border-line bg-chrome py-4", waiter ? "w-56 px-3" : "w-60 px-3.5")}
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
              "flex min-h-tap w-full items-center gap-3 rounded px-4 text-left text-base",
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
      {waiter && (
        <p aria-hidden="true" className="mt-auto px-2 pt-4 font-script text-xl leading-tight text-fg-2">
          Good Food,
          <br />
          Happier People ♡
        </p>
      )}
    </nav>
  );
}

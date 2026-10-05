// The title of what the menu is showing, with the dietary filters and the
// search beside it (D03). The filters are toggles that add up (register-logic's
// filterItems); a chip is its tone and its word, with a tick when it is on.
// Left out: the design's sort and list/grid switches, which nothing does.

import { BeefIcon, CheckIcon, LeafIcon, SproutIcon, WheatOffIcon, type LucideIcon } from "lucide-react";
import { SearchField } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import type { DietFilter } from "./register-logic";

interface MenuHeaderProps {
  title: string;
  filter: DietFilter;
  onFilter: (next: DietFilter) => void;
  /** Bumped to empty the search box: SearchField keeps what is typed in it. */
  searchKey: number;
  onSearch: (query: string) => void;
  /** The search is drawn elsewhere: over the categories, for a waiter (T02). */
  searchElsewhere?: boolean;
}

const chips: readonly { key: keyof DietFilter; label: string; icon: LucideIcon; tone: string }[] = [
  { key: "veg", label: "Veg", icon: LeafIcon, tone: "border-chip-success-border bg-chip-success-bg text-chip-success-fg" },
  { key: "nonVeg", label: "Non-Veg", icon: BeefIcon, tone: "border-chip-danger-border bg-chip-danger-bg text-chip-danger-fg" },
  { key: "vegan", label: "Vegan", icon: SproutIcon, tone: "border-chip-success-border bg-chip-success-bg text-chip-success-fg" },
  { key: "glutenFree", label: "Gluten Free", icon: WheatOffIcon, tone: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg" },
];

export function MenuHeader({ title, filter, onFilter, searchKey, onSearch, searchElsewhere = false }: MenuHeaderProps) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
      <h1 className="font-serif text-total">{title}</h1>
      <div role="group" aria-label="Dietary filters" className="flex flex-wrap gap-2">
        {chips.map(({ key, label, icon: Icon, tone }) => {
          const on = filter[key];
          return (
            <button
              key={key}
              type="button"
              aria-pressed={on}
              onClick={() => onFilter({ ...filter, [key]: !on })}
              className={cx(
                "inline-flex min-h-tap items-center gap-2 rounded-pill border px-4 text-base outline-offset-2 outline-attention focus-visible:outline-3",
                on ? cx(tone, "font-semibold") : "border-line-strong bg-raised text-fg-2 hover:bg-raised-high",
              )}
            >
              <Icon aria-hidden="true" size={16} />
              {label}
              {on && <CheckIcon aria-hidden="true" size={15} />}
            </button>
          );
        })}
      </div>
      {!searchElsewhere && <SearchField key={searchKey} label="Search the menu" onSearch={onSearch} className="ml-auto w-full sm:w-64" />}
    </div>
  );
}

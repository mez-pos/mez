// The title of what the menu is showing, with the dietary filters beside it
// (D03). The filters are toggles that add up (register-logic's filterItems); a
// chip is its tone and its word, with a tick when it is on. Left out: the
// design's sort and list/grid switches, which nothing does.
//
// One row, so the cards keep the height (TILL-3). The search is over the
// categories (CategorySidebar). From xl the four pills fit beside the title;
// below it they wait behind one Diet button, which says what is on and opens
// them under it.

import { BeefIcon, CheckIcon, ChevronDownIcon, LeafIcon, ListFilterIcon, SproutIcon, WheatOffIcon, type LucideIcon } from "lucide-react";
import { useDisclosure } from "../../../shared/shell/use-disclosure";
import { cx } from "../../../shared/ui/cx";
import { dietFilterActive, dietLabels, dietSummary, type DietFilter } from "./register-logic";

interface MenuHeaderProps {
  title: string;
  filter: DietFilter;
  onFilter: (next: DietFilter) => void;
}

const chips: readonly { key: keyof DietFilter; icon: LucideIcon; tone: string }[] = [
  { key: "veg", icon: LeafIcon, tone: "border-chip-success-border bg-chip-success-bg text-chip-success-fg" },
  { key: "nonVeg", icon: BeefIcon, tone: "border-chip-danger-border bg-chip-danger-bg text-chip-danger-fg" },
  { key: "vegan", icon: SproutIcon, tone: "border-chip-success-border bg-chip-success-bg text-chip-success-fg" },
  { key: "glutenFree", icon: WheatOffIcon, tone: "border-chip-warning-border bg-chip-warning-bg text-chip-warning-fg" },
];

const pill = "inline-flex min-h-tap items-center gap-2 rounded-pill border px-4 text-base outline-offset-2 outline-focus focus-visible:outline-3";
const pillOff = "border-line-strong bg-raised text-fg-2 hover:bg-raised-high";

export function MenuHeader({ title, filter, onFilter }: MenuHeaderProps) {
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
      <h1 className="font-serif text-total">{title}</h1>
      <DietPills filter={filter} onFilter={onFilter} className="hidden flex-wrap gap-2 xl:flex" />
      <DietMenu filter={filter} onFilter={onFilter} />
    </div>
  );
}

interface DietPillsProps {
  filter: DietFilter;
  onFilter: (next: DietFilter) => void;
  className: string;
}

function DietPills({ filter, onFilter, className }: DietPillsProps) {
  return (
    <div role="group" aria-label="Dietary filters" className={className}>
      {chips.map(({ key, icon: Icon, tone }) => {
        const on = filter[key];
        return (
          <button
            key={key}
            type="button"
            aria-pressed={on}
            onClick={() => onFilter({ ...filter, [key]: !on })}
            className={cx(pill, on ? cx(tone, "font-semibold") : pillOff)}
          >
            <Icon aria-hidden="true" size={16} />
            {dietLabels[key]}
            {on && <CheckIcon aria-hidden="true" size={15} />}
          </button>
        );
      })}
    </div>
  );
}

/** Below xl: one button saying what is on, and the four pills under it. A tap elsewhere closes it. */
function DietMenu({ filter, onFilter }: Omit<DietPillsProps, "className">) {
  const { open, toggle, box, trigger, panel, panelId, leaves } = useDisclosure();
  const on = dietFilterActive(filter);
  return (
    <div ref={box} onBlur={leaves} className="relative ml-auto xl:hidden">
      <button
        ref={trigger}
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={toggle}
        className={cx(pill, on ? "border-accent bg-accent-tint font-semibold text-accent-text" : pillOff)}
      >
        <ListFilterIcon aria-hidden="true" size={16} />
        {dietSummary(filter)}
        <ChevronDownIcon aria-hidden="true" size={16} className={cx("shrink-0", open && "rotate-180")} />
      </button>
      {open && (
        <div ref={panel} id={panelId} className="absolute right-0 top-full z-20 mt-2 rounded-card border border-line bg-raised p-2 shadow-card">
          <DietPills filter={filter} onFilter={onFilter} className="flex w-52 flex-col gap-2" />
        </div>
      )}
    </div>
  );
}

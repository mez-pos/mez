// The item tiles. One tap rings an item up (terminal-register.md): the tile
// is the whole target, at least 7rem tall, and it shows only what helps
// someone pick the right thing fast. A sold-out item stays in its place,
// says so in words, and cannot be rung up (ADR-109): staff can tell a
// guest it is gone rather than hunt for it.

import type { MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { choiceCount } from "./register-logic";

interface ItemGridProps {
  items: readonly MenuItemResponse[];
  groupsFor: (item: MenuItemResponse) => readonly ModifierGroupResponse[];
  stationName: (code: string) => string;
  onTap: (item: MenuItemResponse) => void;
  soldOut: (item: MenuItemResponse) => boolean;
}

export function ItemGrid({ items, groupsFor, stationName, onTap, soldOut }: ItemGridProps) {
  return (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))] gap-2.5">
      {items.map((item) => {
        const choices = choiceCount(groupsFor(item));
        const gone = soldOut(item);
        return (
          <li key={item.id}>
            <button
              type="button"
              onClick={() => onTap(item)}
              disabled={gone}
              aria-label={gone ? `${item.name}, sold out` : undefined}
              className={cx(
                "flex h-full min-h-28 w-full flex-col gap-1.5 rounded border border-line bg-raised p-3.5 text-left",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                gone ? "cursor-not-allowed opacity-60" : "hover:bg-raised-high active:bg-raised-high",
              )}
            >
              <span className={cx("text-base font-semibold leading-snug", gone ? "text-fg-muted line-through" : "text-fg")}>{item.name}</span>
              {gone && <span className="text-meta font-semibold uppercase text-warning">Sold out</span>}
              <span className="text-meta uppercase text-fg-faint">
                {stationName(item.station)}
                {choices > 0 && ` · ${choices} choices`}
              </span>
              <span className="mt-auto self-end">
                <Money amount={item.price} />
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

// The item tiles. One tap rings an item up (terminal-register.md): the tile
// is the whole target, at least 7rem tall, and it shows only what helps
// someone pick the right thing fast. A sold-out item stays in its place,
// says so in words, and cannot be rung up (ADR-109): staff can tell a
// guest it is gone rather than hunt for it.

import type { MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { photoUrl } from "../../../shared/menu";
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
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(11rem,1fr))] gap-3">
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
                "flex h-full min-h-28 w-full flex-col gap-1.5 overflow-hidden rounded-card border border-line bg-raised p-4 text-left shadow-rest",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                gone ? "cursor-not-allowed opacity-60" : "hover:bg-raised-high active:bg-raised-high",
              )}
            >
              {item.photo && (
                // The name is under it, so the photo says nothing a reader needs.
                <img
                  src={photoUrl(item.photo)}
                  alt=""
                  loading="lazy"
                  decoding="async"
                  className={cx("-mx-4 -mt-4 mb-1.5 h-24 w-[calc(100%+2rem)] max-w-none object-cover", gone && "grayscale")}
                />
              )}
              <span className={cx("font-serif text-lg leading-snug", gone && "text-fg-muted line-through")}>{item.name}</span>
              {gone && <span className="text-meta font-semibold uppercase text-warning">Sold out</span>}
              <span className="text-meta uppercase text-fg-faint">
                {stationName(item.station)}
                {choices > 0 && ` · ${choices} choices`}
              </span>
              <span className="mt-auto self-end pt-1">
                <Money amount={item.price} className="text-lg font-semibold" />
              </span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

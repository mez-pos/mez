// The item cards (D03). A card looks finished with no photo: a tinted block
// wearing its category's icon stands where the photo would be. A diet mark and
// the name lead; a description, how many choices it has, and the price follow.
// It is two targets (ADR-154): the card opens the item modal when there is
// anything to choose, and the round "+" adds it with its defaults at once. A
// popular item wears a crown, beside its price. A sold-out item stays in its
// place, says so in words, and cannot be rung up (ADR-109): staff can tell a
// guest it is gone.

import { CrownIcon, PlusIcon, WheatOffIcon } from "lucide-react";
import type { MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";
import { photoUrl } from "../../../shared/menu";
import { Money } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { DietMark } from "../../../shared/ui/DietMark";
import { CategoryIcon } from "./CategoryIcon";
import { choiceCount, type CardTap, type CategoryIconKey } from "./register-logic";

interface ItemGridProps {
  items: readonly MenuItemResponse[];
  groupsFor: (item: MenuItemResponse) => readonly ModifierGroupResponse[];
  iconFor: (item: MenuItemResponse) => CategoryIconKey;
  onTap: (item: MenuItemResponse, tap: CardTap) => void;
  soldOut: (item: MenuItemResponse) => boolean;
  /** Narrower cards, so three fit beside the waiter's rail at 1366 wide (T02). */
  dense?: boolean;
}

// The grid is auto-fill: as many cards as the width holds, never a fixed
// count. A card's photo band and padding give way on a short screen (the
// `short:` variant, tokens.md): a photo's band is shorter, and a card with no
// photo has none, since the category's icon again on every card is not worth a
// row of dishes (TILL-3). So a 1024x768 tablet shows six cards, not four.

export function ItemGrid({ items, groupsFor, iconFor, onTap, soldOut, dense = false }: ItemGridProps) {
  return (
    <ul
      className={cx(
        "grid gap-3 2xl:gap-4",
        dense
          ? "grid-cols-[repeat(auto-fill,minmax(11rem,1fr))] 2xl:grid-cols-[repeat(auto-fill,minmax(13rem,1fr))]"
          : "grid-cols-[repeat(auto-fill,minmax(12.5rem,1fr))] 2xl:grid-cols-[repeat(auto-fill,minmax(14rem,1fr))]",
      )}
    >
      {items.map((item) => {
        const choices = choiceCount(groupsFor(item));
        const gone = soldOut(item);
        return (
          <li key={item.id} className="relative">
            <button
              type="button"
              onClick={() => onTap(item, "card")}
              disabled={gone}
              aria-label={gone ? `${item.name}, sold out` : undefined}
              className={cx(
                "flex h-full w-full flex-col overflow-hidden rounded-card border border-line bg-raised text-left shadow-rest",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                gone ? "cursor-not-allowed opacity-60" : "hover:bg-raised-high active:bg-raised-high",
              )}
            >
              <span className={cx("flex h-28 w-full items-center justify-center bg-sunken text-fg-faint", item.photo ? "short:h-16" : "short:hidden")}>
                {item.photo ? (
                  // The name is under it, so the photo says nothing a reader needs.
                  <img src={photoUrl(item.photo)} alt="" loading="lazy" decoding="async" className={cx("size-full object-cover", gone && "grayscale")} />
                ) : (
                  <CategoryIcon name={iconFor(item)} size={34} />
                )}
              </span>
              <span className="flex flex-1 flex-col gap-1 p-3.5 short:p-3">
                <span className="flex items-start gap-2">
                  <DietMark diet={item.diet} />
                  <span className={cx("font-serif text-lg leading-snug text-fg-heading", gone && "line-through")}>{item.name}</span>
                  {item.gluten_free && (
                    <span role="img" aria-label="Gluten free" title="Gluten free" className="mt-1 shrink-0 text-chip-warning-fg">
                      <WheatOffIcon aria-hidden="true" size={15} />
                    </span>
                  )}
                </span>
                {item.description && <span className="line-clamp-2 text-sm text-fg-muted short:line-clamp-1">{item.description}</span>}
                {gone && <span className="text-meta font-semibold uppercase text-warning">Sold out</span>}
                {choices > 0 && !gone && <span className="text-sm text-fg-muted">{choices} choices</span>}
                <span className="mt-auto flex items-center gap-2 pt-2">
                  <Money amount={item.price} className="text-lg font-semibold text-fg-heading" />
                  {item.popular && (
                    <span
                      role="img"
                      aria-label="Popular"
                      className="flex size-7 shrink-0 items-center justify-center rounded-round bg-highlight text-fg-heading"
                    >
                      <CrownIcon aria-hidden="true" size={15} />
                    </span>
                  )}
                  {/* Holds the place of the "+", which is a button of its own over this corner. */}
                  <span aria-hidden="true" className="ml-auto size-tap shrink-0" />
                </span>
              </span>
            </button>
            <button
              type="button"
              onClick={() => onTap(item, "plus")}
              disabled={gone}
              aria-label={`Add ${item.name}`}
              className={cx(
                // On its place-holder in the card's corner, which moves in with the padding on a short screen.
                "absolute bottom-3.5 right-3.5 flex size-tap items-center justify-center rounded-round short:bottom-3 short:right-3",
                "outline-offset-2 outline-line-focus focus-visible:outline-2",
                gone ? "cursor-not-allowed bg-sunken text-fg-muted opacity-60" : "bg-attention text-attention-fg hover:bg-attention-hover",
              )}
            >
              <PlusIcon aria-hidden="true" size={20} strokeWidth={2.4} />
            </button>
          </li>
        );
      })}
    </ul>
  );
}

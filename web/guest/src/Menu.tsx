// M02, the menu (docs/design/screens/guest-menu.md): every dish, narrowed by
// a category, a search and the diets the menu has. A dish opens in a sheet
// (ItemSheet.tsx); one with no choices to make is added straight from its
// card. The cart's bar stays at the foot while there is anything in it.

import { useState, type ReactNode } from "react";
import type { GuestItemResponse, GuestMenuResponse, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { DietMark } from "../../shared/ui/DietMark";
import { SearchField } from "../../shared/ui/SearchField";
import { countsByDish, inCartText, itemsText } from "./cart-logic";
import { ItemSheet } from "./ItemSheet";
import { OrderingBy, tableLabel } from "./Landing";
import { Notice } from "./Notice";
import type { CartApi } from "./use-cart";
import {
  categoriesWithItems,
  countText,
  dietFilterWords,
  headingFor,
  noFilter,
  offeredDietFilters,
  photoUrl,
  showsPhotoFrames,
  toggleDiet,
  visibleItems,
  type MenuFilter,
} from "./menu-logic";

interface MenuProps {
  table: GuestTableResponse;
  menu: GuestMenuResponse;
  cart: CartApi;
  onViewCart: () => void;
  /** The order placed, over the head, sticking with it (GST-4). */
  strip?: ReactNode;
}

const chip = "inline-flex min-h-tap shrink-0 items-center rounded-pill border px-4 text-base outline-offset-2 outline-focus focus-visible:outline-3";
const chipOff = "border-line-strong bg-raised text-fg";
const chipOn = "border-accent bg-accent font-semibold text-accent-fg";

export function Menu({ table, menu, cart, onViewCart, strip }: MenuProps) {
  // The dish that is open, by id: read from the menu each time, so a sheet
  // left open shows it sold out when it does.
  const [openId, setOpenId] = useState<number | null>(null);
  const open = openId === null ? undefined : menu.items.find((it) => it.id === openId);
  const [filter, setFilter] = useState<MenuFilter>(noFilter);
  // Clearing the search from outside the field means giving it a new one.
  const [searchKey, setSearchKey] = useState(0);
  const categories = categoriesWithItems(menu);
  const diets = offeredDietFilters(menu);
  const items = visibleItems(menu, filter);
  const framed = showsPhotoFrames(menu);
  const inCart = countsByDish(cart.lines);
  const filtered = filter.search.trim() !== "" || filter.diets.length > 0 || filter.category !== null;

  const clear = () => {
    setFilter(noFilter);
    setSearchKey((k) => k + 1);
  };

  return (
    <div className={cx("mx-auto min-h-dvh w-full max-w-md bg-canvas text-fg", cart.count > 0 ? "pb-28" : "pb-10")}>
      {/* The order's strip and the head stick together: a decline is never scrolled out of sight (GST-4). */}
      <div className="sticky top-0 z-10">
        {strip}
        {/* The venue's name in the bar, where mez's logo was (GST-10). */}
        <header className="flex h-14 items-center justify-between gap-3 border-b border-line bg-chrome px-4">
          <p className="min-w-0 truncate font-serif text-xl">{table.venue.name}</p>
          <p className="shrink-0 rounded-pill bg-attention-tint px-3.5 py-1.5 text-base font-bold text-attention-text">{tableLabel(table.table.name)}</p>
        </header>
      </div>

      <main className="px-4">
        <Notice text={cart.notice} onDismiss={cart.dismissNotice} />

        <SearchField key={searchKey} className="mt-3" label="Search for dishes" onSearch={(search) => setFilter((f) => ({ ...f, search }))} />

        {categories.length > 1 && (
          <nav aria-label="Categories" className="-mx-4 mt-3 flex gap-2 overflow-x-auto px-4 scrollbar-none">
            <button
              type="button"
              aria-pressed={filter.category === null}
              className={cx(chip, filter.category === null ? chipOn : chipOff)}
              onClick={() => setFilter((f) => ({ ...f, category: null }))}
            >
              All
            </button>
            {categories.map((c) => (
              <button
                key={c.id}
                type="button"
                aria-pressed={filter.category === c.id}
                className={cx(chip, filter.category === c.id ? chipOn : chipOff)}
                onClick={() => setFilter((f) => ({ ...f, category: c.id }))}
              >
                {c.name}
              </button>
            ))}
          </nav>
        )}

        {diets.length > 0 && (
          <div role="group" aria-label="Diet" className="-mx-4 mt-2 flex gap-2 overflow-x-auto px-4 scrollbar-none">
            {diets.map((diet) => {
              const on = filter.diets.includes(diet);
              return (
                <button
                  key={diet}
                  type="button"
                  aria-pressed={on}
                  className={cx(chip, on ? "border-success bg-accent-tint font-semibold text-accent-text" : chipOff)}
                  onClick={() => setFilter((f) => toggleDiet(f, diet))}
                >
                  {dietFilterWords[diet]}
                </button>
              );
            })}
          </div>
        )}

        <div className="mt-5 flex items-end justify-between gap-3">
          <h1 className="font-serif text-xl">{headingFor(menu, filter)}</h1>
          <p aria-live="polite" className="shrink-0 text-sm text-fg-muted">
            {countText(items.length)}
          </p>
        </div>

        {items.length === 0 ? (
          <div className="mt-6 rounded-card border border-line bg-raised p-6 text-center">
            <p className="text-base text-fg-2">{menu.items.length === 0 ? "The menu is empty for now." : "Nothing on the menu matches."}</p>
            {filtered && (
              <Button variant="secondary" className="mt-4" onClick={clear}>
                Show everything
              </Button>
            )}
          </div>
        ) : (
          <ul className="mt-3 grid grid-cols-2 gap-3">
            {items.map((item) => (
              <DishCard
                key={item.id}
                item={item}
                framed={framed}
                inCart={inCart.get(item.id) ?? 0}
                onOpen={() => setOpenId(item.id)}
                // Nothing to choose: one tap adds one. Anything to choose opens it.
                onAdd={() => (item.modifier_groups.length === 0 ? cart.add({ item_id: item.id, modifier_ids: [], qty: 1 }) : setOpenId(item.id))}
              />
            ))}
          </ul>
        )}
        <OrderingBy className="mt-10" />
      </main>

      {cart.count > 0 && (
        // Clear of the phone's home bar: the page is drawn to the edges (GST-6).
        <div className="fixed inset-x-0 bottom-0 z-10 mx-auto w-full max-w-md px-3 pb-[max(--spacing(3),env(safe-area-inset-bottom))]">
          <button
            type="button"
            onClick={onViewCart}
            className="flex min-h-charge w-full items-center justify-between gap-3 rounded-card bg-accent px-5 text-accent-fg shadow-modal outline-offset-2 outline-focus focus-visible:outline-3"
          >
            <span className="text-lg font-bold">View cart</span>
            <span aria-live="polite" className="text-base">
              {itemsText(cart.count)} · {formatMoney(cart.total)}
            </span>
          </button>
        </div>
      )}

      {open && <ItemSheet key={open.id} menu={menu} item={open} onAdd={cart.add} onClose={() => setOpenId(null)} />}
    </div>
  );
}

interface DishCardProps {
  item: GuestItemResponse;
  /** Whether cards have a frame for a photo: once any dish on the menu has one. */
  framed: boolean;
  /** How many of it the cart holds already, whatever was chosen (GST-8). */
  inCart: number;
  onOpen: () => void;
  onAdd: () => void;
}

function DishCard({ item, framed, inCart, onOpen, onAdd }: DishCardProps) {
  return (
    // Sold out is a sunken card with its photo in grey and every word at full
    // contrast, never a fade (TECH-13, ADR-390).
    <li className={cx("flex flex-col overflow-hidden rounded-card border border-line", item.sold_out ? "bg-sunken" : "bg-raised shadow-rest")}>
      {/* The whole of the card above the button opens the dish. */}
      <button type="button" onClick={onOpen} className="flex flex-1 flex-col text-left outline-offset-[-3px] outline-focus focus-visible:outline-3">
        {framed &&
          (item.photo ? (
            // Below the fold on a slow link a photo is fetched when it is
            // scrolled to, and decoded off the main thread.
            <img
              src={photoUrl(item.photo)}
              alt=""
              loading="lazy"
              decoding="async"
              className={cx("aspect-[4/3] w-full bg-sunken object-cover", item.sold_out && "grayscale")}
            />
          ) : (
            // The menu has photos and this dish has none yet: the frame is
            // kept, empty, so the cards line up.
            <span aria-hidden="true" className="block aspect-[4/3] w-full bg-sunken" />
          ))}
        <span className="flex flex-1 flex-col p-3 pb-2">
          {item.popular && !item.sold_out && (
            <span className="mb-1.5 self-start rounded-pill bg-highlight px-2.5 py-0.5 text-sm font-bold text-highlight-fg">Popular</span>
          )}
          <span className="flex items-start gap-1.5">
            <DietMark diet={item.diet} className="mt-0.5" />
            <span className="font-serif text-base leading-snug">{item.name}</span>
          </span>
          {item.description && <span className="mt-1 line-clamp-2 text-sm text-fg-muted">{item.description}</span>}
          {/* Said quietly: on a menu of coffees it is on nearly every card (GST-9). */}
          {item.gluten_free && <span className="mt-1 text-sm text-fg-muted">Gluten free</span>}
          {/* What is in the cart already, beside the price; under it on a narrow card (GST-8). */}
          <span className="mt-auto flex flex-wrap items-baseline justify-between gap-x-2 pt-2">
            <span className={cx("font-serif text-lg", item.sold_out && "text-fg-muted")}>{formatMoney(item.price)}</span>
            {inCart > 0 && <span className="text-sm font-bold text-accent-text">{inCartText(inCart)}</span>}
          </span>
        </span>
      </button>
      <div className="px-3 pb-3">
        {item.sold_out ? (
          <p className="flex min-h-tap items-center justify-center rounded-pill border border-chip-warning-border bg-chip-warning-bg text-base font-semibold text-chip-warning-fg">
            Sold out
          </p>
        ) : (
          // The tint, not the fill: one on every dish, and View cart is the primary (GST-9).
          <Button variant="tint" className="w-full" aria-label={`Add ${item.name}`} onClick={onAdd}>
            + Add
          </Button>
        )}
      </div>
    </li>
  );
}

// M02, the menu (docs/design/screens/guest-menu.md): every dish, narrowed by
// a category, a search and the diets the menu has. Reading only: a dish is
// opened and added in Task 3c.

import { useState } from "react";
import type { GuestItemResponse, GuestMenuResponse, GuestTableResponse } from "../../shared/api-types";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { DietMark } from "../../shared/ui/DietMark";
import { Logo } from "../../shared/ui/Logo";
import { SearchField } from "../../shared/ui/SearchField";
import { tableLabel } from "./Landing";
import {
  categoriesWithItems,
  countText,
  dietFilterWords,
  headingFor,
  noFilter,
  offeredDietFilters,
  photoUrl,
  toggleDiet,
  visibleItems,
  type MenuFilter,
} from "./menu-logic";

interface MenuProps {
  table: GuestTableResponse;
  menu: GuestMenuResponse;
}

const chip = "inline-flex min-h-tap shrink-0 items-center rounded-pill border px-4 text-base outline-offset-2 outline-line-focus focus-visible:outline-2";
const chipOff = "border-line-strong bg-raised text-fg";
const chipOn = "border-accent bg-accent font-semibold text-accent-fg";

export function Menu({ table, menu }: MenuProps) {
  const [filter, setFilter] = useState<MenuFilter>(noFilter);
  // Clearing the search from outside the field means giving it a new one.
  const [searchKey, setSearchKey] = useState(0);
  const categories = categoriesWithItems(menu);
  const diets = offeredDietFilters(menu);
  const items = visibleItems(menu, filter);
  const filtered = filter.search.trim() !== "" || filter.diets.length > 0 || filter.category !== null;

  const clear = () => {
    setFilter(noFilter);
    setSearchKey((k) => k + 1);
  };

  return (
    <div className="mx-auto min-h-dvh w-full max-w-md bg-canvas pb-10 text-fg">
      <header className="sticky top-0 z-10 flex h-14 items-center justify-between border-b border-line bg-chrome px-4">
        <Logo className="h-6" />
        <p className="rounded-pill bg-attention-tint px-3.5 py-1.5 text-base font-bold text-attention-text">{tableLabel(table.table.name)}</p>
      </header>

      <main className="px-4">
        <div className="mt-3 flex items-center justify-between gap-3 rounded-card bg-accent px-5 py-4 text-accent-fg">
          <p aria-hidden="true" className="font-script text-xl leading-tight">
            Great food,
            <br />
            brighter days ♡
          </p>
          <p className="text-right text-meta leading-relaxed font-bold tracking-[.16em] uppercase opacity-80">{table.venue.name}</p>
        </div>

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
              <DishCard key={item.id} item={item} />
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}

function DishCard({ item }: { item: GuestItemResponse }) {
  return (
    <li className={cx("flex flex-col overflow-hidden rounded-card border border-line bg-raised shadow-rest", item.sold_out && "opacity-60")}>
      {item.photo && (
        // A dish with no photo has no empty frame where one would be: most
        // venues start with none. Below the fold on a slow link a photo is
        // fetched when it is scrolled to, and decoded off the main thread.
        <img src={photoUrl(item.photo)} alt="" loading="lazy" decoding="async" className="aspect-[4/3] w-full bg-sunken object-cover" />
      )}
      <div className="flex flex-1 flex-col p-3">
        {item.popular && !item.sold_out && (
          <p className="mb-1.5 self-start rounded-pill bg-highlight px-2.5 py-0.5 text-sm font-bold text-highlight-fg">Popular</p>
        )}
        <div className="flex items-start gap-1.5">
          <DietMark diet={item.diet} className="mt-0.5" />
          <h2 className="font-serif text-base leading-snug">{item.name}</h2>
        </div>
        {item.description && <p className="mt-1 line-clamp-2 text-sm text-fg-muted">{item.description}</p>}
        {item.gluten_free && <p className="mt-1 text-sm font-semibold text-fg-2">Gluten free</p>}
        <div className="mt-auto flex items-center justify-between gap-2 pt-2">
          <p className="font-serif text-lg">{formatMoney(item.price)}</p>
          {item.sold_out && <span className="rounded-pill bg-sunken px-2.5 py-0.5 text-sm font-semibold text-fg-2">Sold out</span>}
        </div>
      </div>
    </li>
  );
}

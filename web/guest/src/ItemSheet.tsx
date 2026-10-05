// M03, a dish opened (docs/design/screens/guest-item.md): what it is, the
// choices it comes with, how many, and Add. The rules for choosing are the
// till's own (shared/modifier-selection.ts), so a guest can build nothing a
// cashier could not.

import { useState } from "react";
import type { GuestItemResponse, GuestMenuResponse, ModifierGroupResponse, Money } from "../../shared/api-types";
import { firstUnmet, groupHint, groupState, initialSelection, lineTotal, toggle, toModifiers } from "../../shared/modifier-selection";
import { formatMoney } from "../../shared/money";
import { Button } from "../../shared/ui/Button";
import { cx } from "../../shared/ui/cx";
import { Qty } from "../../shared/ui/Qty";
import { Sheet } from "../../shared/ui/Sheet";
import { MAX_QTY, groupsFor, type CartLine } from "./cart-logic";
import { photoUrl } from "./menu-logic";

const dietWords = { veg: "Vegetarian", vegan: "Vegan", non_veg: "Non-vegetarian" } as const;

interface ItemSheetProps {
  menu: GuestMenuResponse;
  item: GuestItemResponse;
  onAdd: (line: CartLine) => void;
  onClose: () => void;
}

/** "+₹40.00" for a choice that costs more, "−₹10.00" for one that costs less, nothing for one that costs the same. */
export function deltaText(price: Money): string {
  if (price.minor === 0) return "";
  return price.minor > 0 ? `+${formatMoney(price)}` : formatMoney(price);
}

export function ItemSheet({ menu, item, onAdd, onClose }: ItemSheetProps) {
  const groups = groupsFor(menu, item);
  const [selection, setSelection] = useState(() => initialSelection(groups));
  const [qty, setQty] = useState(1);

  const modifiers = toModifiers(groups, selection);
  const unmet = firstUnmet(groups, selection);
  const add = () => {
    onAdd({ item_id: item.id, modifier_ids: modifiers.map((m) => m.modifier_id), qty });
    onClose();
  };

  const footer = item.sold_out ? (
    <p className="py-2 text-center text-base font-semibold text-fg-2">Sold out for now</p>
  ) : (
    <div className="flex items-center gap-3">
      <Qty value={qty} onChange={setQty} bounds={{ min: 1, max: MAX_QTY }} label={item.name} />
      <Button variant="primary" className="min-w-0 flex-1" disabled={unmet !== null} onClick={add}>
        {unmet ? `Choose ${unmet.name.toLowerCase()}` : `Add to cart · ${formatMoney(lineTotal(item.price, modifiers, qty))}`}
      </Button>
    </div>
  );

  return (
    <Sheet open onClose={onClose} title={item.name} footer={footer}>
      {item.photo && <img src={photoUrl(item.photo)} alt="" className="mb-4 aspect-[4/3] w-full rounded bg-sunken object-cover" />}
      <p className="font-serif text-xl">{formatMoney(item.price)}</p>
      {item.description && <p className="mt-1.5 text-base text-fg-2">{item.description}</p>}
      {(item.diet || item.gluten_free || item.popular) && (
        <p className="mt-3 flex flex-wrap gap-2 text-sm font-semibold">
          {item.diet && (
            <span className={cx("rounded-pill px-3 py-1", item.diet === "non_veg" ? "bg-sunken text-fg-2" : "bg-accent-tint text-accent-text")}>
              {dietWords[item.diet]}
            </span>
          )}
          {item.gluten_free && <span className="rounded-pill bg-sunken px-3 py-1 text-fg-2">Gluten free</span>}
          {item.popular && <span className="rounded-pill bg-highlight px-3 py-1 text-highlight-fg">Popular</span>}
        </p>
      )}

      {!item.sold_out &&
        groups.map((group) => <Choices key={group.id} group={group} selection={selection} onToggle={(id) => setSelection((s) => toggle(s, group, id))} />)}
    </Sheet>
  );
}

interface ChoicesProps {
  group: ModifierGroupResponse;
  selection: ReturnType<typeof initialSelection>;
  onToggle: (modifierId: number) => void;
}

function Choices({ group, selection, onToggle }: ChoicesProps) {
  const chosen = selection.get(group.id) ?? [];
  const state = groupState(group, selection);
  const one = group.max_select === 1;
  const required = group.min_select > 0;
  return (
    <fieldset className="mt-6">
      <legend className="flex w-full items-baseline justify-between gap-3">
        <span className="font-serif text-lg">{group.name}</span>
        <span className={cx("text-sm font-semibold", required && !state.satisfied ? "text-danger" : "text-fg-muted")}>{groupHint(group)}</span>
      </legend>
      <div className="mt-2 flex flex-col gap-2">
        {group.modifiers.map((m) => {
          const on = chosen.includes(m.id);
          // A full pick-several group takes no more until one is let go.
          const blocked = !on && !one && state.full;
          return (
            <button
              key={m.id}
              type="button"
              role={one ? "radio" : "checkbox"}
              aria-checked={on}
              disabled={blocked}
              onClick={() => onToggle(m.id)}
              className={cx(
                "flex min-h-tap items-center gap-3 rounded border px-4 text-left text-base outline-offset-2 outline-line-focus focus-visible:outline-2 disabled:opacity-50",
                on ? "border-accent bg-accent-tint" : "border-line-strong bg-raised",
              )}
            >
              <span
                aria-hidden="true"
                className={cx(
                  "flex size-5 shrink-0 items-center justify-center border-2",
                  one ? "rounded-round" : "rounded-tight",
                  on ? "border-accent bg-accent" : "border-line-strong",
                )}
              >
                {on && <span className={cx("bg-accent-fg", one ? "size-2 rounded-round" : "size-2.5 rounded-[2px]")} />}
              </span>
              <span className="min-w-0 flex-1">{m.name}</span>
              <span className="shrink-0 font-semibold text-fg-2">{deltaText(m.price)}</span>
            </button>
          );
        })}
      </div>
    </fieldset>
  );
}

// The menu items card of the menu editor (D18): each item with its photo or a
// plain tile, its name and price, where it stands, and "..." for what can be
// done to it without opening it. The row itself opens it in the editor.

import { UtensilsIcon } from "lucide-react";
import { useState } from "react";
import type { EditorItem, MenuEditor } from "../../../shared/api-types";
import { photoUrl } from "../../../shared/menu";
import { Card, Money, StatusChip } from "../../../shared/ui";
import { cx } from "../../../shared/ui/cx";
import { useClock } from "../../../shared/use-clock";
import { inputClass } from "../FormFields";
import { RowMenu } from "../RowMenu";
import { canReorder, emptyLine, itemState, listItems, listTitle, orderOptions, type ListFilter, type Order } from "./browse-logic";
import { itemsOf, moved, problemOf } from "./menu-logic";
import type { MenuEditorApi } from "./use-menu-editor";

interface ItemListProps {
  menu: MenuEditor;
  filter: ListFilter;
  onOrder: (order: Order) => void;
  /** The item open in the editor, if any. */
  openId: number | null;
  onOpen: (item: EditorItem) => void;
  api: MenuEditorApi;
  onProblem: (message: string) => void;
}

export function ItemList({ menu, filter, onOrder, openId, onOpen, api, onProblem }: ItemListProps) {
  const [busy, setBusy] = useState(false);
  const now = new Date(useClock(60_000));
  const items = listItems(menu, filter, now);
  const run = (fn: () => Promise<void>) => {
    setBusy(true);
    fn()
      .catch((err: unknown) => onProblem(problemOf(err).message))
      .finally(() => setBusy(false));
  };
  // Moving an item saves its category's whole order, so it is offered only when the list is that order.
  const order = canReorder(filter) ? itemsOf(menu, filter.scope).map((i) => i.id) : null;
  const move = (item: EditorItem, delta: -1 | 1) => {
    const next = order && moved(order, order.indexOf(item.id), delta);
    if (next) run(() => api.orderItems(item.category_id, next));
  };

  return (
    <Card padding="none" className="flex min-w-0 flex-col overflow-hidden">
      <div className="flex flex-wrap items-center gap-3 border-b border-line p-4">
        <h2 className="min-w-40 flex-1 font-serif text-xl">{listTitle(items.length)}</h2>
        <div className="w-36 shrink-0">
          <select
            aria-label="Order"
            value={filter.order}
            onChange={(e) => onOrder(e.target.value as Order)}
            className={cx(inputClass, "rounded-pill text-base font-semibold")}
          >
            {orderOptions.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </div>
      </div>
      {typeof filter.scope === "number" && menu.categories.some((c) => c.id === filter.scope && !c.active) && (
        <p className="border-b border-line-soft p-4 text-sm text-fg-2">This category is retired: none of its items are on the order screen.</p>
      )}
      {items.length === 0 ? (
        <p className="p-4 text-sm text-fg-2">{emptyLine(menu, filter)}</p>
      ) : (
        <ul aria-label="Menu items" className="flex max-h-[70vh] min-h-64 flex-col gap-1.5 overflow-y-auto p-2.5 [scrollbar-width:thin]">
          {items.map((item, row) => {
            const state = itemState(item, now);
            const at = order ? order.indexOf(item.id) : -1;
            return (
              <li
                key={item.id}
                className={cx(
                  "flex min-w-0 shrink-0 items-center gap-1 rounded border",
                  item.id === openId ? "border-chip-success-border bg-accent-tint" : "border-transparent hover:bg-raised-high",
                )}
              >
                <button
                  type="button"
                  aria-current={item.id === openId ? "true" : undefined}
                  onClick={() => onOpen(item)}
                  className="flex min-h-tap min-w-0 flex-1 items-center gap-2.5 rounded p-2 text-left outline-offset-2 outline-line-focus focus-visible:outline-2"
                >
                  {item.photo ? (
                    <img src={photoUrl(item.photo)} alt="" loading="lazy" className="size-11 shrink-0 rounded-tight object-cover" />
                  ) : (
                    <span aria-hidden="true" className="flex size-11 shrink-0 items-center justify-center rounded-tight bg-sunken text-fg-muted">
                      <UtensilsIcon size={18} />
                    </span>
                  )}
                  <span className="min-w-0 flex-1">
                    <span className={cx("block truncate text-base font-bold", state === "retired" && "text-fg-muted")}>{item.name}</span>
                    {/* The price and where it stands share the second line, so the name has the whole of the first. */}
                    <span className="mt-0.5 flex flex-wrap items-center gap-x-2.5 gap-y-1">
                      <Money amount={item.price} tone={state === "retired" ? "muted" : "default"} className="text-sm" />
                      <StatusChip status={{ kind: "menu", status: state }} className="px-2 py-0 text-sm" />
                    </span>
                  </span>
                </button>
                <RowMenu
                  name={item.name}
                  disabled={busy}
                  // The list scrolls inside itself, so near its foot the menu opens upward to stay in sight.
                  up={row >= 3 && row >= items.length - 3}
                  actions={[
                    ...(state !== "retired"
                      ? [
                          {
                            label: state === "sold_out" ? "Back on sale" : "Sold out today",
                            run: () => run(() => api.setItemSoldOut(item.id, state !== "sold_out")),
                          },
                        ]
                      : []),
                    { label: state === "retired" ? "Bring back" : "Retire", run: () => run(() => api.setItemActive(item.id, state === "retired")) },
                    ...(order && at > 0 ? [{ label: "Move up", run: () => move(item, -1) }] : []),
                    ...(order && at >= 0 && at < order.length - 1 ? [{ label: "Move down", run: () => move(item, 1) }] : []),
                  ]}
                />
              </li>
            );
          })}
        </ul>
      )}
      {typeof filter.scope === "number" && !order && filter.search.trim() === "" && (
        <p className="border-t border-line-soft p-4 text-sm text-fg-muted">To move an item up or down, show every item in menu order.</p>
      )}
    </Card>
  );
}

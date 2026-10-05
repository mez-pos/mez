// What the menu editor's two lists work out (D18): the categories with their
// counts, and the items a category, a search and a filter leave, in the order
// asked for. The menu is the server's; this picks from it and orders it.

import type { EditorItem, MenuEditor } from "../../../shared/api-types";
import { isSoldOut } from "../../../shared/menu";
import type { MenuItemState } from "../../../shared/ui/status";

/** Which items the list is of: every one, the popular ones, or one category's. */
export type Scope = "all" | "popular" | number;

export interface ScopeRow {
  scope: Scope;
  name: string;
  count: number;
  /** A category taken off the tills; false for All items and Popular. */
  retired: boolean;
}

/** The categories column: All items, Popular, then each category in register order, with how many items each has. */
export function scopeRows(menu: MenuEditor): ScopeRow[] {
  return [
    { scope: "all", name: "All items", count: menu.items.length, retired: false },
    { scope: "popular", name: "Popular", count: menu.items.filter((i) => i.popular).length, retired: false },
    ...menu.categories.map((c) => ({
      scope: c.id,
      name: c.name,
      count: menu.items.filter((i) => i.category_id === c.id).length,
      retired: !c.active,
    })),
  ];
}

/** The scope to show: the one asked for, or All items when its category has gone. */
export function validScope(menu: MenuEditor, wanted: Scope): Scope {
  return typeof wanted === "number" && !menu.categories.some((c) => c.id === wanted) ? "all" : wanted;
}

/** Where an item stands: retired wins over sold out, since a retired item is on no till at all. */
export function itemState(item: Pick<EditorItem, "active" | "sold_out_until">, now: Date): MenuItemState {
  if (!item.active) return "retired";
  return isSoldOut(item, now) ? "sold_out" : "active";
}

/** Which states the list shows. */
export type Show = "all" | MenuItemState;

export const showOptions: readonly { value: Show; label: string }[] = [
  { value: "all", label: "All items" },
  { value: "active", label: "Active" },
  { value: "sold_out", label: "Sold out" },
  { value: "retired", label: "Retired" },
];

/** How the list is ordered: as the tills show it, by name, or by price. */
export type Order = "menu" | "name" | "price";

export const orderOptions: readonly { value: Order; label: string }[] = [
  { value: "menu", label: "Menu order" },
  { value: "name", label: "A–Z" },
  { value: "price", label: "Price" },
];

export interface ListFilter {
  scope: Scope;
  /** What is typed in the search: matched anywhere in the name, whatever the case. */
  search: string;
  show: Show;
  order: Order;
}

/**
 * The items the list shows. In menu order they are as the server sends
 * them: by category, then each category's own order. A search looks through
 * every category, so a dish is found without knowing where it is filed.
 */
export function listItems(menu: MenuEditor, filter: ListFilter, now: Date): EditorItem[] {
  const needle = filter.search.trim().toLowerCase();
  const rank = new Map(menu.categories.map((c, i) => [c.id, i]));
  const kept = menu.items
    .map((item, at) => ({ item, at }))
    .filter(({ item }) => {
      if (needle !== "") return item.name.toLowerCase().includes(needle);
      if (filter.scope === "all") return true;
      return filter.scope === "popular" ? item.popular : item.category_id === filter.scope;
    })
    .filter(({ item }) => filter.show === "all" || itemState(item, now) === filter.show);
  const by: Record<Order, (a: (typeof kept)[number], b: (typeof kept)[number]) => number> = {
    menu: (a, b) => (rank.get(a.item.category_id) ?? 0) - (rank.get(b.item.category_id) ?? 0) || a.at - b.at,
    name: (a, b) => a.item.name.localeCompare(b.item.name) || a.at - b.at,
    price: (a, b) => a.item.price.minor - b.item.price.minor || a.item.name.localeCompare(b.item.name),
  };
  return kept.sort(by[filter.order]).map(({ item }) => item);
}

/**
 * Whether an item can be moved up or down from the list: only when the list
 * is one category in menu order with nothing filtered out, because the order
 * saved is the category's whole order.
 */
export function canReorder(filter: ListFilter): filter is ListFilter & { scope: number } {
  return typeof filter.scope === "number" && filter.order === "menu" && filter.show === "all" && filter.search.trim() === "";
}

/** The list's title: "Menu items (68)". */
export function listTitle(count: number): string {
  return `Menu items (${count})`;
}

/** What the list says when it is empty, by why. */
export function emptyLine(menu: MenuEditor, filter: ListFilter): string {
  if (filter.search.trim() !== "") return `Nothing on the menu matches "${filter.search.trim()}".`;
  if (menu.items.length === 0) return menu.categories.length === 0 ? "No menu yet. Add a category, then the items in it." : "No items yet. Add the first one.";
  if (filter.show !== "all") return "No items like that here.";
  return filter.scope === "popular" ? "No item is marked popular yet." : "No items here yet.";
}

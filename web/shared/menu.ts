// The venue's menu on the client: where it is fetched from, when a cached
// copy is stale, and an index the register reads it through.
//
// The menu is fetched once and kept for good. It is not refreshed on a
// timer: the realtime stream names the current version on every connection
// and whenever it changes (a `menu` notice), and a cached copy of any other
// version is refetched. The server answers with an ETag, so the browser's
// own revalidation turns an unchanged refetch into a 304.

import type { MenuItemResponse, MenuResponse, ModifierGroupResponse } from "./api-types";
import type { RealtimeNotice } from "./realtime";

export const menuPath = "/api/v1/menu";
export const menuQueryKey = ["menu"] as const;

/**
 * Whether a notice makes the cached menu stale. Nothing cached is nothing
 * to invalidate: the first read fetches the current menu anyway.
 */
export function isMenuStale(notice: RealtimeNotice, cached: Pick<MenuResponse, "version"> | undefined): boolean {
  return notice.kind === "menu" && cached !== undefined && cached.version !== notice.version;
}

/** The menu, indexed for the register and the item modal. */
export interface MenuIndex {
  readonly menu: MenuResponse;
  /** Items of a category, in menu order; empty for an unknown category. */
  itemsIn(categoryId: number): readonly MenuItemResponse[];
  item(id: number): MenuItemResponse | undefined;
  /** An item's modifier groups, in the order the sheet shows them. */
  groupsFor(item: MenuItemResponse): readonly ModifierGroupResponse[];
}

const none: readonly never[] = [];

export function indexMenu(menu: MenuResponse): MenuIndex {
  const byCategory = new Map<number, MenuItemResponse[]>();
  const items = new Map<number, MenuItemResponse>();
  for (const item of menu.items) {
    items.set(item.id, item);
    const list = byCategory.get(item.category_id);
    if (list) list.push(item);
    else byCategory.set(item.category_id, [item]);
  }
  const groups = new Map(menu.modifier_groups.map((g) => [g.id, g]));

  return {
    menu,
    itemsIn: (categoryId) => byCategory.get(categoryId) ?? none,
    item: (id) => items.get(id),
    // A group id the menu does not carry is dropped rather than thrown on:
    // the server already leaves retired groups out, so this is a menu read
    // mid-edit, and the item is still worth selling.
    groupsFor: (item) =>
      item.modifier_groups.flatMap((id) => {
        const g = groups.get(id);
        return g ? [g] : [];
      }),
  };
}

/**
 * Whether an item is sold out at now: it carries the moment it is back on
 * sale, the start of the next trading day (ADR-109), and is sold out until
 * then. Compared with this device's clock, so it comes back the next
 * morning without a menu edit. An unreadable time is treated as on sale:
 * the till would rather ring up a dish than hide one it cannot explain.
 */
export function isSoldOut(item: { readonly sold_out_until?: string }, now: Date): boolean {
  if (!item.sold_out_until) return false;
  const until = Date.parse(item.sold_out_until);
  return Number.isFinite(until) && until > now.getTime();
}

/** Where a stored photo is served (ADR-110). Its name is its hash, so the URL never changes what it shows. */
export function photoUrl(name: string): string {
  return `/api/v1/menu/photos/${encodeURIComponent(name)}`;
}

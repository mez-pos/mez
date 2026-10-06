// What the menu screen shows for a category, a search and the diet filters
// (docs/design/screens/guest-menu.md). Pure, so each rule is tested without
// a phone.

import { ApiError, isUnreachable } from "../../shared/api";
import type { CategoryResponse, GuestItemResponse, GuestMenuResponse } from "../../shared/api-types";

export type DietFilter = "veg" | "vegan" | "gluten_free";

export const dietFilterWords: Readonly<Record<DietFilter, string>> = {
  veg: "Veg",
  vegan: "Vegan",
  gluten_free: "Gluten free",
};

export interface MenuFilter {
  /** A category's id, or null for every category. */
  readonly category: number | null;
  readonly search: string;
  readonly diets: readonly DietFilter[];
}

export const noFilter: MenuFilter = { category: null, search: "", diets: [] };

function suits(item: GuestItemResponse, diet: DietFilter): boolean {
  switch (diet) {
    case "veg":
      // A vegan dish is a vegetarian one.
      return item.diet === "veg" || item.diet === "vegan";
    case "vegan":
      return item.diet === "vegan";
    case "gluten_free":
      return item.gluten_free === true;
  }
}

/** The words of a search, lower case. A search of spaces is no search. */
function wordsOf(search: string): string[] {
  return search
    .toLowerCase()
    .split(/\s+/)
    .filter((w) => w !== "");
}

/**
 * The items to show, in the menu's own order, with what is sold out moved to
 * the end: a guest reads from the top, and the top should be what they can
 * have. A search matches when every word is in the name or the description.
 * The diet filters narrow together: Veg and Gluten free shows dishes that
 * are both.
 */
export function visibleItems(menu: Pick<GuestMenuResponse, "items">, filter: MenuFilter): GuestItemResponse[] {
  const words = wordsOf(filter.search);
  const shown = menu.items.filter((item) => {
    if (filter.category !== null && item.category_id !== filter.category) return false;
    if (!filter.diets.every((diet) => suits(item, diet))) return false;
    const text = `${item.name} ${item.description ?? ""}`.toLowerCase();
    return words.every((w) => text.includes(w));
  });
  return [...shown.filter((it) => !it.sold_out), ...shown.filter((it) => it.sold_out)];
}

/** The categories that have a dish in them, in the menu's order. An empty one is not offered. */
export function categoriesWithItems(menu: Pick<GuestMenuResponse, "items" | "categories">): CategoryResponse[] {
  const used = new Set(menu.items.map((it) => it.category_id));
  return menu.categories.filter((c) => used.has(c.id));
}

/** The diet filters worth offering: the ones at least one dish satisfies. */
export function offeredDietFilters(menu: Pick<GuestMenuResponse, "items">): DietFilter[] {
  return (["veg", "vegan", "gluten_free"] as const).filter((diet) => menu.items.some((it) => suits(it, diet)));
}

/** A filter with one diet switched on or off. */
export function toggleDiet(filter: MenuFilter, diet: DietFilter): MenuFilter {
  const diets = filter.diets.includes(diet) ? filter.diets.filter((d) => d !== diet) : [...filter.diets, diet];
  return { ...filter, diets };
}

/** The title over the dishes: the search when there is one, else the category, else "All dishes". */
export function headingFor(menu: Pick<GuestMenuResponse, "categories">, filter: MenuFilter): string {
  const search = filter.search.trim();
  if (search !== "") return `Results for "${search}"`;
  if (filter.category === null) return "All dishes";
  return menu.categories.find((c) => c.id === filter.category)?.name ?? "All dishes";
}

/** "18 dishes", "1 dish". */
export function countText(count: number): string {
  return count === 1 ? "1 dish" : `${count} dishes`;
}

/**
 * Whether the cards have a frame for a photo. They do once any dish on the
 * menu has one, so the grid is even; a menu with no photos at all has no
 * frames, since most venues start with none and a wall of empty frames is
 * not a menu. It is asked of the whole menu, not of what a filter left, so
 * the cards do not change shape as a guest searches.
 */
export function showsPhotoFrames(menu: Pick<GuestMenuResponse, "items">): boolean {
  return menu.items.some((it) => it.photo !== undefined && it.photo !== "");
}

/** Where a dish's photo is served to a guest. */
export function photoUrl(name: string): string {
  return `/order/api/photos/${encodeURIComponent(name)}`;
}

/**
 * Why the venue could not be read, as the screen that says so needs it:
 * the code leads nowhere, the venue could not be reached, or it answered
 * with something else.
 */
export type Problem = "unknown_code" | "collect_off" | "unreachable" | "failed";

export function problemOf(error: unknown): Problem {
  if (!(error instanceof ApiError)) return "failed";
  if (error.code === "table_code_unknown" || error.code === "pickup_code_unknown") return "unknown_code";
  if (error.code === "pickup_off") return "collect_off";
  // The relay answers 502, 503 and 504 for a venue it cannot reach, and 429
  // for one that is busy: all of them pass, and all of them say try again.
  if (isUnreachable(error) || error.status === 429) {
    return "unreachable";
  }
  return "failed";
}

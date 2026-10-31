// What the register decides without React: which items a search finds, and
// what ringing an item up sends (docs/design/screens/terminal-register.md).

import type { AddLine, CategoryResponse, LineModifier, MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";

/** Lowercase, accents stripped: "Café" and "cafe" are the same word. */
export function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
}

const words = (text: string): string[] =>
  fold(text)
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean);

/**
 * Items whose name has a word starting with every word of the query, in menu
 * order. "fl wh" finds Flat white; "white" finds it too; an empty query finds
 * nothing, because an empty search box means "show the category".
 */
export function searchItems(items: readonly MenuItemResponse[], query: string): MenuItemResponse[] {
  const wanted = words(query);
  if (wanted.length === 0) return [];
  return items.filter((item) => {
    const have = words(item.name);
    return wanted.every((w) => have.some((h) => h.startsWith(w)));
  });
}

/**
 * The modifiers an item rings up with when nobody chooses: each group's
 * defaults. Null when the defaults do not satisfy a group's bounds, which
 * means somebody has to choose, in the item modal.
 */
export function defaultModifiers(groups: readonly ModifierGroupResponse[]): LineModifier[] | null {
  const chosen: LineModifier[] = [];
  for (const group of groups) {
    const defaults = group.modifiers.filter((m) => m.is_default);
    if (defaults.length < group.min_select || defaults.length > group.max_select) return null;
    for (const m of defaults) chosen.push({ modifier_id: m.id, name: m.name, price: m.price });
  }
  return chosen;
}

/** Where an item card was tapped: the card itself, or its round "+". */
export type CardTap = "card" | "plus";

/**
 * What a tap on an item card does (ADR-154): the modifiers to ring it up with
 * at once, or null to open the item modal. The "+" adds with the defaults,
 * for the plain order in a rush; the card opens the modal whenever there is
 * anything to choose. An item with no choices is rung up either way, and one
 * whose defaults leave a choice to make opens the modal either way.
 */
export function ringsUpWith(groups: readonly ModifierGroupResponse[], tap: CardTap): LineModifier[] | null {
  if (tap === "card" && groups.length > 0) return null;
  return defaultModifiers(groups);
}

/**
 * The AddLine command for an item, with its snapshot facts copied: one of it
 * on its own course, unless the item modal chose how many and which course.
 */
export function addLineFor(item: MenuItemResponse, modifiers: readonly LineModifier[], chosen: { qty?: number; course?: number } = {}): AddLine {
  return {
    item_id: item.id,
    name: item.name,
    qty: chosen.qty ?? 1,
    unit_price: item.price,
    modifiers: [...modifiers],
    course: chosen.course ?? item.course,
    station: item.station,
    tax_rate: item.tax_rate,
    ...(item.hsn ? { hsn: item.hsn } : {}),
    ...(item.tax_exempt ? { tax_exempt: true } : {}),
  };
}

/** How many choices an item offers, for its tile. */
/** "1 choice", "5 choices" (TILL-20). */
export function choicesText(n: number): string {
  return n === 1 ? "1 choice" : `${n} choices`;
}

/**
 * What a dish card is called for a screen reader: its name first, then its
 * price and what marks it, "Filter coffee, ₹60.00, vegetarian, gluten free,
 * popular, 5 choices", where reading the card out began with "Popular
 * Vegetarian" (TILL-14, ADR-393).
 */
export function cardLabel(item: Pick<MenuItemResponse, "name" | "diet" | "gluten_free" | "popular">, price: string, choices: number, soldOut: boolean): string {
  const diet = item.diet === "veg" ? "vegetarian" : item.diet === "non_veg" ? "non-vegetarian" : item.diet === "vegan" ? "vegan" : null;
  const marks = [price, diet, item.gluten_free ? "gluten free" : null, item.popular ? "popular" : null, choices > 0 ? choicesText(choices) : null];
  return [item.name, ...(soldOut ? ["sold out"] : marks)].filter(Boolean).join(", ");
}

export function choiceCount(groups: readonly ModifierGroupResponse[]): number {
  return groups.reduce((n, g) => n + g.modifiers.length, 0);
}

/** The dietary filters on the order screen; any combination may be on. */
export interface DietFilter {
  readonly veg: boolean;
  readonly nonVeg: boolean;
  readonly vegan: boolean;
  readonly glutenFree: boolean;
}

export const noDietFilter: DietFilter = { veg: false, nonVeg: false, vegan: false, glutenFree: false };

export function dietFilterActive(f: DietFilter): boolean {
  return f.veg || f.nonVeg || f.vegan || f.glutenFree;
}

/** Each filter's word, in the order the pills stand. */
export const dietLabels: Readonly<Record<keyof DietFilter, string>> = { veg: "Veg", nonVeg: "Non-veg", vegan: "Vegan", glutenFree: "Gluten free" };

/**
 * What the one Diet button says below xl, where the four pills wait behind it:
 * "Diet" with none on, the filter's word with one, how many with more.
 */
export function dietSummary(f: DietFilter): string {
  const on = (Object.keys(dietLabels) as (keyof DietFilter)[]).filter((key) => f[key]);
  const [only] = on;
  if (only === undefined) return "Diet";
  return on.length === 1 ? `Diet: ${dietLabels[only]}` : `Diet: ${on.length} on`;
}

/**
 * The items a filter lets through. Veg, Non-Veg and Vegan add up (veg and vegan
 * on shows both); Gluten Free narrows whatever they leave. Nothing on shows all.
 */
export function filterItems(items: readonly MenuItemResponse[], f: DietFilter): MenuItemResponse[] {
  const diets = f.veg || f.nonVeg || f.vegan;
  return items.filter((item) => {
    if (f.glutenFree && !item.gluten_free) return false;
    if (!diets) return true;
    return (f.veg && item.diet === "veg") || (f.nonVeg && item.diet === "non_veg") || (f.vegan && item.diet === "vegan");
  });
}

/** The items marked popular, in menu order: the sidebar's Popular. */
export function popularItems(items: readonly MenuItemResponse[]): MenuItemResponse[] {
  return items.filter((item) => item.popular);
}

export type CategoryIconKey = "utensils" | "star" | "leaf" | "soup" | "pizza" | "burger" | "salad" | "coffee" | "dessert";

/**
 * Which icon a category's name earns in the sidebar, from the words in it. The
 * menu is the venue's own, so this guesses and a plain fork and knife answers
 * for what it cannot place.
 */
export function categoryIconKey(name: string): CategoryIconKey {
  // By word, from its start: "tea" is a drink and "steak" is not.
  const named = words(name);
  const has = (...parts: string[]) => parts.some((p) => named.some((w) => w.startsWith(p)));
  if (has("pizza")) return "pizza";
  if (has("burger", "sandwich", "wrap", "grill")) return "burger";
  if (has("salad")) return "salad";
  if (has("coffee", "tea", "chai", "beverage", "drink", "juice", "shake", "smoothie", "mocktail")) return "coffee";
  if (has("dessert", "sweet", "cake", "bakery", "pastry", "cream")) return "dessert";
  if (has("soup", "main", "curry", "rice", "noodle", "pasta", "biryani", "meal")) return "soup";
  if (has("starter", "snack", "appetizer", "appetiser", "bite", "side")) return "leaf";
  return "utensils";
}

/** What the order screen's menu is showing: everything, the popular items, or one category. */
export type MenuView = "all" | "popular" | number;

export interface SidebarEntry {
  readonly view: MenuView;
  readonly label: string;
  readonly icon: CategoryIconKey;
  readonly count: number;
}

/** All Items, Popular when anything is popular, then each category in menu order. */
export function sidebarEntries(categories: readonly CategoryResponse[], items: readonly MenuItemResponse[]): SidebarEntry[] {
  const popular = popularItems(items).length;
  return [
    { view: "all", label: "All items", icon: "utensils", count: items.length },
    ...(popular > 0 ? [{ view: "popular" as const, label: "Popular", icon: "star" as const, count: popular }] : []),
    ...categories.map((c) => ({
      view: c.id,
      label: c.name,
      icon: categoryIconKey(c.name),
      count: items.filter((i) => i.category_id === c.id).length,
    })),
  ];
}

/** The items a view shows, in menu order. */
export function itemsForView(view: MenuView, items: readonly MenuItemResponse[]): MenuItemResponse[] {
  if (view === "all") return [...items];
  if (view === "popular") return popularItems(items);
  return items.filter((i) => i.category_id === view);
}

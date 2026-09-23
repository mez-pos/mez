// What the register decides without React: which items a search finds, and
// what ringing an item up sends (docs/design/screens/terminal-register.md).

import type { AddLine, LineModifier, MenuItemResponse, ModifierGroupResponse } from "../../../shared/api-types";

/** Lowercase, accents stripped: "Café" and "cafe" are the same word. */
export function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
}

const words = (text: string): string[] => fold(text).split(/[^\p{L}\p{N}]+/u).filter(Boolean);

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
 * means somebody has to choose, on the modifier sheet.
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

/** The AddLine command for one of an item, with its snapshot facts copied. */
export function addLineFor(item: MenuItemResponse, modifiers: readonly LineModifier[]): AddLine {
  return {
    item_id: item.id,
    name: item.name,
    qty: 1,
    unit_price: item.price,
    modifiers: [...modifiers],
    course: item.course,
    station: item.station,
    tax_rate: item.tax_rate,
    ...(item.hsn ? { hsn: item.hsn } : {}),
    ...(item.tax_exempt ? { tax_exempt: true } : {}),
  };
}

/** How many choices an item offers, for its tile. */
export function choiceCount(groups: readonly ModifierGroupResponse[]): number {
  return groups.reduce((n, g) => n + g.modifiers.length, 0);
}

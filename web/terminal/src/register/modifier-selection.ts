// What the item modal has chosen, and the rules for choosing, without React.
// A group's bounds are min_select and max_select from the menu; the domain
// takes the result as the line's modifiers, in group then menu order.

import type { LineModifier, Money, ModifierGroupResponse } from "../../../shared/api-types";

/** Chosen modifier ids per group id, each list in the menu's order. */
export type Selection = ReadonlyMap<number, readonly number[]>;

/**
 * Where the sheet starts: the line's current modifiers when editing, each
 * group's defaults for a new line. A modifier the menu no longer offers is
 * dropped, so the sheet can only show what can be chosen.
 */
export function initialSelection(groups: readonly ModifierGroupResponse[], current?: readonly LineModifier[]): Selection {
  const had = current ? new Set(current.map((m) => m.modifier_id)) : null;
  return new Map(groups.map((g) => [g.id, g.modifiers.filter((m) => (had ? had.has(m.id) : m.is_default)).map((m) => m.id)]));
}

/**
 * Taps a modifier. A pick-one group swaps to it; tapping the chosen one
 * clears it only when the group is optional. A pick-several group adds or
 * removes it, and refuses a new one when full rather than dropping another
 * the person chose on purpose.
 */
export function toggle(selection: Selection, group: ModifierGroupResponse, modifierId: number): Selection {
  const chosen = selection.get(group.id) ?? [];
  const on = chosen.includes(modifierId);
  let next: readonly number[];
  if (group.max_select === 1) {
    if (on) next = group.min_select >= 1 ? chosen : [];
    else next = [modifierId];
  } else if (on) {
    next = chosen.filter((id) => id !== modifierId);
  } else if (chosen.length >= group.max_select) {
    return selection;
  } else {
    const order = group.modifiers.map((m) => m.id);
    next = [...chosen, modifierId].sort((a, b) => order.indexOf(a) - order.indexOf(b));
  }
  if (next === chosen) return selection;
  const out = new Map(selection);
  out.set(group.id, next);
  return out;
}

export interface GroupState {
  readonly count: number;
  readonly satisfied: boolean;
  readonly full: boolean;
  /** "Choose 1", "Up to 3", "Choose 2 to 3". */
  readonly rule: string;
}

export function groupState(group: ModifierGroupResponse, selection: Selection): GroupState {
  const count = selection.get(group.id)?.length ?? 0;
  const { min_select: min, max_select: max } = group;
  const rule = min === max ? `Choose ${min}` : min === 0 ? `Up to ${max}` : `Choose ${min} to ${max}`;
  return { count, satisfied: count >= min && count <= max, full: count >= max, rule };
}

/**
 * What a group asks for, beside its name (D04): "Required" for the plain
 * choose-one, "Select multiple" when every choice may be taken, and the rule
 * in words where neither says enough.
 */
export function groupHint(group: ModifierGroupResponse): string {
  const { min_select: min, max_select: max } = group;
  const rule = groupState(group, new Map()).rule;
  if (min > 0) return min === 1 && max === 1 ? "Required" : `Required · ${rule}`;
  if (max === 1) return "Optional";
  return max >= group.modifiers.length ? "Select multiple" : rule;
}

/** The first group still short of its minimum, which is what the sheet names. */
export function firstUnmet(groups: readonly ModifierGroupResponse[], selection: Selection): ModifierGroupResponse | null {
  return groups.find((g) => !groupState(g, selection).satisfied) ?? null;
}

/** The line's modifiers, in group order and then menu order. */
export function toModifiers(groups: readonly ModifierGroupResponse[], selection: Selection): LineModifier[] {
  return groups.flatMap((g) => {
    const chosen = new Set(selection.get(g.id) ?? []);
    return g.modifiers.filter((m) => chosen.has(m.id)).map((m) => ({ modifier_id: m.id, name: m.name, price: m.price }));
  });
}

/**
 * One of the item with these modifiers, for the sheet's live figure. Display
 * only: the server prices the line. Integer minor units, never a float.
 */
export function unitPrice(base: Money, modifiers: readonly LineModifier[]): Money {
  return { currency: base.currency, minor: modifiers.reduce((sum, m) => sum + m.price.minor, base.minor) };
}

/** The whole line as the item modal's button shows it: one of it, times how many. Display only. */
export function lineTotal(base: Money, modifiers: readonly LineModifier[], qty: number): Money {
  return { currency: base.currency, minor: unitPrice(base, modifiers).minor * qty };
}

/** Whether two modifier lists are the same choice, in the same order. */
export function sameModifiers(a: readonly LineModifier[] | undefined, b: readonly LineModifier[] | undefined): boolean {
  const x = a ?? [];
  const y = b ?? [];
  return x.length === y.length && x.every((m, i) => m.modifier_id === y[i]?.modifier_id);
}

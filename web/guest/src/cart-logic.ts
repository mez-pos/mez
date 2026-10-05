// The guest's cart (docs/design/screens/guest-cart.md), without React. It is
// the phone's own until an order is placed: a list of dishes, the choices
// made on each, and how many.
//
// A line keeps ids and a count and nothing else. Names and prices are read
// from the menu each time the cart is shown, so a price that changed while
// the phone was in a pocket is never shown as it was. Money is integer minor
// units throughout, never a float.

import type { GuestItemResponse, GuestMenuResponse, LineModifier, Money } from "../../shared/api-types";
import { firstUnmet, initialSelection, toModifiers, unitPrice } from "../../shared/modifier-selection";

/** The most of one line. More than any table orders of one dish, and a bound on what a stored cart can claim. */
export const MAX_QTY = 20;

/** The most lines a cart holds, for the same reason. */
export const MAX_LINES = 50;

export interface CartLine {
  readonly item_id: number;
  /** The chosen modifiers' ids, in the menu's order. */
  readonly modifier_ids: readonly number[];
  readonly qty: number;
}

export type Cart = readonly CartLine[];

export const emptyCart: Cart = [];

/** A line is one dish with one set of choices: the same again is more of it, not another line. */
export function lineKey(line: Pick<CartLine, "item_id" | "modifier_ids">): string {
  return `${line.item_id}:${line.modifier_ids.join(",")}`;
}

/** Adds to the cart, onto the line with the same dish and choices if there is one. */
export function addLine(cart: Cart, line: CartLine): Cart {
  const qty = clampQty(line.qty);
  if (qty === 0) return cart;
  const key = lineKey(line);
  const at = cart.findIndex((l) => lineKey(l) === key);
  if (at >= 0) {
    return cart.map((l, i) => (i === at ? { ...l, qty: Math.min(MAX_QTY, l.qty + qty) } : l));
  }
  if (cart.length >= MAX_LINES) return cart;
  return [...cart, { item_id: line.item_id, modifier_ids: [...line.modifier_ids], qty }];
}

/** Sets how many of a line. None takes the line out. */
export function setQty(cart: Cart, key: string, qty: number): Cart {
  const next = clampQty(qty);
  if (next === 0) return cart.filter((l) => lineKey(l) !== key);
  return cart.map((l) => (lineKey(l) === key ? { ...l, qty: next } : l));
}

function clampQty(qty: number): number {
  if (!Number.isFinite(qty)) return 0;
  return Math.max(0, Math.min(MAX_QTY, Math.trunc(qty)));
}

/** A line as the cart shows it, priced from the menu as it is now. */
export interface PricedLine {
  readonly key: string;
  readonly item: GuestItemResponse;
  readonly modifiers: readonly LineModifier[];
  /** The choices in words, "Oat, Extra shot", or empty. */
  readonly options: string;
  readonly qty: number;
  readonly unit: Money;
  readonly total: Money;
}

interface MenuLookup {
  item(id: number): GuestItemResponse | undefined;
  groupsOf(item: GuestItemResponse): GuestMenuResponse["modifier_groups"];
}

function lookup(menu: GuestMenuResponse): MenuLookup {
  const items = new Map(menu.items.map((it) => [it.id, it]));
  const groups = new Map(menu.modifier_groups.map((g) => [g.id, g]));
  return {
    item: (id) => items.get(id),
    groupsOf: (item) => item.modifier_groups.flatMap((id) => groups.get(id) ?? []),
  };
}

/** The groups a dish's choices are made from, in the dish's order. */
export function groupsFor(menu: GuestMenuResponse, item: GuestItemResponse): GuestMenuResponse["modifier_groups"] {
  return lookup(menu).groupsOf(item);
}

/**
 * Prices one line from the menu, or says it can no longer be ordered: the
 * dish is gone or sold out, a choice it was made with is gone, or its
 * choices no longer satisfy a group (a group became required since).
 */
function price(look: MenuLookup, line: CartLine): PricedLine | null {
  const item = look.item(line.item_id);
  if (!item || item.sold_out) return null;
  const groups = look.groupsOf(item);
  const offered = new Set(groups.flatMap((g) => g.modifiers.map((m) => m.id)));
  if (!line.modifier_ids.every((id) => offered.has(id))) return null;

  const chosen = line.modifier_ids.map((id) => ({ modifier_id: id, name: "", price: item.price }));
  const selection = initialSelection(groups, chosen);
  if (firstUnmet(groups, selection) !== null) return null;

  const modifiers = toModifiers(groups, selection);
  const unit = unitPrice(item.price, modifiers);
  return {
    key: lineKey(line),
    item,
    modifiers,
    options: modifiers.map((m) => m.name).join(", "),
    qty: line.qty,
    unit,
    total: { currency: unit.currency, minor: unit.minor * line.qty },
  };
}

export interface Reconciled {
  /** The cart with what can no longer be ordered taken out. */
  readonly cart: Cart;
  readonly lines: readonly PricedLine[];
  /** The dishes taken out, by name where the menu still has one. */
  readonly dropped: readonly string[];
}

/**
 * Holds the cart against the menu as it is now. A line that can no longer
 * be ordered is taken out and named, so the guest is told and does not find
 * out at the counter. What is left is priced at today's prices.
 */
export function reconcile(cart: Cart, menu: GuestMenuResponse): Reconciled {
  const look = lookup(menu);
  const kept: CartLine[] = [];
  const lines: PricedLine[] = [];
  const dropped: string[] = [];
  for (const line of cart) {
    const priced = price(look, line);
    if (priced) {
      kept.push(line);
      lines.push(priced);
    } else {
      dropped.push(look.item(line.item_id)?.name ?? "A dish");
    }
  }
  return { cart: kept.length === cart.length ? cart : kept, lines, dropped };
}

/** How many dishes are in the cart, counting each one of a line. */
export function countOf(cart: Cart): number {
  return cart.reduce((sum, l) => sum + l.qty, 0);
}

/** What the dishes come to. Before any tax, which the venue works out on the bill. */
export function totalOf(lines: readonly PricedLine[], currency: string): Money {
  return { currency, minor: lines.reduce((sum, l) => sum + l.total.minor, 0) };
}

/** "3 items", "1 item". */
export function itemsText(count: number): string {
  return count === 1 ? "1 item" : `${count} items`;
}

/** "Flat white was", "Flat white and Latte were", "Flat white, Latte and Idli were", for the notice. */
export function droppedText(names: readonly string[]): string {
  if (names.length === 0) return "";
  const list = names.length === 1 ? names[0] : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
  const verb = names.length === 1 ? "is" : "are";
  return `${list} ${verb} no longer available and ${names.length === 1 ? "was" : "were"} taken out of your cart.`;
}

// Storage. The cart is kept on the phone by table code, so a reload or a
// locked screen does not lose it, and a phone that scans another table's
// sticker starts clean.

export function storageKey(code: string): string {
  return `mez.cart.${code}`;
}

export function serialize(cart: Cart): string {
  return JSON.stringify(cart.map((l) => ({ i: l.item_id, m: l.modifier_ids, q: l.qty })));
}

/**
 * Reads a stored cart. What is in storage is not trusted: another version of
 * the app wrote it, or something else did. Anything that is not a line is
 * left out, and nothing here can throw.
 */
export function parse(text: string | null): Cart {
  if (!text) return emptyCart;
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return emptyCart;
  }
  if (!Array.isArray(raw)) return emptyCart;
  let cart: Cart = emptyCart;
  for (const entry of raw.slice(0, MAX_LINES)) {
    if (typeof entry !== "object" || entry === null) continue;
    const { i, m, q } = entry as { i?: unknown; m?: unknown; q?: unknown };
    if (!isId(i) || !Array.isArray(m) || !m.every(isId) || typeof q !== "number") continue;
    cart = addLine(cart, { item_id: i, modifier_ids: m, qty: q });
  }
  return cart;
}

function isId(value: unknown): value is number {
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0;
}

// What the menu editor says about the menu (backoffice-menu.md). The menu is
// the server's; this picks from it, words it, and turns the item form into
// the request the server checks.

import { ApiError } from "../../../shared/api";
import type {
  EditorCategory,
  EditorCourse,
  EditorItem,
  EditorStation,
  EditorTaxClass,
  MenuEditor,
  MenuItemInput,
  Money,
} from "../../../shared/api-types";
import { amountInputText, parseAmount } from "../../../shared/money";

/** The items of one category, in their order. */
export function itemsOf(menu: MenuEditor, categoryId: number): EditorItem[] {
  return menu.items.filter((i) => i.category_id === categoryId);
}

/** The category to show: the one asked for if it still exists, else the first. */
export function selectedCategory(menu: MenuEditor, wanted: number | null): EditorCategory | null {
  return menu.categories.find((c) => c.id === wanted) ?? menu.categories[0] ?? null;
}

/**
 * ids with the one at index moved by delta places (-1 up, +1 down), as a new
 * list. Null when it is already at that end.
 */
export function moved(ids: readonly number[], index: number, delta: -1 | 1): number[] | null {
  const to = index + delta;
  if (index < 0 || index >= ids.length || to < 0 || to >= ids.length) return null;
  const next = [...ids];
  [next[index], next[to]] = [next[to]!, next[index]!];
  return next;
}

/**
 * A tax rate, thousandths of a percent (ADR-049), as text: 5000 is "5%",
 * 2500 is "2.5%", 12375 is "12.375%". Integer arithmetic only.
 */
export function rateText(rate: number): string {
  const whole = Math.trunc(rate / 1000);
  const rest = String(rate % 1000).padStart(3, "0").replace(/0+$/, "");
  return rest ? `${whole}.${rest}%` : `${whole}%`;
}

/**
 * "Food, 5%", or "Water, exempt". A name that already says its rate
 * ("Restaurant 5%") is left alone rather than said twice.
 */
export function taxClassText(t: EditorTaxClass): string {
  const rate = t.exempt ? "exempt" : rateText(t.rate);
  return t.name.toLowerCase().includes(rate) ? t.name : `${t.name}, ${rate}`;
}

/** "1 Mains". */
export function courseText(c: EditorCourse): string {
  return `${c.number} ${c.name}`;
}

/**
 * What a select may offer: the active rows, and the one already chosen even
 * if it has since been retired, so an old item's form still shows what it
 * has. The server refuses a retired choice on save, with its reason.
 */
export function choices<T extends { id: number; active: boolean }>(rows: readonly T[], current: number): T[] {
  return rows.filter((r) => r.active || r.id === current);
}

export interface ItemForm {
  name: string;
  categoryId: number;
  /** As typed, in major units: "180.50". */
  price: string;
  hsn: string;
  taxClassId: number;
  stationId: number;
  courseId: number;
}

/** A new item's form: in the given category, with the first station, course and tax class chosen. */
export function newItemForm(menu: MenuEditor, categoryId: number): ItemForm {
  return {
    name: "",
    categoryId,
    price: "",
    hsn: "",
    taxClassId: menu.tax_classes.find((t) => t.active)?.id ?? 0,
    stationId: menu.stations.find((s) => s.active)?.id ?? 0,
    courseId: menu.courses[0]?.id ?? 0,
  };
}

export function itemFormFrom(item: EditorItem): ItemForm {
  return {
    name: item.name,
    categoryId: item.category_id,
    price: amountInputText(item.price),
    hsn: item.hsn,
    taxClassId: item.tax_class_id,
    stationId: item.station_id,
    courseId: item.course_id,
  };
}

export type ItemField = "name" | "category_id" | "price" | "hsn" | "tax_class_id" | "station_id" | "course_id" | "photo";

/** A refusal as the form shows it: under its field, or over the form. */
export interface Problem {
  field: ItemField | "ids" | null;
  message: string;
}

export const unreadablePrice = "Type a price like 180 or 180.50.";

/** The request for a form, or the problem that stops it being sent. */
export function toItemInput(form: ItemForm, currency: string): { input: MenuItemInput } | { problem: Problem } {
  const minor = parseAmount(form.price, currency);
  if (minor === null) return { problem: { field: "price", message: unreadablePrice } };
  return {
    input: {
      category_id: form.categoryId,
      name: form.name.trim(),
      price: { minor, currency },
      hsn: form.hsn.trim(),
      tax_class_id: form.taxClassId,
      station_id: form.stationId,
      course_id: form.courseId,
    },
  };
}

/**
 * The price change an edit would make, for the notice under the price: what
 * open tickets keep and what new lines ring up at. Null for a new item, an
 * unchanged price, or one the form cannot read yet.
 */
export function priceChange(saved: EditorItem | null, typed: string): { was: Money; now: Money } | null {
  if (!saved) return null;
  const minor = parseAmount(typed, saved.price.currency);
  if (minor === null || minor === saved.price.minor) return null;
  return { was: saved.price, now: { minor, currency: saved.price.currency } };
}

const itemFields: readonly string[] = ["name", "category_id", "price", "hsn", "tax_class_id", "station_id", "course_id", "ids", "photo"];

export function problemOf(err: unknown): Problem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the mez server. Check this computer's connection." };
    if (err.code === "forbidden") return { field: null, message: "Only someone who can edit the menu can do this." };
    if (err.code === "menu_order_stale") {
      return { field: null, message: "The menu changed on another screen. It has been reloaded; move it again." };
    }
    const field = itemFields.includes(err.field ?? "") ? (err.field as Problem["field"]) : null;
    return { field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}

/** Names by id, for the table's columns. */
export function namesById(menu: MenuEditor): {
  taxClass: (id: number) => string;
  station: (id: number) => string;
  course: (id: number) => string;
} {
  const tax = new Map(menu.tax_classes.map((t) => [t.id, taxClassText(t)]));
  const stations = new Map(menu.stations.map((s: EditorStation) => [s.id, s.active ? s.name : `${s.name} (retired)`]));
  const courses = new Map(menu.courses.map((c) => [c.id, courseText(c)]));
  return {
    taxClass: (id) => tax.get(id) ?? "Unknown",
    station: (id) => stations.get(id) ?? "Unknown",
    course: (id) => courses.get(id) ?? "Unknown",
  };
}

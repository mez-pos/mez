// What the Stock screen says about stock (backoffice-stock.md). The ledger
// and its rules are the server's (ADR-111, ADR-112); this words them and
// turns the forms into the requests the server checks.

import { ApiError } from "../../../shared/api";
import type { RecipePart, StockItem, StockItemInput, StockMovement, StockMovementInput, StockUnit, WasteReason } from "../../../shared/api-types";
import { amountInputText, parseAmount } from "../../../shared/money";
import { formatQty, parseQty, qtyInputText } from "../../../shared/stock-qty";

/** The movements a person enters; sold is the till's. */
export type HandKind = "received" | "wasted" | "counted" | "adjusted";

export const reasons: readonly { value: WasteReason; label: string }[] = [
  { value: "expired", label: "Expired" },
  { value: "spoiled", label: "Spoiled" },
  { value: "spilled", label: "Spilled" },
  { value: "dropped", label: "Dropped" },
  { value: "other", label: "Other" },
];

export const units: readonly { value: StockUnit; label: string }[] = [
  { value: "g", label: "Grams" },
  { value: "ml", label: "Millilitres" },
  { value: "each", label: "Each" },
];

/** A refusal as a form shows it: a field, a recipe row's field, or over the form. */
export interface Problem {
  field: string | null;
  row?: { index: number; field: "stock_item_id" | "qty" };
  message: string;
}

const unreadable = "Type an amount like 250 or 1.5.";

export interface MovementForm {
  qty: string;
  reason: WasteReason | "";
  note: string;
}

export function emptyMovementForm(): MovementForm {
  return { qty: "", reason: "", note: "" };
}

/** The request for a movement form, or the problem that stops it being sent. */
export function toMovementInput(kind: HandKind, form: MovementForm): { input: StockMovementInput } | { problem: Problem } {
  const qty = parseQty(form.qty, kind === "adjusted");
  if (qty === null) {
    return { problem: { field: "qty", message: kind === "adjusted" ? "Type a change like 500 or -500." : unreadable } };
  }
  const input: StockMovementInput = { kind, qty };
  if (kind === "wasted" && form.reason) input.reason = form.reason;
  if (form.note.trim()) input.note = form.note.trim();
  return { input };
}

/** What a count would record against what the ledger says, or null while it cannot be read. */
export function countDelta(typed: string, onHand: number): number | null {
  const counted = parseQty(typed);
  return counted === null ? null : counted - onHand;
}

/** A movement in words for the history: "Received", "Wasted: spilled", "Counted 4,700 g". */
export function movementText(m: StockMovement, unit: StockUnit): string {
  switch (m.kind) {
    case "received":
      return "Received";
    case "wasted":
      return m.reason ? `Wasted: ${m.reason}` : "Wasted";
    case "counted":
      return m.counted === undefined ? "Counted" : `Counted ${formatQty(m.counted, unit)}`;
    case "adjusted":
      return "Corrected";
    case "sold":
      return "Sold";
  }
}

export interface ItemForm {
  name: string;
  unit: StockUnit;
  low: string;
  /** Counted, blind, as each shift closes (ADR-114). */
  countAtClose: boolean;
  /** What a kilogram, a litre or one of it costs, as typed; empty for not known (ADR-124). */
  cost: string;
}

/** What a cost is quoted for, by the item's unit: in full for the form, short for a table. */
export const costMeasures: Readonly<Record<StockUnit, { full: string; short: string }>> = {
  g: { full: "a kilogram", short: "kg" },
  ml: { full: "a litre", short: "l" },
  each: { full: "one", short: "each" },
};

export function emptyItemForm(): ItemForm {
  return { name: "", unit: "g", low: "", countAtClose: false, cost: "" };
}

export function itemFormFrom(it: StockItem): ItemForm {
  return {
    name: it.name,
    unit: it.unit,
    low: it.low_level === 0 ? "" : qtyInputText(it.low_level),
    countAtClose: it.count_at_close,
    cost: it.cost.minor === 0 ? "" : amountInputText(it.cost),
  };
}

/** The request for an item form; an empty low-stock level is none, an empty cost not known. */
export function toItemInput(form: ItemForm, currency: string): { input: StockItemInput } | { problem: Problem } {
  const low = form.low.trim() === "" ? 0 : parseQty(form.low);
  if (low === null) return { problem: { field: "low_level", message: unreadable } };
  const cost = form.cost.trim() === "" ? 0 : parseAmount(form.cost, currency);
  if (cost === null) return { problem: { field: "cost", message: "Type a cost like 60 or 1250.50." } };
  return { input: { name: form.name.trim(), unit: form.unit, low_level: low, count_at_close: form.countAtClose, cost: { minor: cost, currency } } };
}

/** What a recipe takes, in words: "Beans 18 g, Milk 200 ml"; empty for none. */
export function recipeText(parts: readonly RecipePart[], stock: ReadonlyMap<number, StockItem>): string {
  return parts
    .map((p) => {
      const it = stock.get(p.stock_item_id);
      return it ? `${it.name} ${formatQty(p.qty, it.unit)}` : `Unknown ${p.qty / 1000}`;
    })
    .join(", ");
}

export interface RecipeRow {
  key: string;
  stockItemId: number;
  qty: string;
}

let rows = 0;

export function newRecipeRow(stockItemId = 0): RecipeRow {
  rows += 1;
  return { key: `row-${rows}`, stockItemId, qty: "" };
}

export function recipeRowsFrom(parts: readonly RecipePart[]): RecipeRow[] {
  return parts.map((p) => ({ key: `part-${p.stock_item_id}`, stockItemId: p.stock_item_id, qty: qtyInputText(p.qty) }));
}

/** The parts for a recipe form, or the row whose amount cannot be read. */
export function toParts(form: readonly RecipeRow[]): { parts: RecipePart[] } | { problem: Problem } {
  const parts: RecipePart[] = [];
  for (const [index, r] of form.entries()) {
    const qty = parseQty(r.qty);
    if (qty === null) return { problem: { field: null, row: { index, field: "qty" }, message: unreadable } };
    parts.push({ stock_item_id: r.stockItemId, qty });
  }
  return { parts };
}

const rowField = /^parts\.(\d+)\.(stock_item_id|qty)$/;

export function problemOf(err: unknown): Problem {
  if (err instanceof ApiError) {
    if (err.unreachable) return { field: null, message: "Can't reach the mez server. Check this computer's connection." };
    if (err.code === "forbidden") return { field: null, message: "Only someone who can manage stock can do this." };
    const row = rowField.exec(err.field ?? "");
    if (row) return { field: null, row: { index: Number(row[1]), field: row[2] as "stock_item_id" | "qty" }, message: err.message };
    return { field: err.field, message: err.message };
  }
  return { field: null, message: "Something went wrong. Try again." };
}

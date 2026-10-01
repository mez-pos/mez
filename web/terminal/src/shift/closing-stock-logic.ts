// The till's closing stock count (terminal-close.md, ADR-114): every item
// on the list, counted blind and sent together. The ledger's figure is not
// on the till until the count is in.

import type { ClosingStockCount, ClosingStockItem, ClosingStockSheet } from "../../../shared/api-types";
import { parseQty } from "../../../shared/stock-qty";

/** The counts to send, or the first item whose count cannot be read. */
export function toCounts(
  items: readonly ClosingStockItem[],
  typed: Readonly<Record<number, string>>,
): { counts: ClosingStockCount[] } | { problem: { itemId: number; message: string } } {
  const counts: ClosingStockCount[] = [];
  for (const it of items) {
    const text = typed[it.id] ?? "";
    const qty = parseQty(text);
    if (qty === null) {
      return {
        problem: { itemId: it.id, message: text.trim() === "" ? `Count the ${it.name.toLowerCase()} too.` : "Type an amount like 250 or 1.5." },
      };
    }
    counts.push({ stock_item_id: it.id, qty });
  }
  return { counts };
}

/**
 * Whether the stock count still stands between the till and the close: a
 * list to count that is not counted yet. With nothing on the list, or the
 * sheet not read, it does not.
 */
export function stockCountOwed(sheet: ClosingStockSheet | undefined): boolean {
  return sheet !== undefined && !sheet.done && sheet.items.length > 0;
}

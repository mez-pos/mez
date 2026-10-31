// The words for the orders still open as the day is closed (TBS-2,
// ADR-384). They are not closed with the shift: they go on into the next
// one. But once the drawer is counted every till waits for the close, so
// close of day says so before the count, while they can still be settled.

import type { DayOrderResponse } from "../../../shared/api-types";
import { tableText } from "../bills/orders-logic";

/** How many places are named before the rest are only counted. */
const namedPlaces = 3;

/**
 * The orders the Orders screen's Open tab shows, the server's word for each:
 * a table's ticket paid in full and kept for the next round is paid, not
 * open (ADR-383). They are what the close counts as carried.
 */
export function openOrders(orders: readonly DayOrderResponse[]): DayOrderResponse[] {
  return orders.filter((o) => o.status === "open");
}

/** "3 orders are still open", or null with none. */
export function openOrdersTitle(n: number): string | null {
  if (n <= 0) return null;
  return n === 1 ? "1 order is still open" : `${n} orders are still open`;
}

/**
 * Where they are: tables by the floor's name, orders to collect by their
 * code, the rest counted at the counter. "T5, T7 and 1 at the counter".
 */
export function openOrdersWhere(orders: readonly Pick<DayOrderResponse, "table_id" | "pickup_code">[], names: ReadonlyMap<number, string>): string {
  const named = orders.filter((o) => o.table_id || o.pickup_code).map((o) => tableText(o.table_id, names, o.pickup_code));
  const counter = orders.length - named.length;
  const parts = named.slice(0, namedPlaces);
  if (named.length > namedPlaces) parts.push(`${named.length - namedPlaces} more`);
  if (counter > 0) parts.push(`${counter} at the counter`);
  return listText(parts);
}

/** The Submit count confirm's line on what stays open; null with none, or while none are known. */
export function carryWords(n: number | null): string | null {
  if (n === null || n <= 0) return null;
  return n === 1 ? "1 order is still open. It carries into the next shift." : `${n} orders are still open. They carry into the next shift.`;
}

/** "a", "a and b", "a, b and c". */
function listText(parts: readonly string[]): string {
  if (parts.length <= 1) return parts.join("");
  return `${parts.slice(0, -1).join(", ")} and ${parts[parts.length - 1]}`;
}

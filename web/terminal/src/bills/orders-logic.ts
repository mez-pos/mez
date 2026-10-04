// What the Orders screen (D09) decides for itself. Every money figure is the
// server's: the day's list, the collected total, an order's own totals. These
// only count rows, filter them, and word them.

import type { DayOrderItem, DayOrderResponse, DayStatus, OrderResponse, PastBillResponse, TenderResponse } from "../../../shared/api-types";
import { serviceLabels } from "../register/rail-logic";
import { methodNames } from "../tender/tender-logic";
import { voidable } from "./bills-logic";

/** The tabs over the table: every order, or those of one status. */
export type OrdersTab = "all" | DayStatus;

export const ordersTabs: readonly { id: OrdersTab; label: string }[] = [
  { id: "all", label: "All" },
  { id: "open", label: "Open" },
  { id: "paid", label: "Paid" },
  { id: "refunded", label: "Refunded" },
  { id: "voided", label: "Voided" },
];

/** How many orders each tab holds. */
export function tabCounts(orders: readonly DayOrderResponse[]): Record<OrdersTab, number> {
  const counts: Record<OrdersTab, number> = { all: orders.length, open: 0, paid: 0, refunded: 0, voided: 0 };
  for (const o of orders) counts[o.status] += 1;
  return counts;
}

/** What a row is called: its invoice number once it has one, else its order id. */
export function orderNumber(o: Pick<DayOrderResponse, "id" | "invoice_no">): string {
  return o.invoice_no || o.id;
}

/** A row's table, by the floor's name for it; a dash when it has none. */
export function tableText(tableId: number | undefined, names: ReadonlyMap<number, string>): string {
  if (!tableId) return "–";
  return names.get(tableId) ?? `Table ${tableId}`;
}

/** "2 × Masala dosa, 1 × Filter coffee"; one of a thing is just its name. */
export function itemsText(items: readonly DayOrderItem[]): string {
  if (items.length === 0) return "No items";
  return items.map((i) => (i.qty === 1 ? i.name : `${i.qty} × ${i.name}`)).join(", ");
}

/**
 * The rows a tab and a search leave. The search is over what a cashier can
 * read off a receipt or the floor: the invoice number, the order id, the
 * table's name. An order carries no guest's name to search.
 */
export function filterOrders(orders: readonly DayOrderResponse[], tab: OrdersTab, query: string, names: ReadonlyMap<number, string>): DayOrderResponse[] {
  const q = query.trim().toLowerCase();
  return orders.filter((o) => {
    if (tab !== "all" && o.status !== tab) return false;
    if (q === "") return true;
    const table = o.table_id ? tableText(o.table_id, names).toLowerCase() : "";
    return (
      // An id is random letters: two of them would match a table's name by chance.
      (q.length >= 4 && o.id.toLowerCase().includes(q)) ||
      (o.invoice_no ?? "").toLowerCase().includes(q) ||
      (table !== "" && (table === q || table.includes(q) || `table ${table}` === q))
    );
  });
}

/**
 * When an order was opened or a bill closed, in the device's zone: "10:24 am"
 * today, "4 Oct, 10:24 am" for a bill from before.
 */
export function timeText(iso: string, now: Date): string {
  const at = new Date(iso);
  const time = at.toLocaleTimeString("en-IN", { hour: "numeric", minute: "2-digit", hour12: true });
  if (at.toDateString() === now.toDateString()) return time;
  return `${at.toLocaleDateString("en-IN", { day: "numeric", month: "short" })}, ${time}`;
}

/** "1 order today", "86 orders today". */
export function countText(n: number): string {
  return `${n} ${n === 1 ? "order" : "orders"} today`;
}

/** A bill from before today, found by its number, as a row's status. */
export function pastStatus(b: Pick<PastBillResponse, "credited">): DayStatus {
  return b.credited.minor > 0 ? "refunded" : "paid";
}

/** An order's place on the list, read from the order itself, for the panel's chip. */
export function statusOf(order: Pick<OrderResponse, "status" | "lines" | "totals">): DayStatus {
  if (order.status !== "closed") return "open";
  if (order.totals.credited.minor > 0) return "refunded";
  return order.lines.some((l) => l.status !== "voided") ? "paid" : "voided";
}

/** "Table T4 · Dine-in": where and how, under the panel's title. */
export function whereText(order: Pick<OrderResponse, "table_id" | "service">, names: ReadonlyMap<number, string>): string {
  const how = serviceLabels[order.service];
  return order.table_id ? `${tableText(order.table_id, names)} · ${how}` : how;
}

/** "Cash + Card": the ways the money that stands was taken; null with none. */
export function paymentText(tenders: readonly TenderResponse[]): string | null {
  const taken = tenders.filter((t) => t.status !== "voided");
  if (taken.length === 0) return null;
  return [...new Set(taken.map((t) => methodNames[t.method]))].join(" + ");
}

/** The payments Void can act on. */
export function voidableTenders(tenders: readonly TenderResponse[]): TenderResponse[] {
  return tenders.filter(voidable);
}

// What the Orders screen (D09) decides for itself. Every money figure is the
// server's: the day's list, the collected total, an order's own totals. These
// only count rows, filter them, and word them.

import type { DayOrderItem, DayOrderResponse, DayStatus, GuestRoundProgress, OrderResponse, PastBillResponse, TenderResponse } from "../../../shared/api-types";
import { serviceLabels } from "../register/rail-logic";
import { methodNames } from "../tender/tender-logic";
import { voidable } from "./bills-logic";

/**
 * The tabs over the table: every order, those of one status, or the orders
 * to collect that are still out (ADR-253), which is what a counter calls
 * from.
 */
export type OrdersTab = "all" | "collect" | DayStatus;

export const ordersTabs: readonly { id: OrdersTab; label: string }[] = [
  { id: "all", label: "All" },
  { id: "collect", label: "To collect" },
  { id: "open", label: "Open" },
  { id: "paid", label: "Paid" },
  { id: "refunded", label: "Refunded" },
  { id: "voided", label: "Voided" },
];

/** How many orders each tab holds. */
export function tabCounts(orders: readonly DayOrderResponse[]): Record<OrdersTab, number> {
  const counts: Record<OrdersTab, number> = { all: orders.length, collect: 0, open: 0, paid: 0, refunded: 0, voided: 0 };
  for (const o of orders) {
    counts[o.status] += 1;
    if (toCollect(o)) counts.collect += 1;
  }
  return counts;
}

/**
 * Whether an order is one to collect that is still out: paid for on a
 * guest's phone, with a code, and not yet handed over. Handing it over
 * closes its ticket, and it is then a paid order like any other.
 */
export function toCollect(o: Pick<DayOrderResponse, "pickup_code" | "status">): boolean {
  return o.pickup_code !== undefined && o.pickup_code !== "" && o.status === "open";
}

/**
 * What the kitchen has done with an order to collect, for whoever calls the
 * codes. ready is the one to call. It is read off the kitchen's own tickets:
 * an order made at no station never says ready, and is handed over when it
 * is.
 */
export function collectWords(progress: GuestRoundProgress | undefined): { label: string; ready: boolean } {
  switch (progress) {
    case "sent":
      return { label: "With the kitchen", ready: false };
    case "preparing":
      return { label: "Being made", ready: false };
    case "ready":
      return { label: "Ready to call", ready: true };
    case "cancelled":
      return { label: "Cancelled", ready: false };
    default:
      return { label: "Paid", ready: false };
  }
}

/** What a row is called: its invoice number once it has one, else its order id. */
export function orderNumber(o: Pick<DayOrderResponse, "id" | "invoice_no">): string {
  return o.invoice_no || o.id;
}

/** A row's table, by the floor's name for it; an order to collect's code; a dash when it has neither. */
export function tableText(tableId: number | undefined, names: ReadonlyMap<number, string>, pickupCode?: string): string {
  if (!tableId) return pickupCode || "–";
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
 * table's name, and the pickup code on a guest's phone, whole. An order
 * carries no guest's name to search.
 *
 * To collect, what is ready to call comes first and then the longest
 * waiting: the list is read from the top by whoever is calling.
 */
export function filterOrders(orders: readonly DayOrderResponse[], tab: OrdersTab, query: string, names: ReadonlyMap<number, string>): DayOrderResponse[] {
  const q = query.trim().toLowerCase();
  const rows = orders.filter((o) => {
    if (tab === "collect" ? !toCollect(o) : tab !== "all" && o.status !== tab) return false;
    if (q === "") return true;
    const table = o.table_id ? tableText(o.table_id, names).toLowerCase() : "";
    return (
      (o.pickup_code ?? "").toLowerCase() === q ||
      // An id is random letters: two of them would match a table's name by chance.
      (q.length >= 4 && o.id.toLowerCase().includes(q)) ||
      (o.invoice_no ?? "").toLowerCase().includes(q) ||
      (table !== "" && (table === q || table.includes(q) || `table ${table}` === q))
    );
  });
  if (tab !== "collect") return rows;
  const ready = (o: DayOrderResponse) => (collectWords(o.progress).ready ? 0 : 1);
  return [...rows].sort((a, b) => ready(a) - ready(b) || a.opened_at.localeCompare(b.opened_at));
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

/** "Table T4 · Dine-in": where and how, under the panel's title. "To collect · B7K" for an order with a pickup code. */
export function whereText(order: Pick<OrderResponse, "table_id" | "service">, names: ReadonlyMap<number, string>, pickupCode?: string): string {
  if (pickupCode) return `To collect · ${pickupCode}`;
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

// What the Orders screen (D09) decides for itself. Every money figure is the
// server's: the day's list, the collected total, an order's own totals. These
// only count rows, filter them, and word them.

import type {
  DayOrderItem,
  DayOrderResponse,
  DayStatus,
  GuestRoundProgress,
  Money,
  OrderResponse,
  OrderStatus,
  PastBillResponse,
  TenderResponse,
  TotalsResponse,
} from "../../../shared/api-types";
import { venueTime } from "../../../shared/venue-clock";
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

/** The tab a link asks for, "?tab=open", or All for none or one that is not a tab. */
export function tabFrom(search: string): OrdersTab {
  const asked = new URLSearchParams(search).get("tab");
  return ordersTabs.find((t) => t.id === asked)?.id ?? "all";
}

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

/**
 * A row's number: its invoice number once it has one, else a dash. Its time
 * and its items are columns of their own, and its `ord_` id is the system's,
 * not a name staff use (TILL-8, TBS-7, ADR-393).
 */
export function orderNumber(o: Pick<DayOrderResponse, "invoice_no">): string {
  return o.invoice_no || "–";
}

/**
 * What staff call an order where it stands alone (TILL-8, TBS-7, ADR-393):
 * its invoice number once it has one; before that, when it was opened, on
 * the venue's clock, and its first dish: "12:40 PM · Masala dosa". Never its
 * `ord_` id.
 */
export function orderName(o: { invoiceNo?: string | undefined; openedAt: string; firstItem?: string | undefined }, timezone: string | null): string {
  if (o.invoiceNo) return o.invoiceNo;
  const opened = venueTime(o.openedAt, timezone);
  return o.firstItem ? `${opened} · ${o.firstItem}` : opened;
}

/** The name of an order as the till holds it: its first invoice, when it was opened, its first dish still on it. */
export function ticketName(order: Pick<OrderResponse, "invoices" | "opened_at" | "lines">, timezone: string | null): string {
  return orderName(
    { invoiceNo: order.invoices[0]?.invoice_no, openedAt: order.opened_at, firstItem: order.lines.find((l) => l.status !== "voided")?.name },
    timezone,
  );
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

/** Where the money on an order stands, for the panel's head: what is due, or what was paid. */
export interface Standing {
  readonly word: "Amount due" | "Paid";
  readonly amount: Money;
  readonly tone: "warning" | "success";
}

/**
 * What is due while anything is, else what was paid while anything stands
 * paid; null for an order with neither, which its chip says enough about. The
 * head shows it beside the status, so a paid order still open reads as paid
 * without scrolling to its totals (TBS-16).
 */
export function standing(totals: Pick<TotalsResponse, "balance_due" | "net_paid">): Standing | null {
  if (totals.balance_due.minor > 0) return { word: "Amount due", amount: totals.balance_due, tone: "warning" };
  if (totals.net_paid.minor > 0) return { word: "Paid", amount: totals.net_paid, tone: "success" };
  return null;
}

/** What the panel needs to know of an order to choose its buttons. */
export interface BillFacts {
  readonly status: OrderStatus;
  /** Money is still due on it. */
  readonly owed: boolean;
  /** It has an invoice. */
  readonly invoiced: boolean;
  /** Something on it can still be credited. */
  readonly creditable: boolean;
  /** How many of its payments Void can act on. */
  readonly voidable: number;
  /** An order to collect that has not been handed over. */
  readonly collecting: boolean;
}

/** The panel's buttons, each drawn only when it does something. More holds Invoice, History and Void. */
export interface BillActions {
  readonly takeBalance: boolean;
  readonly handOver: boolean;
  readonly openOrder: boolean;
  readonly pay: boolean;
  readonly reprint: boolean;
  readonly refund: boolean;
  readonly invoice: boolean;
  readonly voidPayment: boolean;
}

/**
 * Which of the panel's actions apply. A button that could only be drawn shut
 * is left out (TBS-16): Pay on a bill that owes nothing or is saved, Refund
 * on one that owes or has nothing left to credit, the invoice's three before
 * there is an invoice. History is always under More.
 */
export function billActions(o: BillFacts): BillActions {
  const stillOpen = o.status !== "closed" && o.status !== "reopened";
  return {
    takeBalance: o.owed && o.status === "reopened",
    handOver: o.collecting,
    openOrder: stillOpen,
    pay: stillOpen && o.owed && o.status !== "held",
    reprint: o.invoiced,
    refund: o.invoiced && o.creditable && !o.owed,
    invoice: o.invoiced,
    voidPayment: o.voidable > 0,
  };
}

// What the table's own screen works out (T03): whether a table has one order to
// show, where that order stands with the kitchen, and when the party sat down.
// The counts are read from the order's lines and kitchen tickets, as the server
// projects them (ADR-089, ADR-090).

import type { LiveTableResponse, OrderResponse } from "../../../shared/api-types";
import { clockText } from "../../../shared/venue-text";
import { readyLines } from "../register/send-logic";
import { elapsed } from "../register/ticket-time";
import { route } from "../shell/routes";

/**
 * The order the table's screen shows: the one order of a table with guests.
 * None for a free table or one to clear (the floor seats and clears), nor for
 * a table with several orders (the floor asks which).
 */
export function detailOrder(t: LiveTableResponse): string | null {
  if (t.status === "free" || t.status === "dirty") return null;
  return t.orders.length === 1 ? (t.orders[0] ?? null) : null;
}

export interface ProgressStep {
  readonly key: "placed" | "sent" | "preparing" | "ready";
  readonly label: string;
  readonly detail: string;
  /** Done, where the order is now, or not reached. */
  readonly state: "done" | "now" | "todo";
  /** When it happened, where the order says. */
  readonly at?: string;
}

const items = (n: number) => `${n} ${n === 1 ? "item" : "items"}`;

/** The order's way through the kitchen, step by step, for "Order Status". */
export function orderProgress(order: OrderResponse): ProgressStep[] {
  const lines = order.lines.filter((l) => l.status !== "voided");
  const atPass = readyLines(order);
  const sent = lines.filter((l) => l.sent);
  const toSend = lines.filter((l) => !l.sent && (l.station ?? "") !== "").length;
  const ready = sent.filter((l) => atPass.has(l.id)).length;
  const preparing = sent.length - ready;
  const firstSent = order.tickets.reduce<string | undefined>((min, t) => (min === undefined || t.at < min ? t.at : min), undefined);

  return [
    {
      key: "placed",
      label: "Order placed",
      detail: lines.length > 0 ? items(lines.length) : "Nothing ordered yet",
      state: lines.length > 0 ? "done" : "todo",
      ...(lines.length > 0 ? { at: order.opened_at } : {}),
    },
    {
      key: "sent",
      label: "Sent to kitchen",
      detail: sent.length > 0 ? items(sent.length) : toSend > 0 ? `${items(toSend)} to send` : "Nothing sent yet",
      state: sent.length > 0 ? "done" : toSend > 0 ? "now" : "todo",
      ...(sent.length > 0 && firstSent ? { at: firstSent } : {}),
    },
    {
      key: "preparing",
      label: "In preparation",
      detail: preparing > 0 ? items(preparing) : "Nothing in preparation",
      state: preparing > 0 ? "now" : ready > 0 ? "done" : "todo",
    },
    {
      key: "ready",
      label: "Ready to serve",
      detail: ready > 0 ? `${items(ready)} at the pass` : "Nothing at the pass",
      state: ready > 0 ? "done" : "todo",
    },
  ];
}

/** "7:14 PM", in the venue's zone. */
export function timeText(at: string, timezone: string | null): string {
  return clockText(new Date(at), timezone).split(" · ")[1] ?? "";
}

/** "Started 7:14 PM · 1h 12m", or null for a table with no time. */
export function startedText(since: string | undefined, now: number, timezone: string | null): string | null {
  if (!since) return null;
  return `Started ${timeText(since, timezone)} · ${elapsed(since, now, 0).text}`;
}

/**
 * What a tap on a table's card does on the waiter's home (ADR-171): a table with
 * one order opens its own screen, a free one is seated and one to clear is
 * cleared right there, and anything else (several orders at one table) is the
 * floor's, whose panel asks which.
 */
export type CardOpens = "detail" | "seat" | "clear" | "floor";

export function cardOpens(t: LiveTableResponse): CardOpens {
  if (t.status === "free") return "seat";
  if (t.status === "dirty") return "clear";
  return detailOrder(t) ? "detail" : "floor";
}

/** Where the order screen's back arrow leads a waiter: the order's table, or the tables. */
export function backFromOrder(where: { readonly table_id?: number }): { readonly href: string; readonly label: string } {
  return where.table_id ? { href: route.waiterTable(where.table_id), label: "Back to the table" } : { href: route.waiter, label: "Back to the tables" };
}

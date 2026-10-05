// What the order rail says about the order in front, without React: how it is
// served, where, and who is at it (D03).

import type { Money, OpenOrder, OrderResponse, ServiceType, TotalsResponse } from "../../../shared/api-types";
import { rateText } from "../tender/invoice-format";

/** One row of tax in the totals: "CGST 2.5%" and what it comes to. */
export interface TaxRow {
  readonly label: string;
  readonly amount: Money;
}

/**
 * The tax rows of an order's totals: each component the server worked out
 * ("CGST 2.5%", "SGST 2.5%"), or one row, "Tax", when it names none (an order
 * with nothing on it yet, a profile with a single tax). The amounts are the
 * server's.
 */
export function taxRows(order: Pick<OrderResponse, "tax" | "totals"> | undefined, currency: string): TaxRow[] {
  const components = order?.tax?.components ?? [];
  if (components.length === 0) return [{ label: "Tax", amount: order?.totals.tax ?? { minor: 0, currency } }];
  return components.map((c) => ({ label: `${c.name} ${rateText(c.rate)}`, amount: c.amount }));
}

/**
 * What the order still asks for: its total, or the balance once any of it is
 * paid. Zero in the venue's currency before there is an order.
 */
export function amountOwed(totals: Pick<TotalsResponse, "total" | "paid" | "balance_due"> | undefined, currency: string): Money {
  if (!totals) return { minor: 0, currency };
  return totals.paid.minor > 0 ? totals.balance_due : totals.total;
}

/**
 * How the order in front is served. An order says so itself; a ticket not yet
 * opened says what it will open as, which is what the server would make of it
 * (ADR-146): dine-in with a table, takeaway without.
 */
export function serviceOf(order: { readonly service: ServiceType } | undefined, opening: OpenOrder): ServiceType {
  return order?.service ?? opening.service ?? (opening.table_id ? "dine_in" : "takeaway");
}

export const serviceLabels: Readonly<Record<ServiceType, string>> = {
  dine_in: "Dine-in",
  takeaway: "Takeaway",
  delivery: "Delivery",
};

/**
 * The rail's heading: the table by its name from the floor ("Table T4", or
 * "Table T4 + T7" for a joined party), else how the order is served.
 */
export function railTitle(
  where: { readonly table_id?: number; readonly joined?: readonly number[] },
  service: ServiceType,
  names: ReadonlyMap<number, string> = new Map(),
): string {
  if (!where.table_id) return serviceLabels[service];
  const name = (id: number) => names.get(id) ?? String(id);
  return `Table ${[where.table_id, ...(where.joined ?? [])].map(name).join(" + ")}`;
}

/** "4 guests", "1 guest", or null when nobody was counted (a counter sale). */
export function guestsText(covers: number): string | null {
  if (covers <= 0) return null;
  return `${covers} ${covers === 1 ? "guest" : "guests"}`;
}

/**
 * Whether the rail asks for a table: a dine-in order not yet at one. Takeaway
 * and delivery have none to pick, and a seated order is moved from the floor.
 */
export function needsTable(order: { readonly table_id?: number } | undefined, opening: OpenOrder, service: ServiceType): boolean {
  return service === "dine_in" && !(order?.table_id ?? opening.table_id);
}

/**
 * The service can be chosen only before the order is opened: it is written on
 * the opening event and changes only by seating (ADR-146).
 */
export function serviceLocked(orderId: string | null): boolean {
  return orderId !== null;
}

/**
 * A line's course in words, for its small print (T02): the menu's name for the
 * number, "Course 5" for one the menu no longer has, nothing for a line with no
 * course.
 */
export function courseNamer(courses: readonly { readonly number: number; readonly name: string }[]): (course: number | undefined) => string | null {
  const names = new Map(courses.map((c) => [c.number, c.name]));
  return (course) => (course ? (names.get(course) ?? `Course ${course}`) : null);
}

/**
 * Whether Save parks the order (hold). A waiter's order at a table is kept at
 * that table as it stands, so Save only leaves it; holding it would shut it to
 * new items until someone resumed it. Any other order is parked, which is how
 * it is found again (ADR-168).
 */
export function saveParks(waiter: boolean, where: { readonly table_id?: number }): boolean {
  return !(waiter && Boolean(where.table_id));
}

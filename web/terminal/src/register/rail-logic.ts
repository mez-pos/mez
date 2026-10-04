// What the order rail says about the order in front, without React: how it is
// served, where, and who is at it (D03).

import type { OpenOrder, ServiceType } from "../../../shared/api-types";

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

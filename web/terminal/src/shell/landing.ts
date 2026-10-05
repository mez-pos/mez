// Where a person lands after signing in (rebuild flow map, ADR-148): on their
// own work, not on a menu of everything. A cashier on the order screen, a
// waiter on their tables, the kitchen on its board, the owner and a manager
// on the order screen with the full navigation around it.
//
// The role's name decides for the roles every venue starts with, because the
// waiter and the cashier hold the same permissions (ADR-145) and only where
// they land tells them apart. A role the owner made themselves, or renamed, is
// landed by what it can do.

import type { Permission } from "../../../shared/api-types";

/** The first screen of a role: a path of the terminal. */
export type Landing = "/pos" | "/waiter" | "/pass";

// A Map, because the key is a name the owner may choose: "constructor" must not
// find something on Object's prototype.
const byRoleName: ReadonlyMap<string, Landing> = new Map<string, Landing>([
  ["owner", "/pos"],
  ["manager", "/pos"],
  ["cashier", "/pos"],
  ["waiter", "/waiter"],
  ["kitchen", "/pass"],
]);

export function landingFor(role: string, permissions: readonly Permission[]): Landing {
  const known = byRoleName.get(role.trim().toLowerCase());
  if (known) return known;
  if (permissions.includes("order.open")) return "/pos";
  if (permissions.includes("kitchen.view")) return "/pass";
  return "/pos";
}

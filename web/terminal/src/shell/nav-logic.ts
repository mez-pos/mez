// The top navigation, as data (rebuild flow map, D05): Orders and Tables for
// everyone who works the floor, and Menu and Reports for those who may open
// them. Menu and Reports are the back office's screens: they are links out to
// it, shown only to a person whose role can do something there, so a pill never
// leads to a screen that would refuse them (ADR-148).

import type { Permission } from "../../../shared/api-types";
import { route } from "./routes";

export type NavId = "orders" | "tables" | "menu" | "reports";

export interface NavItem {
  id: NavId;
  label: string;
  href: string;
  /** A page of the back office, which is another app: a link, not a route. */
  external: boolean;
}

/** The back office's own pages that the pills open. */
export const backOffice = { menu: "/backoffice/menu", reports: "/backoffice/sales" } as const;

export function navItems(permissions: readonly Permission[]): NavItem[] {
  const items: NavItem[] = [
    { id: "orders", label: "Orders", href: route.order, external: false },
    { id: "tables", label: "Tables", href: route.tables, external: false },
  ];
  if (permissions.includes("menu.edit")) items.push({ id: "menu", label: "Menu", href: backOffice.menu, external: true });
  if (permissions.includes("reports.view")) items.push({ id: "reports", label: "Reports", href: backOffice.reports, external: true });
  return items;
}

/** The pill to show pressed for a path: Orders covers the order screen, its history and paying. */
export function activeNav(path: string): NavId | null {
  if (path === route.order || path === route.orders || path.startsWith(`${route.orders}/`) || path.startsWith("/pay/")) return "orders";
  if (path === route.tables) return "tables";
  return null;
}

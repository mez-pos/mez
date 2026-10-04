import { describe, expect, it } from "vitest";
import { activeNav, navItems } from "./nav-logic";

describe("navItems", () => {
  it("gives a floor role Orders and Tables only", () => {
    expect(navItems(["order.open", "order.edit", "payment.take"]).map((i) => i.id)).toEqual(["orders", "tables"]);
  });

  it("adds Menu and Reports for those who can open them, as links out to the back office", () => {
    const owner = navItems(["order.open", "menu.edit", "reports.view"]);
    expect(owner.map((i) => i.id)).toEqual(["orders", "tables", "menu", "reports"]);
    expect(owner.filter((i) => i.external).map((i) => i.href)).toEqual(["/backoffice/menu", "/backoffice/sales"]);
    expect(owner.filter((i) => !i.external).map((i) => i.href)).toEqual(["/pos", "/tables"]);
  });

  it("follows the back office's own rule: a manager, who cannot edit the menu, sees Reports without Menu", () => {
    expect(navItems(["order.open", "reports.view", "staff.manage"]).map((i) => i.id)).toEqual(["orders", "tables", "reports"]);
  });
});

describe("activeNav", () => {
  it("keeps Orders pressed through the order screen, its history and paying", () => {
    for (const path of ["/pos", "/orders", "/orders/ord_ABC", "/pay/ord_ABC"]) expect(activeNav(path)).toBe("orders");
  });
  it("presses Tables on the floor", () => {
    expect(activeNav("/tables")).toBe("tables");
  });
  it("presses nothing elsewhere, and does not mistake a longer word for a route", () => {
    for (const path of ["/", "/shift", "/shift/close", "/order", "/orderly", "/tablesaw", "/posx"]) expect(activeNav(path)).toBeNull();
  });
});

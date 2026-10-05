import { describe, expect, it } from "vitest";
import { activeNav, activeWaiterNav, navItems, waiterNav } from "./nav-logic";

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

describe("the waiter's navigation", () => {
  it("is Home, Orders and Menu, each a screen of the terminal", () => {
    expect(waiterNav.map((i) => [i.id, i.href])).toEqual([
      ["home", "/waiter"],
      ["orders", "/orders"],
      ["menu", "/pos"],
    ]);
  });
  it("presses Home on the waiter's tables only", () => {
    expect(activeWaiterNav("/waiter")).toBe("home");
    expect(activeWaiterNav("/waiters")).toBeNull();
  });
  it("presses Orders on the day's orders, and Menu while an order is taken or paid", () => {
    for (const path of ["/orders", "/orders/ord_ABC"]) expect(activeWaiterNav(path)).toBe("orders");
    for (const path of ["/pos", "/pay/ord_ABC"]) expect(activeWaiterNav(path)).toBe("menu");
  });
  it("presses More on the floor plan, which More opens", () => {
    expect(activeWaiterNav("/tables")).toBe("more");
  });
  it("presses nothing elsewhere", () => {
    for (const path of ["/", "/shift", "/posx", "/tablesaw"]) expect(activeWaiterNav(path)).toBeNull();
  });
});

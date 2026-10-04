import { describe, expect, it } from "vitest";
import { landingFor } from "./landing";

describe("landingFor", () => {
  it("lands each role every venue starts with on its own work", () => {
    expect(landingFor("owner", ["order.open"])).toBe("/pos");
    expect(landingFor("manager", ["order.open"])).toBe("/pos");
    expect(landingFor("cashier", ["order.open", "payment.take"])).toBe("/pos");
    expect(landingFor("waiter", ["order.open", "order.edit", "payment.take", "kitchen.view"])).toBe("/waiter");
    expect(landingFor("kitchen", ["kitchen.view"])).toBe("/kitchen");
  });

  it("tells the waiter from the cashier by the role, since their permissions are the same", () => {
    const same = ["order.open", "order.edit", "payment.take", "kitchen.view"] as const;
    expect(landingFor("waiter", same)).not.toBe(landingFor("cashier", same));
  });

  it("reads the role's name without regard to case or spaces", () => {
    expect(landingFor("  Waiter ", [])).toBe("/waiter");
    expect(landingFor("KITCHEN", [])).toBe("/kitchen");
  });

  it("lands a role the owner made by what it can do", () => {
    expect(landingFor("Bar lead", ["order.open", "line.void"])).toBe("/pos");
    expect(landingFor("Head chef", ["kitchen.view"])).toBe("/kitchen");
    expect(landingFor("Observer", ["reports.view"])).toBe("/pos");
    expect(landingFor("Nobody", [])).toBe("/pos");
  });

  it("is not fooled by a role named after something on Object's prototype", () => {
    for (const role of ["constructor", "toString", "valueOf", "__proto__", "hasOwnProperty"]) {
      expect(landingFor(role, ["order.open"])).toBe("/pos");
    }
  });

  it("prefers the order screen to the kitchen for a role that can do both", () => {
    expect(landingFor("Runner", ["kitchen.view", "order.open"])).toBe("/pos");
  });
});

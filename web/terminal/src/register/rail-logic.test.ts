import { describe, expect, it } from "vitest";
import type { OpenOrder } from "../../../shared/api-types";
import { guestsText, needsTable, railTitle, serviceLocked, serviceOf } from "./rail-logic";

const opening = (extra: Partial<OpenOrder> = {}): OpenOrder => ({ channel: "counter", covers: 0, ...extra });

describe("serviceOf", () => {
  it("reads it off an order", () => {
    expect(serviceOf({ service: "delivery" }, opening())).toBe("delivery");
  });
  it("reads what an unopened ticket will open as", () => {
    expect(serviceOf(undefined, opening({ service: "dine_in" }))).toBe("dine_in");
    expect(serviceOf(undefined, opening({ table_id: 4 }))).toBe("dine_in");
    expect(serviceOf(undefined, opening())).toBe("takeaway");
  });
  it("lets the order win over what the ticket was opened with", () => {
    expect(serviceOf({ service: "takeaway" }, opening({ service: "dine_in" }))).toBe("takeaway");
  });
});

describe("railTitle", () => {
  const names = new Map([
    [4, "T4"],
    [7, "T7"],
  ]);
  it("names the table from the floor", () => {
    expect(railTitle({ table_id: 4 }, "dine_in", names)).toBe("Table T4");
  });
  it("names every table of a joined party", () => {
    expect(railTitle({ table_id: 4, joined: [7] }, "dine_in", names)).toBe("Table T4 + T7");
  });
  it("falls back to the number before the floor has been read", () => {
    expect(railTitle({ table_id: 9 }, "dine_in")).toBe("Table 9");
  });
  it("says how it is served with no table", () => {
    expect(railTitle({}, "dine_in")).toBe("Dine-in");
    expect(railTitle({}, "takeaway")).toBe("Takeaway");
    expect(railTitle({}, "delivery")).toBe("Delivery");
  });
});

describe("guestsText", () => {
  it("counts, singular and plural, and says nothing for nobody", () => {
    expect(guestsText(0)).toBeNull();
    expect(guestsText(1)).toBe("1 guest");
    expect(guestsText(4)).toBe("4 guests");
  });
});

describe("needsTable", () => {
  it("is a dine-in order with no table", () => {
    expect(needsTable(undefined, opening({ service: "dine_in" }), "dine_in")).toBe(true);
    expect(needsTable({}, opening(), "dine_in")).toBe(true);
  });
  it("is not once seated, or for takeaway and delivery", () => {
    expect(needsTable({ table_id: 3 }, opening(), "dine_in")).toBe(false);
    expect(needsTable(undefined, opening({ table_id: 3 }), "dine_in")).toBe(false);
    expect(needsTable(undefined, opening(), "takeaway")).toBe(false);
    expect(needsTable(undefined, opening(), "delivery")).toBe(false);
  });
});

describe("serviceLocked", () => {
  it("locks once the order exists", () => {
    expect(serviceLocked(null)).toBe(false);
    expect(serviceLocked("ord_1")).toBe(true);
  });
});

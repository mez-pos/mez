import { describe, expect, it } from "vitest";
import type { OpenOrder, OrderResponse } from "../../../shared/api-types";
import { amountOwed, courseNamer, foldedTax, guestsText, needsTable, railTitle, saveParks, serviceLocked, serviceOf, taxRows } from "./rail-logic";

const opening = (extra: Partial<OpenOrder> = {}): OpenOrder => ({ channel: "counter", covers: 0, ...extra });

describe("taxRows", () => {
  const inr = (minor: number) => ({ minor, currency: "INR" });
  const totals = { tax: inr(1572) } as OrderResponse["totals"];
  const component = (name: string, rate: number, minor: number) => ({ name, rate, taxable: inr(31428), amount: inr(minor) });
  const gst = (pricing: "inclusive" | "exclusive", components = [component("CGST", 2500, 786), component("SGST", 2500, 786)]) =>
    ({ profile: "gst_in", pricing, components, taxable: inr(31428), tax: inr(1572), total: inr(33000) }) as NonNullable<OrderResponse["tax"]>;
  it("names each tax the server worked out, with its rate", () => {
    expect(taxRows({ tax: gst("exclusive"), totals }, "INR")).toEqual([
      { label: "CGST 2.5%", amount: inr(786) },
      { label: "SGST 2.5%", amount: inr(786) },
    ]);
  });
  it("says a tax is in the prices already, where it is", () => {
    expect(taxRows({ tax: gst("inclusive"), totals }, "INR")).toEqual([
      { label: "incl. CGST 2.5%", amount: inr(786) },
      { label: "incl. SGST 2.5%", amount: inr(786) },
    ]);
    expect(taxRows({ tax: gst("inclusive", []), totals }, "INR")).toEqual([{ label: "incl. Tax", amount: inr(1572) }]);
  });
  it("is one row when the server names no components", () => {
    expect(taxRows({ totals }, "INR")).toEqual([{ label: "Tax", amount: inr(1572) }]);
    const none = { components: [] } as unknown as NonNullable<OrderResponse["tax"]>;
    expect(taxRows({ tax: none, totals }, "INR")).toEqual([{ label: "Tax", amount: inr(1572) }]);
  });
  it("is zero in the venue's currency before there is an order", () => {
    expect(taxRows(undefined, "EUR")).toEqual([{ label: "Tax", amount: { minor: 0, currency: "EUR" } }]);
  });
});

describe("foldedTax", () => {
  const inr = (minor: number) => ({ minor, currency: "INR" });
  const totals = { tax: inr(1572) } as OrderResponse["totals"];
  const component = (name: string, minor: number) => ({ name, rate: 2500, taxable: inr(31428), amount: inr(minor) });
  const tax = (pricing: "inclusive" | "exclusive", components: ReturnType<typeof component>[]) =>
    ({ profile: "gst_in", pricing, components, taxable: inr(31428), tax: inr(1572), total: inr(33000) }) as NonNullable<OrderResponse["tax"]>;
  const both = [component("CGST", 786), component("SGST", 786)];

  const cases: [string, Pick<OrderResponse, "tax" | "totals"> | undefined, { label: string; amount: { minor: number; currency: string } } | null][] = [
    ["inclusive GST folds into one row with the order's tax", { tax: tax("inclusive", both), totals }, { label: "incl. GST", amount: inr(1572) }],
    ["exclusive GST says no incl.", { tax: tax("exclusive", both), totals }, { label: "GST", amount: inr(1572) }],
    [
      "components that are not GST fold as Tax",
      { tax: tax("exclusive", [component("VAT", 786), component("Cess", 786)]), totals },
      { label: "Tax", amount: inr(1572) },
    ],
    ["one component has nothing to fold", { tax: tax("inclusive", [component("IGST", 1572)]), totals }, null],
    ["no components has nothing to fold", { totals }, null],
    ["no order has nothing to fold", undefined, null],
  ];
  it.each(cases)("%s", (_name, order, want) => {
    expect(foldedTax(order, "INR")).toEqual(want);
  });
});

describe("amountOwed", () => {
  const inr = (minor: number) => ({ minor, currency: "INR" });
  it("is the total until something is paid, then the balance", () => {
    expect(amountOwed({ total: inr(93000), paid: inr(0), balance_due: inr(93000) }, "INR")).toEqual(inr(93000));
    expect(amountOwed({ total: inr(93000), paid: inr(50000), balance_due: inr(43000) }, "INR")).toEqual(inr(43000));
  });
  it("is zero in the venue's currency before there is an order", () => {
    expect(amountOwed(undefined, "EUR")).toEqual({ minor: 0, currency: "EUR" });
  });
});

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

describe("courseNamer", () => {
  const name = courseNamer([
    { number: 1, name: "Starter" },
    { number: 2, name: "Main Course" },
  ]);
  it("names a line's course by its number", () => {
    expect(name(1)).toBe("Starter");
    expect(name(2)).toBe("Main Course");
  });
  it("says a course the menu no longer has by its number", () => {
    expect(name(5)).toBe("Course 5");
  });
  it("says nothing for a line with no course", () => {
    expect(name(undefined)).toBeNull();
    expect(name(0)).toBeNull();
  });
});

describe("saveParks", () => {
  it("parks a cashier's order, table or not, as Save Order always has", () => {
    expect(saveParks(false, { table_id: 4 })).toBe(true);
    expect(saveParks(false, {})).toBe(true);
  });
  it("leaves a waiter's table order open: it is kept at its table already", () => {
    expect(saveParks(true, { table_id: 4 })).toBe(false);
  });
  it("parks a waiter's order with no table, so it is found again under Orders", () => {
    expect(saveParks(true, {})).toBe(true);
  });
});

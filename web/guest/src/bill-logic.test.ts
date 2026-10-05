import { describe, expect, it } from "vitest";
import type { GuestBillResponse } from "../../shared/api-types";
import { billState, itemsLabel, payLine, summaryRows } from "./bill-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const bill = (change: Partial<GuestBillResponse> = {}): GuestBillResponse => ({
  open: true,
  lines: [
    { name: "Flat white", qty: 2, choices: ["Oat"], amount: inr(44000) },
    { name: "Biscotti", qty: 1, choices: [], amount: inr(6000) },
  ],
  items: inr(50000),
  discounts: inr(0),
  taxes: [
    { name: "CGST", rate: 2500, amount: inr(1250) },
    { name: "SGST", rate: 2500, amount: inr(1250) },
  ],
  tax_included: false,
  total: inr(52500),
  paid: inr(0),
  balance_due: inr(52500),
  ...change,
});

describe("itemsLabel", () => {
  it("counts each dish of each line", () => {
    expect(itemsLabel(bill())).toBe("Items (3)");
    expect(itemsLabel({ lines: [] })).toBe("Items (0)");
  });
});

describe("summaryRows", () => {
  it("is the items and each tax by name and rate", () => {
    expect(summaryRows(bill())).toEqual([
      { label: "Items (3)", amount: inr(50000) },
      { label: "CGST (2.5%)", amount: inr(1250) },
      { label: "SGST (2.5%)", amount: inr(1250) },
    ]);
  });
  it("shows a discount only when there is one, as money off", () => {
    expect(summaryRows(bill({ discounts: inr(5000) }))[1]).toEqual({ label: "Discount", amount: inr(5000), minus: true });
    expect(summaryRows(bill()).some((r) => r.label === "Discount")).toBe(false);
  });
  it("says tax in the prices is included, so the rows are not added to the total", () => {
    const rows = summaryRows(bill({ tax_included: true, total: inr(50000) }));
    expect(rows[1]).toEqual({ label: "Includes CGST (2.5%)", amount: inr(1250), within: true });
    expect(rows[2]?.within).toBe(true);
  });
  it("has no tax rows for an untaxed venue", () => {
    expect(summaryRows(bill({ taxes: [] }))).toEqual([{ label: "Items (3)", amount: inr(50000) }]);
  });
});

describe("billState and payLine", () => {
  const cases: [string, Partial<GuestBillResponse>, ReturnType<typeof billState>][] = [
    ["no ticket", { open: false, total: inr(0), balance_due: inr(0) }, "none"],
    ["all of it owed", {}, "due"],
    ["some of it paid", { paid: inr(20000), balance_due: inr(32500) }, "part_paid"],
    ["settled", { paid: inr(52500), balance_due: inr(0) }, "paid"],
    ["a ticket with nothing on it yet", { lines: [], items: inr(0), taxes: [], total: inr(0), balance_due: inr(0) }, "due"],
  ];
  it.each(cases)("%s", (_name, change, want) => {
    expect(billState(bill(change))).toBe(want);
  });
  it("sends a guest to staff or the counter while anything is owed, and promises no payment the phone cannot take", () => {
    expect(payLine("due")).toBe("To pay, ask a member of staff or pay at the counter.");
    expect(payLine("part_paid")).toBe(payLine("due"));
    expect(payLine("paid")).toContain("paid");
    expect(payLine("none")).toContain("once it is accepted");
  });
});

import { describe, expect, it } from "vitest";
import type { DayOrderResponse, LineResponse, OrderResponse, TenderResponse } from "../../../shared/api-types";
import {
  countText,
  filterOrders,
  itemsText,
  orderNumber,
  pastStatus,
  paymentText,
  statusOf,
  tabCounts,
  tableText,
  timeText,
  voidableTenders,
  whereText,
} from "./orders-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });
const names = new Map([[4, "T4"]]);

const row = (extra: Partial<DayOrderResponse>): DayOrderResponse =>
  ({ id: "ord_AAAA", status: "paid", service: "dine_in", covers: 2, items: [], ...extra }) as DayOrderResponse;

const day = [
  row({ id: "ord_OPEN", status: "open", table_id: 4 }),
  row({ id: "ord_PAID", status: "paid", invoice_no: "INV/26-27/000041" }),
  row({ id: "ord_BACK", status: "refunded", invoice_no: "INV/26-27/000040", table_id: 9 }),
];

describe("tabCounts", () => {
  it("counts every tab, the empty ones too", () => {
    expect(tabCounts(day)).toEqual({ all: 3, open: 1, paid: 1, refunded: 1, voided: 0 });
  });
});

describe("orderNumber", () => {
  it("is the invoice number once there is one", () => {
    expect(orderNumber(day[1]!)).toBe("INV/26-27/000041");
    expect(orderNumber(day[0]!)).toBe("ord_OPEN");
  });
});

describe("tableText", () => {
  const cases: [string, number | undefined, string][] = [
    ["the floor's name", 4, "T4"],
    ["a table the floor no longer has", 9, "Table 9"],
    ["no table", undefined, "–"],
    ["the server's zero", 0, "–"],
  ];
  it.each(cases)("%s", (_n, id, want) => {
    expect(tableText(id, names)).toBe(want);
  });
});

describe("itemsText", () => {
  it("lists what was rung up", () => {
    expect(
      itemsText([
        { qty: 2, name: "Masala dosa" },
        { qty: 1, name: "Filter coffee" },
      ]),
    ).toBe("2 × Masala dosa, Filter coffee");
  });
  it("says so when there is nothing", () => {
    expect(itemsText([])).toBe("No items");
  });
});

describe("filterOrders", () => {
  const ids = (tab: Parameters<typeof filterOrders>[1], q: string) => filterOrders(day, tab, q, names).map((o) => o.id);
  const cases: [string, Parameters<typeof filterOrders>[1], string, string[]][] = [
    ["everything", "all", "", ["ord_OPEN", "ord_PAID", "ord_BACK"]],
    ["a tab", "refunded", "", ["ord_BACK"]],
    ["an empty tab", "voided", "", []],
    ["an invoice's digits", "all", "000041", ["ord_PAID"]],
    ["an order id, in any case", "all", "ORD_open", ["ord_OPEN"]],
    ["a table's name", "all", "t4", ["ord_OPEN"]],
    ["letters an id happens to hold are not a table", "all", "pa", []],
    ["a table as it is said", "all", "Table 9", ["ord_BACK"]],
    ["a search inside a tab", "paid", "t4", []],
    ["spaces around it", "all", "  000040 ", ["ord_BACK"]],
    ["nothing like it", "all", "zzz", []],
  ];
  it.each(cases)("%s", (_n, tab, q, want) => {
    expect(ids(tab, q)).toEqual(want);
  });
});

describe("timeText", () => {
  // Built in the machine's own zone, so the test reads the same anywhere.
  const now = new Date(2026, 9, 5, 13, 0);
  it("is the time alone today", () => {
    expect(timeText(new Date(2026, 9, 5, 10, 24).toISOString(), now).toLowerCase()).toBe("10:24 am");
  });
  it("names the day for a bill from before", () => {
    expect(timeText(new Date(2026, 9, 4, 21, 5).toISOString(), now).toLowerCase()).toBe("4 oct, 9:05 pm");
  });
});

describe("countText", () => {
  it("counts in words", () => {
    expect(countText(0)).toBe("0 orders today");
    expect(countText(1)).toBe("1 order today");
    expect(countText(86)).toBe("86 orders today");
  });
});

describe("pastStatus", () => {
  it("is refunded once anything was credited", () => {
    expect(pastStatus({ credited: inr(0) })).toBe("paid");
    expect(pastStatus({ credited: inr(500) })).toBe("refunded");
  });
});

const order = (extra: Partial<OrderResponse>, credited = 0, lines: Partial<LineResponse>[] = [{ status: "active" }]): OrderResponse =>
  ({ status: "closed", service: "dine_in", lines, totals: { credited: inr(credited) }, ...extra }) as OrderResponse;

describe("statusOf", () => {
  const cases: [string, OrderResponse, string][] = [
    ["open", order({ status: "open" }), "open"],
    ["reopened by a voided payment", order({ status: "reopened" }), "open"],
    ["closed and kept", order({}), "paid"],
    ["closed, part credited", order({}, 100), "refunded"],
    ["closed, every line voided", order({}, 0, [{ status: "voided" }]), "voided"],
    ["closed, given away", order({}, 0, [{ status: "comped" }]), "paid"],
  ];
  it.each(cases)("%s", (_n, o, want) => {
    expect(statusOf(o)).toBe(want);
  });
});

describe("whereText", () => {
  it("is the table and how it is served, or how alone", () => {
    expect(whereText({ table_id: 4, service: "dine_in" }, names)).toBe("T4 · Dine-in");
    expect(whereText({ service: "takeaway" }, names)).toBe("Takeaway");
  });
});

const tender = (extra: Partial<TenderResponse>): TenderResponse =>
  ({ id: 1, method: "cash", amount: inr(1000), status: "captured", refunded: inr(0), ...extra }) as TenderResponse;

describe("paymentText", () => {
  it("names each way once, leaving out a voided payment", () => {
    expect(paymentText([tender({}), tender({ method: "card" }), tender({}), tender({ method: "upi", status: "voided" })])).toBe("Cash + Card");
    expect(paymentText([tender({ status: "voided" })])).toBeNull();
    expect(paymentText([])).toBeNull();
  });
});

describe("voidableTenders", () => {
  it("keeps the payments taken with nothing refunded", () => {
    const got = voidableTenders([tender({ id: 1 }), tender({ id: 2, status: "voided" }), tender({ id: 3, refunded: inr(100) })]);
    expect(got.map((t) => t.id)).toEqual([1]);
  });
});

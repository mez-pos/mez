import { describe, expect, it } from "vitest";
import type { DayOrderResponse, LineResponse, OrderResponse, TenderResponse } from "../../../shared/api-types";
import {
  billActions,
  collectWords,
  countText,
  filterOrders,
  itemsText,
  orderNumber,
  pastStatus,
  paymentText,
  standing,
  statusOf,
  tabCounts,
  tableText,
  timeText,
  toCollect,
  voidableTenders,
  whereText,
  type BillActions,
  type BillFacts,
  tabFrom,
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
    expect(tabCounts(day)).toEqual({ all: 3, collect: 0, open: 1, paid: 1, refunded: 1, voided: 0 });
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

describe("orders to collect", () => {
  const at = (minute: number) => `2026-10-06T10:${String(minute).padStart(2, "0")}:00Z`;
  const out = [
    row({ id: "ord_NEW1", status: "open", service: "takeaway", pickup_code: "M4Q", progress: "sent", opened_at: at(30) }),
    row({ id: "ord_RDY1", status: "open", service: "takeaway", pickup_code: "B7K", progress: "ready", opened_at: at(20) }),
    row({ id: "ord_OLD1", status: "open", service: "takeaway", pickup_code: "X9Z", progress: "preparing", opened_at: at(10) }),
    row({ id: "ord_RDY2", status: "open", service: "takeaway", pickup_code: "T2W", progress: "ready", opened_at: at(25) }),
    row({ id: "ord_GONE", status: "paid", service: "takeaway", pickup_code: "H5N", progress: "ready", opened_at: at(5) }),
    row({ id: "ord_TBL4", status: "open", table_id: 4, opened_at: at(1) }),
  ];

  it("are the ones with a code that have not been handed over", () => {
    expect(out.filter(toCollect).map((o) => o.pickup_code)).toEqual(["M4Q", "B7K", "X9Z", "T2W"]);
    expect(tabCounts(out).collect).toBe(4);
    // Each is still counted under its own status as well.
    expect(tabCounts(out).open).toBe(5);
    expect(tabCounts(out).paid).toBe(1);
  });

  it("lists what is ready to call first, then the longest waiting", () => {
    expect(filterOrders(out, "collect", "", names).map((o) => o.pickup_code)).toEqual(["B7K", "T2W", "X9Z", "M4Q"]);
    // Every other tab keeps the server's order.
    expect(filterOrders(out, "open", "", names).map((o) => o.id)).toEqual(["ord_NEW1", "ord_RDY1", "ord_OLD1", "ord_RDY2", "ord_TBL4"]);
  });

  it("is found by its code, as it is read off a guest's phone", () => {
    expect(filterOrders(out, "all", "b7k", names).map((o) => o.id)).toEqual(["ord_RDY1"]);
    expect(filterOrders(out, "all", " B7K ", names).map((o) => o.id)).toEqual(["ord_RDY1"]);
    // One handed over is still found by what it was called.
    expect(filterOrders(out, "all", "H5N", names).map((o) => o.id)).toEqual(["ord_GONE"]);
    // Part of a code finds nothing: three characters match too much by chance.
    expect(filterOrders(out, "all", "B7", names)).toEqual([]);
  });

  it("shows the code where a table's name would be", () => {
    expect(tableText(undefined, names, "B7K")).toBe("B7K");
    expect(tableText(4, names)).toBe("T4");
    expect(tableText(undefined, names)).toBe("–");
  });

  it("says what the kitchen has done, in words for whoever is calling", () => {
    expect(collectWords("sent")).toEqual({ label: "With the kitchen", ready: false });
    expect(collectWords("preparing")).toEqual({ label: "Being made", ready: false });
    expect(collectWords("ready")).toEqual({ label: "Ready to call", ready: true });
    // Made at no station, or nothing known: it is paid, and that is all that is said.
    expect(collectWords("accepted")).toEqual({ label: "Paid", ready: false });
    expect(collectWords(undefined)).toEqual({ label: "Paid", ready: false });
    expect(collectWords("cancelled")).toEqual({ label: "Cancelled", ready: false });
  });

  it("says where a collected order was, by its code", () => {
    expect(whereText({ service: "takeaway" }, names, "B7K")).toBe("To collect · B7K");
    expect(whereText({ service: "takeaway" }, names)).not.toContain("collect");
  });
});

describe("standing", () => {
  const cases: [string, number, number, ReturnType<typeof standing>][] = [
    ["money due wins over what was paid", 50000, 36000, { word: "Amount due", amount: inr(50000), tone: "warning" }],
    ["paid in full, the order still open", 0, 86000, { word: "Paid", amount: inr(86000), tone: "success" }],
    ["nothing due and nothing paid", 0, 0, null],
  ];
  it.each(cases)("%s", (_n, due, paid, want) => {
    expect(standing({ balance_due: inr(due), net_paid: inr(paid) })).toEqual(want);
  });
});

describe("billActions", () => {
  const facts = (extra: Partial<BillFacts>): BillFacts => ({
    status: "open",
    owed: false,
    invoiced: false,
    creditable: false,
    voidable: 0,
    collecting: false,
    ...extra,
  });
  const drawn = (a: BillActions) =>
    Object.entries(a)
      .filter(([, on]) => on)
      .map(([name]) => name);

  const cases: [string, BillFacts, string[]][] = [
    ["an open order that owes", facts({ owed: true }), ["openOrder", "pay"]],
    ["a saved order that owes: Open order brings it back, Pay is not drawn", facts({ status: "held", owed: true }), ["openOrder"]],
    [
      "paid and still open: no Pay, the invoice's buttons and Refund",
      facts({ status: "settling", invoiced: true, creditable: true, voidable: 1 }),
      ["openOrder", "reprint", "refund", "invoice", "voidPayment"],
    ],
    ["a closed bill", facts({ status: "closed", invoiced: true, creditable: true, voidable: 2 }), ["reprint", "refund", "invoice", "voidPayment"]],
    ["a closed bill credited in full", facts({ status: "closed", invoiced: true }), ["reprint", "invoice"]],
    ["a bill closed at nothing has no invoice", facts({ status: "closed" }), []],
    ["a reopened bill that owes", facts({ status: "reopened", owed: true, invoiced: true, creditable: true }), ["takeBalance", "reprint", "invoice"]],
    ["an order to collect still out", facts({ collecting: true, invoiced: true, creditable: true }), ["handOver", "openOrder", "reprint", "refund", "invoice"]],
  ];
  it.each(cases)("%s", (_n, f, want) => {
    expect(drawn(billActions(f))).toEqual(want);
  });
});

describe("tabFrom", () => {
  it("opens on the tab a link names, as close of day's link to the open orders does (TBS-2)", () => {
    expect(tabFrom("?tab=open")).toBe("open");
    expect(tabFrom("tab=collect")).toBe("collect");
  });
  it("opens on All for no tab, or one that is not a tab", () => {
    expect(tabFrom("")).toBe("all");
    expect(tabFrom("?tab=closed")).toBe("all");
    expect(tabFrom("?q=42")).toBe("all");
  });
});

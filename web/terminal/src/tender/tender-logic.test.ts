import { describe, expect, it } from "vitest";
import type { OrderResponse, TenderPayment } from "../../../shared/api-types";
import { attemptKey, chargeBlock, chargeLabel, planTender, quickTenders } from "./tender-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

describe("quickTenders", () => {
  const cases: [string, number, string, number[]][] = [
    ["a coffee: exact, then the notes above it", 18000, "INR", [18000, 19000, 20000, 50000]],
    ["an odd total rounds up to each step", 45250, "INR", [45250, 46000, 50000]],
    ["a round total has nothing between it and the next step", 50000, "INR", [50000, 51000, 55000, 60000, 100000]],
    ["paise below a rupee", 5, "INR", [5, 1000, 5000, 10000, 50000]],
    ["a zero-decimal currency uses whole units", 1234, "JPY", [1234, 1240, 1250, 1300, 1500]],
    ["nothing owed, nothing offered", 0, "INR", []],
  ];
  it.each(cases)("%s", (_name, due, currency, want) => {
    expect(quickTenders(due, currency)).toEqual(want);
  });
});

describe("planTender", () => {
  const cases: [string, Parameters<typeof planTender>[0], ReturnType<typeof planTender>][] = [
    ["cash, exact", { method: "cash", entered: 45000, tip: 0, due: 45000 }, { amount: 45000, change: 0, charge: 45000, problem: null }],
    ["cash, a ₹500 note for ₹450", { method: "cash", entered: 50000, tip: 0, due: 45000 }, { amount: 45000, change: 5000, charge: 45000, problem: null }],
    [
      "cash, a tip comes out of the change, never into the amount",
      { method: "cash", entered: 50000, tip: 2000, due: 45000 },
      { amount: 45000, change: 3000, charge: 45000, problem: null },
    ],
    [
      "cash, less than the balance is a part payment",
      { method: "cash", entered: 20000, tip: 0, due: 45000 },
      { amount: 20000, change: 0, charge: 20000, problem: null },
    ],
    [
      "cash, the tip eats into a part payment",
      { method: "cash", entered: 20000, tip: 5000, due: 45000 },
      { amount: 15000, change: 0, charge: 15000, problem: null },
    ],
    [
      "cash, a tip larger than the cash",
      { method: "cash", entered: 1000, tip: 2000, due: 45000 },
      { amount: 0, change: 0, charge: 0, problem: "The tip is more than the cash handed over." },
    ],
    [
      "cash, the whole of it tip",
      { method: "cash", entered: 2000, tip: 2000, due: 45000 },
      { amount: 0, change: 0, charge: 0, problem: "Enter the cash handed over." },
    ],
    ["cash, nothing entered", { method: "cash", entered: 0, tip: 0, due: 45000 }, { amount: 0, change: 0, charge: 0, problem: "Enter the cash handed over." }],
    [
      "card, the balance with a tip: the card is charged both, the amount is the bill",
      { method: "card", entered: 45000, tip: 4500, due: 45000 },
      { amount: 45000, change: 0, charge: 49500, problem: null },
    ],
    ["UPI, part of the balance", { method: "upi", entered: 15000, tip: 0, due: 45000 }, { amount: 15000, change: 0, charge: 15000, problem: null }],
    [
      "card, more than the balance (it moved on another till)",
      { method: "card", entered: 45000, tip: 0, due: 30000 },
      { amount: 30000, change: 0, charge: 30000, problem: null },
    ],
    ["card, nothing entered", { method: "card", entered: 0, tip: 0, due: 45000 }, { amount: 0, change: 0, charge: 0, problem: "Enter the amount." }],
    [
      "a share in cash: more than the share, change back",
      { method: "cash", entered: 50000, tip: 0, due: 35100, whole: true },
      { amount: 35100, change: 14900, charge: 35100, problem: null },
    ],
    [
      "a share in cash: less than the share is refused, not a part payment",
      { method: "cash", entered: 20000, tip: 0, due: 35100, whole: true },
      { amount: 0, change: 0, charge: 0, problem: "A share is paid in one go: take at least the share." },
    ],
    [
      "a share by card takes exactly the share, whatever was keyed",
      { method: "card", entered: 100, tip: 500, due: 35100, whole: true },
      { amount: 35100, change: 0, charge: 35600, problem: null },
    ],
    ["nothing owed", { method: "cash", entered: 50000, tip: 0, due: 0 }, { amount: 0, change: 0, charge: 0, problem: "Nothing is owed on this ticket." }],
  ];
  it.each(cases)("%s", (_name, input, want) => {
    expect(planTender(input)).toEqual(want);
  });
});

describe("attemptKey", () => {
  const body = (minor: number): TenderPayment => ({ method: "cash", amount: inr(minor), tip: inr(0) });
  let n = 0;
  const fresh = () => `k${++n}`;

  it("a first attempt gets a new key", () => {
    expect(attemptKey(null, body(100), () => "a")).toEqual({ key: "a", body: body(100) });
  });
  it("a retry of the same tender reuses the key", () => {
    const last = { key: "a", body: body(100) };
    expect(attemptKey(last, body(100), fresh).key).toBe("a");
  });
  it("a changed tender is a new one", () => {
    const last = { key: "a", body: body(100) };
    expect(attemptKey(last, body(200), () => "b").key).toBe("b");
  });
  it("a changed reference is a new one", () => {
    const last = { key: "a", body: { ...body(100), method: "card" as const, reference: "1" } };
    expect(attemptKey(last, { ...body(100), method: "card", reference: "2" }, () => "b").key).toBe("b");
  });
});

describe("chargeBlock", () => {
  const order = (extra: Partial<OrderResponse>): OrderResponse =>
    ({
      status: "open",
      lines: [{ status: "active" }],
      totals: { balance_due: inr(45000) },
      ...extra,
    }) as OrderResponse;

  const cases: [string, OrderResponse | undefined, number, string | null][] = [
    ["an open ticket with a balance", order({}), 0, null],
    ["part paid", order({ status: "settling" }), 0, null],
    ["no order yet", undefined, 0, "Nothing to pay yet."],
    ["only voided lines", order({ lines: [{ status: "voided" }] as OrderResponse["lines"] }), 0, "Nothing to pay yet."],
    ["lines still adding", order({}), 1, "Wait for every item to be added."],
    ["held", order({ status: "held" }), 0, "Resume this order to pay it."],
    ["paid in full, still open: Pay finishes it", order({ totals: { balance_due: inr(0) } as OrderResponse["totals"] }), 0, null],
    ["owing the guest", order({ totals: { balance_due: inr(-100) } as OrderResponse["totals"] }), 0, "The guest is owed money back. Refunds come with Task 6."],
    ["closed", order({ status: "closed" }), 0, "This order is closed."],
  ];
  it.each(cases)("%s", (_name, o, pending, want) => {
    expect(chargeBlock(o, pending)).toBe(want);
  });
});

describe("chargeLabel", () => {
  const order = (balance: number, invoiced = false) => ({ status: "settling", invoiced, totals: { balance_due: inr(balance) } }) as OrderResponse;
  it("charges while money is owed, finishes once paid", () => {
    expect(chargeLabel(order(100))).toBe("Pay");
    expect(chargeLabel(order(0))).toBe("Invoice and close");
    expect(chargeLabel(order(0, true))).toBe("Close order");
    expect(chargeLabel(undefined)).toBe("Pay");
  });
});

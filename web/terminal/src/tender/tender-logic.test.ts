import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { OrderResponse, TenderRequest, TenderResponse } from "../../../shared/api-types";
import {
  afterFailure,
  attemptKey,
  chargeBlock,
  chargeLabel,
  closesWhenPaid,
  collectOf,
  elapsedClock,
  payStep,
  payView,
  planTender,
  quickShown,
  quickTenders,
  tenderFailure,
  tenderOutcome,
  approvalRefused,
  tipApprovalAsked,
  tipCeiling,
  tipFor,
  tipNeedsApproval,
  tipNote,
  unsentWords,
} from "./tender-logic";

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

describe("quickShown", () => {
  const cases: [string, number[], number, number[]][] = [
    ["room for all of them", [18000, 19000, 20000, 50000], 4, [18000, 19000, 20000, 50000]],
    ["one too many: the exact amount and the big notes stay, the next ten goes", [20000, 21000, 25000, 30000, 50000], 4, [20000, 25000, 30000, 50000]],
    ["nothing to show", [], 4, []],
  ];
  it.each(cases)("%s", (_name, quick, room, want) => {
    expect(quickShown(quick, room)).toEqual(want);
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
    ["nothing owed", { method: "cash", entered: 50000, tip: 0, due: 0 }, { amount: 0, change: 0, charge: 0, problem: "Nothing is owed on this order." }],
  ];
  it.each(cases)("%s", (_name, input, want) => {
    expect(planTender(input)).toEqual(want);
  });
});

describe("attemptKey", () => {
  const body = (minor: number): TenderRequest => ({ method: "cash", amount: inr(minor), tip: inr(0) });
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
    [
      "owing the guest",
      order({ totals: { balance_due: inr(-100) } as OrderResponse["totals"] }),
      0,
      "The guest is owed money back. A manager settles that on the Orders screen.",
    ],
    ["closed", order({ status: "closed" }), 0, "This order is closed."],
  ];
  it.each(cases)("%s", (_name, o, pending, want) => {
    expect(chargeBlock(o, pending)).toBe(want);
  });
});

describe("chargeLabel", () => {
  const order = (balance: number, extra: Partial<OrderResponse> = {}): OrderResponse =>
    ({
      status: "settling",
      invoiced: false,
      lines: [{ status: "active" }],
      totals: { balance_due: inr(balance) },
      ...extra,
    }) as OrderResponse;
  const nothing = [] as unknown as OrderResponse["lines"];
  const voidedOnly = [{ status: "voided" }] as OrderResponse["lines"];

  const cases: [string, OrderResponse | undefined, string][] = [
    ["money owed", order(100), "Pay"],
    ["paid in full, no invoice yet", order(0), "Invoice and close"],
    ["paid in full and invoiced", order(0, { invoiced: true }), "Close order"],
    ["no order yet", undefined, "Pay"],
    ["an empty table owes nothing but is not paid", order(0, { status: "open", lines: nothing }), "Pay"],
    ["every line voided", order(0, { status: "open", lines: voidedOnly }), "Pay"],
    ["closed", order(0, { status: "closed" }), "Pay"],
  ];
  it.each(cases)("%s", (_name, o, want) => {
    expect(chargeLabel(o)).toBe(want);
  });
});

describe("tipFor", () => {
  const cases: [string, number, number, string, number][] = [
    ["rounds to a whole rupee", 105300, 5, "INR", 5300],
    ["ten percent", 105300, 10, "INR", 10500],
    ["fifteen percent", 105300, 15, "INR", 15800],
    ["a small bill can round to nothing", 900, 5, "INR", 0],
    ["a currency with no minor unit", 1053, 10, "JPY", 105],
    ["nothing owed", 0, 10, "INR", 0],
    ["a credit", -500, 10, "INR", 0],
  ];
  it.each(cases)("%s", (_name, bill, percent, currency, want) => {
    expect(tipFor(bill, percent, currency)).toBe(want);
  });
});

describe("payStep", () => {
  it("is payment, then split while splitting, and complete once paid", () => {
    expect(payStep(false, false)).toBe("payment");
    expect(payStep(false, true)).toBe("split");
    expect(payStep(true, false)).toBe("complete");
    expect(payStep(true, true)).toBe("complete");
  });
});

describe("collectOf", () => {
  it("is what settles the bill plus the tip", () => {
    const plan = planTender({ method: "cash", entered: 120000, tip: 10500, due: 105300 });
    expect(collectOf(plan, 105300, 10500)).toBe(115800);
    const part = planTender({ method: "card", entered: 50000, tip: 0, due: 105300 });
    expect(collectOf(part, 105300, 0)).toBe(50000);
  });
  it("is the whole of what is being paid before an amount is keyed", () => {
    const plan = planTender({ method: "cash", entered: 0, tip: 0, due: 105300 });
    expect(plan.problem).not.toBeNull();
    expect(collectOf(plan, 105300, 5300)).toBe(110600);
  });
});

describe("payView", () => {
  const base = { paid: false, received: false, waiting: false, failed: false, uncertain: false };
  const cases: [string, Partial<typeof base>, string][] = [
    ["keying a payment", {}, "entry"],
    ["a cash refusal stays where it was keyed", { failed: true }, "entry"],
    ["a card being taken on its machine", { waiting: true }, "waiting"],
    ["a card payment the server refused", { waiting: true, failed: true }, "refused"],
    ["money taken, more owed", { received: true }, "received"],
    ["money taken and the bill paid: the paid panel says it", { received: true, paid: true }, "entry"],
    ["a cash payment whose answer was lost", { uncertain: true, failed: true }, "uncertain"],
    ["a card payment whose answer was lost", { uncertain: true, waiting: true, failed: true }, "uncertain"],
  ];
  it.each(cases)("%s", (_n, extra, want) => {
    expect(payView({ ...base, ...extra })).toBe(want);
  });
});

describe("elapsedClock", () => {
  const cases: [number, string][] = [
    [0, "0:00"],
    [24_900, "0:24"],
    [61_000, "1:01"],
    [600_000, "10:00"],
    [-5, "0:00"],
  ];
  it.each(cases)("%d ms is %s", (ms, want) => {
    expect(elapsedClock(ms)).toBe(want);
  });
});

describe("tenderFailure", () => {
  const cases: [string, unknown, string][] = [
    ["no network: it may have been recorded", new ApiError("network", "x"), "uncertain"],
    ["no answer in time", new ApiError("timeout", "x"), "uncertain"],
    ["a gateway timeout", new ApiError("bad_response", "x", 504), "uncertain"],
    ["a bad gateway", new ApiError("bad_response", "x", 502), "uncertain"],
    ["the server failing after it may have written", new ApiError("internal", "x", 500), "uncertain"],
    ["the queue's words around a lost answer", new Error("Not sent: the server can't be reached.", { cause: new ApiError("network", "x") }), "uncertain"],
    ["something that is not the server's at all", new Error("boom"), "uncertain"],
    ["more than is owed: a refusal", new ApiError("over_tender", "x", 409), "refused"],
    ["a bad request: a refusal", new ApiError("invalid_request", "x", 422), "refused"],
    ["a refusal wrapped by the queue", new Error("That's more than is owed now.", { cause: new ApiError("over_tender", "x", 409) }), "refused"],
  ];
  it.each(cases)("%s", (_name, err, want) => {
    expect(tenderFailure(err)).toBe(want);
  });
});

describe("afterFailure", () => {
  const attempt = { key: "k1", body: { method: "cash" as const, amount: inr(45000), tip: inr(0) } };
  it("keeps the attempt, key, method and amount, when the server was not reached", () => {
    expect(afterFailure(attempt, new ApiError("network", "x"))).toBe(attempt);
    expect(afterFailure(attempt, new ApiError("bad_response", "x", 503))).toBe(attempt);
  });
  it("lets it go on a definite refusal", () => {
    expect(afterFailure(attempt, new ApiError("over_tender", "x", 409))).toBeNull();
    expect(afterFailure(attempt, new ApiError("invalid_request", "x", 422))).toBeNull();
  });
});

describe("tenderOutcome", () => {
  const body: TenderRequest = { method: "card", amount: inr(45000), tip: inr(2000) };
  const tender = (id: number, extra: Partial<TenderResponse> = {}): TenderResponse => ({
    id,
    method: "card",
    amount: inr(45000),
    tip: inr(2000),
    status: "captured",
    refunded: inr(0),
    ...extra,
  });
  const order = (tenders: TenderResponse[]) => ({ tenders }) as unknown as OrderResponse;

  it("is recorded when a new tender like it is on the order", () => {
    expect(tenderOutcome([1], body, order([tender(1), tender(2)]))).toBe("recorded");
  });
  it("is absent when the order has no new tender", () => {
    expect(tenderOutcome([1], body, order([tender(1)]))).toBe("absent");
    expect(tenderOutcome([], body, order([]))).toBe("absent");
  });
  it("is absent when the only new tender is another payment, taken elsewhere", () => {
    expect(tenderOutcome([1], body, order([tender(1), tender(2, { method: "cash" })]))).toBe("absent");
    expect(tenderOutcome([1], body, order([tender(1), tender(2, { amount: inr(10000) })]))).toBe("absent");
    expect(tenderOutcome([1], body, order([tender(1), tender(2, { tip: inr(0) })]))).toBe("absent");
  });
  it("does not count a tender already on the order before it was sent", () => {
    expect(tenderOutcome([7], body, order([tender(7)]))).toBe("absent");
  });
});

describe("the tip ceiling (ADR-261)", () => {
  it("is the payment's share at the venue's percent, rounded up to the next paisa", () => {
    const cases: [amount: number, percent: number, want: number][] = [
      [20000, 50, 10000],
      [333, 50, 167], // 166.5 rounds up, as the server rounds
      [1, 1, 1], // a hundredth of a paisa is a paisa
      [1000, 10, 100],
      [20000, 1000, 200000],
      [20000, 0, 0], // no tip without a manager
      [0, 50, 0],
    ];
    for (const [amount, percent, want] of cases) {
      expect(tipCeiling(amount, percent), `${percent}% of ${amount}`).toBe(want);
    }
  });
  it("needs a manager only over it", () => {
    expect(tipNeedsApproval(10000, 20000, 50)).toBe(false); // at it
    expect(tipNeedsApproval(10001, 20000, 50)).toBe(true);
    expect(tipNeedsApproval(167, 333, 50)).toBe(false);
    expect(tipNeedsApproval(168, 333, 50)).toBe(true);
    expect(tipNeedsApproval(500000, 20000, 50)).toBe(true); // 5000 on 200
  });
  it("at zero percent needs one for any tip, and never for none", () => {
    expect(tipNeedsApproval(1, 20000, 0)).toBe(true);
    expect(tipNeedsApproval(0, 20000, 0)).toBe(false);
  });
  it("leaves it to the server before an amount is keyed, or when the till does not know the rule", () => {
    expect(tipNeedsApproval(5000, 0, 50)).toBe(false);
    expect(tipNeedsApproval(500000, 20000, null)).toBe(false);
  });
  it("says so on the tip card", () => {
    expect(tipNote(10001, 20000, 50)).toBe("Over 50%: needs a manager");
    expect(tipNote(1, 20000, 0)).toBe("Any tip needs a manager");
    expect(tipNote(10000, 20000, 50)).toBeNull();
    expect(tipNote(10001, 20000, null)).toBeNull();
  });
  it("hears the server ask for a manager, through the order queue too", () => {
    const asked = new ApiError("tip_needs_approval", "A tip this large needs a manager's approval.", 403, "tip");
    expect(tipApprovalAsked(asked)).toBe(true);
    expect(tipApprovalAsked(new Error("refused", { cause: asked }))).toBe(true);
    expect(tipApprovalAsked(new ApiError("forbidden", "x", 403))).toBe(false);
    expect(tipApprovalAsked(new Error("down"))).toBe(false);
  });
  it("tells the approval refused from the payment refused", () => {
    // The approver's PIN, the approver not allowed, too many tries: the sheet asks again.
    expect(approvalRefused(new ApiError("invalid_pin", "x", 401))).toBe(true);
    expect(approvalRefused(new Error("x", { cause: new ApiError("forbidden", "x", 403, "authorised_by") }))).toBe(true);
    expect(approvalRefused(new ApiError("locked_out", "x", 429))).toBe(true);
    // The cashier's own permission, or the payment itself: the foot says why.
    expect(approvalRefused(new ApiError("forbidden", "x", 403, "payment.take"))).toBe(false);
    expect(approvalRefused(new ApiError("over_tender", "x", 409))).toBe(false);
    expect(approvalRefused(new Error("down"))).toBe(false);
  });
});

describe("closesWhenPaid", () => {
  // ADR-383: the server sends and closes a counter sale with the payment that settles it.
  const cases: ReadonlyArray<{ name: string; order: Pick<OrderResponse, "channel" | "table_id">; closes: boolean }> = [
    { name: "a counter sale with no table", order: { channel: "counter" }, closes: true },
    { name: "a counter sale seated at a table", order: { channel: "counter", table_id: 4 }, closes: false },
    { name: "a guest's round at a table", order: { channel: "qr_table", table_id: 4 }, closes: false },
    { name: "an order to collect", order: { channel: "web" }, closes: false },
  ];
  it.each(cases)("$name: $closes", ({ order, closes }) => {
    expect(closesWhenPaid(order)).toBe(closes);
  });
});

describe("unsentWords", () => {
  const cases: ReadonlyArray<{ waiting: number; sentWithPayment: boolean; words: string | null }> = [
    { waiting: 0, sentWithPayment: true, words: null },
    { waiting: 0, sentWithPayment: false, words: null },
    { waiting: 1, sentWithPayment: true, words: "1 item goes to the kitchen with the payment." },
    { waiting: 3, sentWithPayment: true, words: "3 items go to the kitchen with the payment." },
    { waiting: 1, sentWithPayment: false, words: "1 item has not gone to the kitchen." },
    { waiting: 2, sentWithPayment: false, words: "2 items have not gone to the kitchen." },
  ];
  it.each(cases)("$waiting waiting, sent with the payment $sentWithPayment", ({ waiting, sentWithPayment, words }) => {
    expect(unsentWords(waiting, sentWithPayment)).toBe(words);
  });
});

import { describe, expect, it } from "vitest";
import type { LineResponse, OrderResponse, TenderResponse } from "../../../shared/api-types";
import {
  anythingLeft,
  closedText,
  commandIdentity,
  fullSelection,
  leftOn,
  refundReasonText,
  refundable,
  selectionOf,
  unitsLeft,
  unitsText,
  voidable,
  whereOf,
} from "./bills-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const line = (extra: Partial<LineResponse>): LineResponse =>
  ({ id: 2, qty: 3, status: "active", credited_qty: 0, credited: inr(0), due: inr(37800), ...extra }) as LineResponse;

const tender = (extra: Partial<TenderResponse>): TenderResponse =>
  ({ id: 3, method: "card", amount: inr(12600), tip: inr(0), status: "captured", refunded: inr(0), ...extra }) as TenderResponse;

describe("unitsLeft", () => {
  it("counts what is not credited, on active lines only", () => {
    expect(unitsLeft(line({}))).toBe(3);
    expect(unitsLeft(line({ credited_qty: 2 }))).toBe(1);
    expect(unitsLeft(line({ status: "voided" }))).toBe(0);
  });
});

describe("anythingLeft", () => {
  it("is false once every line is credited in full", () => {
    const order = (lines: LineResponse[]) => ({ lines }) as OrderResponse;
    expect(anythingLeft(order([line({})]))).toBe(true);
    expect(anythingLeft(order([line({ credited: inr(37800) })]))).toBe(false);
    expect(anythingLeft(order([line({ status: "comped" })]))).toBe(false);
  });
});

describe("selectionOf", () => {
  it("asks for the units chosen, in line order, and nothing for none", () => {
    expect(selectionOf({ kind: "items", units: { 3: 1, 2: 2, 4: 0 } }, "INR")).toEqual({
      lines: [
        { line_id: 2, qty: 2 },
        { line_id: 3, qty: 1 },
      ],
    });
    expect(selectionOf({ kind: "items", units: {} }, "INR")).toBeNull();
  });
  it("asks for an amount, and nothing for zero", () => {
    expect(selectionOf({ kind: "amount", minor: 10000 }, "INR")).toEqual({ amount: inr(10000) });
    expect(selectionOf({ kind: "amount", minor: 0 }, "INR")).toBeNull();
  });
});

describe("fullSelection", () => {
  const lines = [line({ id: 2 }), line({ id: 3, credited_qty: 2 }), line({ id: 4, status: "voided" }), line({ id: 5, credited_qty: 3 })];
  it("is every unit not yet credited, on the lines still standing", () => {
    expect(fullSelection(lines)).toEqual({
      lines: [
        { line_id: 2, qty: 3 },
        { line_id: 3, qty: 1 },
      ],
    });
    expect(selectionOf({ kind: "full" }, "INR", lines)).toEqual(fullSelection(lines));
  });
  it("is nothing when nothing is left", () => {
    expect(fullSelection([line({ credited_qty: 3 })])).toBeNull();
    expect(selectionOf({ kind: "full" }, "INR")).toBeNull();
  });
});

describe("unitsText", () => {
  const lines = [
    { id: 2, name: "Masala dosa" },
    { id: 3, name: "Iced latte" },
  ];
  it("names what was chosen, in the bill's order", () => {
    expect(unitsText({ 3: 1, 2: 2 }, lines)).toBe("2 × Masala dosa, 1 × Iced latte");
    expect(unitsText({ 3: 1, 2: 0 }, lines)).toBe("1 × Iced latte");
    expect(unitsText({}, lines)).toBe("");
  });
});

describe("leftOn", () => {
  it("is what a payment has not given back", () => {
    expect(leftOn(tender({}))).toBe(12600);
    expect(leftOn(tender({ refunded: inr(2600) }))).toBe(10000);
  });
});

describe("payments", () => {
  it("offers only what can take money back", () => {
    const ts = [tender({ id: 3 }), tender({ id: 4, status: "voided" }), tender({ id: 5, refunded: inr(12600) }), tender({ id: 6, refunded: inr(100) })];
    expect(refundable(ts).map((t) => t.id)).toEqual([3, 6]);
  });
  it("voids only a payment nothing has gone back on", () => {
    expect(voidable(tender({}))).toBe(true);
    expect(voidable(tender({ refunded: inr(100) }))).toBe(false);
    expect(voidable(tender({ status: "voided" }))).toBe(false);
  });
});

describe("words", () => {
  it("says where and when", () => {
    expect(whereOf({ table_id: 4 })).toBe("Table 4");
    expect(whereOf({})).toBe("Counter");
    const now = new Date(2026, 8, 25, 18, 0);
    expect(closedText(new Date(2026, 8, 25, 13, 5).toISOString(), now)).toBe("13:05");
    expect(closedText(new Date(2026, 8, 24, 21, 30).toISOString(), now)).toBe("24/09 21:30");
  });

  it("says a refund's reason as the till offered it", () => {
    expect(refundReasonText("wrong_item")).toBe("Dish was wrong");
    expect(refundReasonText("stale_code")).toBe("Stale code");
    expect(refundReasonText("")).toBe("");
  });
});

describe("commandIdentity", () => {
  const refund = (approval: { authorised_by: string; pin: string }, extra: Record<string, unknown> = {}) => ({
    ...approval,
    credit: { lines: [{ line_id: 2, qty: 1 }] },
    refunds: [{ payment_id: 3, amount: inr(12600) }],
    reason: "wrong_item",
    ...extra,
  });
  const asha = { authorised_by: "asha", pin: "1234" };
  const ravi = { authorised_by: "ravi", pin: "9876" };

  it("is the same for a retry another manager approves, so it cannot refund twice", () => {
    expect(commandIdentity("/credit-notes", refund(ravi))).toBe(commandIdentity("/credit-notes", refund(asha)));
    expect(commandIdentity("/payments/3/void", { ...ravi, reason: "charged_twice" })).toBe(
      commandIdentity("/payments/3/void", { ...asha, reason: "charged_twice" }),
    );
  });
  it("is the same when a PIN is typed again", () => {
    expect(commandIdentity("/invoices/4/reprint", { ...asha, reason: "x" })).toBe(
      commandIdentity("/invoices/4/reprint", { authorised_by: "asha", pin: "0000", reason: "x" }),
    );
  });
  it("differs when the command does", () => {
    const first = commandIdentity("/credit-notes", refund(asha));
    expect(commandIdentity("/credit-notes", refund(asha, { reason: "goodwill" }))).not.toBe(first);
    expect(commandIdentity("/credit-notes", refund(asha, { refunds: [{ payment_id: 3, amount: inr(100) }] }))).not.toBe(first);
    expect(commandIdentity("/credit-notes", refund(asha, { refunds: [{ payment_id: 5, amount: inr(12600) }] }))).not.toBe(first);
    expect(commandIdentity("/payments/3/void", { reason: "x" })).not.toBe(commandIdentity("/payments/4/void", { reason: "x" }));
  });
  it("takes a command with no body", () => {
    expect(commandIdentity("/close", undefined)).toBe(commandIdentity("/close", undefined));
    expect(commandIdentity("/close", undefined)).not.toBe(commandIdentity("/hold", undefined));
  });
});

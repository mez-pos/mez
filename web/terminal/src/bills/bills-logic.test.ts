import { describe, expect, it } from "vitest";
import type { LineResponse, OrderResponse, TenderResponse } from "../../../shared/api-types";
import { anythingLeft, closedText, refundReasonText, refundable, selectionOf, unitsLeft, voidable, whereOf } from "./bills-logic";

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

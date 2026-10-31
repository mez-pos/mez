import { describe, expect, it } from "vitest";
import type { DayOrderResponse } from "../../../shared/api-types";
import { carryWords, openOrders, openOrdersTitle, openOrdersWhere } from "./open-orders-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const row = (extra: Partial<DayOrderResponse>): DayOrderResponse => ({
  id: "ord_1",
  status: "open",
  service: "dine_in",
  covers: 2,
  opened_at: "2026-10-31T10:00:00Z",
  items: [],
  total: inr(50000),
  credited: inr(0),
  balance_due: inr(50000),
  ...extra,
});

const names = new Map([
  [5, "T5"],
  [7, "T7"],
  [9, "T9"],
  [12, "Patio 2"],
]);

describe("openOrders", () => {
  it("keeps what the Orders screen's Open tab shows: a paid table's ticket is not open (ADR-383)", () => {
    const day = [row({ id: "a" }), row({ id: "b", status: "paid", table_id: 5 }), row({ id: "c", status: "refunded" }), row({ id: "d", table_id: 7 })];
    expect(openOrders(day).map((o) => o.id)).toEqual(["a", "d"]);
  });
});

describe("openOrdersTitle", () => {
  it("counts the open orders", () => {
    expect(openOrdersTitle(1)).toBe("1 order is still open");
    expect(openOrdersTitle(3)).toBe("3 orders are still open");
  });
  it("is nothing with none open", () => {
    expect(openOrdersTitle(0)).toBeNull();
  });
});

describe("openOrdersWhere", () => {
  it("names tables and pickup codes, and counts the counter's", () => {
    expect(openOrdersWhere([row({ table_id: 5 }), row({ table_id: 7 }), row({})], names)).toBe("T5, T7 and 1 at the counter");
    expect(openOrdersWhere([row({ pickup_code: "B7K", service: "takeaway" })], names)).toBe("B7K");
    expect(openOrdersWhere([row({}), row({})], names)).toBe("2 at the counter");
  });
  it("names three, and counts the rest", () => {
    const five = [5, 7, 9, 12, 4].map((t) => row({ table_id: t }));
    expect(openOrdersWhere([...five, row({})], names)).toBe("T5, T7, T9, 2 more and 1 at the counter");
  });
  it("is nothing for no orders", () => {
    expect(openOrdersWhere([], names)).toBe("");
  });
});

describe("carryWords", () => {
  it("tells the confirm what stays open when the count goes in (TBS-2)", () => {
    expect(carryWords(2)).toBe("2 orders are still open. They carry into the next shift.");
    expect(carryWords(1)).toBe("1 order is still open. It carries into the next shift.");
  });
  it("says nothing with none open, or none known", () => {
    expect(carryWords(0)).toBeNull();
    expect(carryWords(null)).toBeNull();
  });
});

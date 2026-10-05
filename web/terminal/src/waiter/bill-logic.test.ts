import { describe, expect, it } from "vitest";
import type { OrderResponse, PaymentMethod, SplitResponse } from "../../../shared/api-types";
import { closeBlock, eachShare, shareName, shareRows, shareStatus, tablePays } from "./bill-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });
const split = (amounts: number[], kind: SplitResponse["kind"] = "evenly"): SplitResponse => ({
  kind,
  total: inr(amounts.reduce((a, b) => a + b, 0)),
  shares: amounts.map((minor, payer) => ({ payer, amount: inr(minor), line_ids: [] })),
});
const order = (balance: number, status: OrderResponse["status"] = "open"): OrderResponse =>
  ({ status, totals: { balance_due: inr(balance) } }) as unknown as OrderResponse;
const paidBy = (entries: [number, PaymentMethod][]) => new Map(entries);

describe("shareName", () => {
  it("is the whole bill when nobody shares it", () => {
    expect(shareName("evenly", 0, 0, 1)).toBe("Whole bill");
  });
  it("counts guests from one", () => {
    expect(shareName("evenly", 1, 0, 3)).toBe("Guest 2");
    expect(shareName("item", 0, 0, 2)).toBe("Guest 1");
  });
  it("names what custom amounts leave over", () => {
    expect(shareName("amount", 2, 2, 3)).toBe("The rest");
    expect(shareName("amount", 1, 2, 3)).toBe("Guest 2");
  });
});

describe("eachShare", () => {
  it("is the share when every guest pays the same", () => {
    expect(eachShare(split([81500, 81500, 81500]))).toBe(81500);
  });
  it("is nothing to say when the shares differ by the odd paisa, or there is one", () => {
    expect(eachShare(split([33334, 33333, 33333]))).toBeNull();
    expect(eachShare(split([244500]))).toBeNull();
  });
});

describe("shareRows", () => {
  const three = split([81500, 81500, 81500]);
  it("says who has paid and how, and which share is being collected", () => {
    const rows = shareRows(three, 0, paidBy([[0, "upi"]]), 1);
    expect(rows.map((r) => [r.name, r.state, r.paidBy])).toEqual([
      ["Guest 1", "paid", "upi"],
      ["Guest 2", "collecting", null],
      ["Guest 3", "waiting", null],
    ]);
    expect(rows.map(shareStatus)).toEqual(["Paid · UPI", "Collecting now", "Awaiting payment"]);
  });
  it("never collects a share that is paid", () => {
    expect(shareRows(three, 0, paidBy([[0, "cash"]]), 0)[0]?.state).toBe("paid");
  });
  it("is one row for a bill nobody shares", () => {
    expect(shareRows(split([244500]), 0, paidBy([]), 0).map((r) => r.name)).toEqual(["Whole bill"]);
  });
});

describe("tablePays", () => {
  it("is every share's payer until one is picked", () => {
    expect(tablePays(split([100, 100]), paidBy([]), null)).toBe(0);
    expect(tablePays(split([100, 100]), paidBy([[0, "cash"]]), null)).toBe(1);
  });
  it("keeps the share picked while it is unpaid", () => {
    expect(tablePays(split([100, 100, 100]), paidBy([[0, "cash"]]), 2)).toBe(2);
  });
  it("moves on from a share picked and since paid", () => {
    expect(tablePays(split([100, 100, 100]), paidBy([[1, "card"]]), 1)).toBe(0);
  });
  it("is nobody once every share is paid", () => {
    expect(
      tablePays(
        split([100, 100]),
        paidBy([
          [0, "cash"],
          [1, "upi"],
        ]),
        null,
      ),
    ).toBeNull();
  });
});

describe("closeBlock", () => {
  it("lets a paid bill close", () => {
    expect(closeBlock(order(0))).toBeNull();
  });
  it("waits for the whole bill", () => {
    expect(closeBlock(order(81500))).toBe("Collect the whole bill to close the table.");
  });
  it("does not close over money owed back", () => {
    expect(closeBlock(order(-500))).toBe("The guest is owed money back. A manager settles that on the Orders screen.");
  });
  it("has nothing to close twice", () => {
    expect(closeBlock(order(0, "closed"))).toBe("This order is closed.");
  });
});

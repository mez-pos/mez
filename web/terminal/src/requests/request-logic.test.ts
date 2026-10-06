import { describe, expect, it } from "vitest";
import { arrived, choicesOf, dishCount, gatewayName, methodWord, owesOtherwise, pillLabel, requestHeading, saidAgo, waited, wantingTables } from "./request-logic";

const ids = (...list: string[]) => list.map((id) => ({ id }));

describe("arrived", () => {
  it("tells a terminal that has not looked yet of everything waiting", () => {
    expect(arrived(null, ids("a", "b"))).toEqual(["a", "b"]);
  });
  it("names only what was not there at the last look", () => {
    expect(arrived(new Set(["a"]), ids("a", "b", "c"))).toEqual(["b", "c"]);
  });
  it("says nothing when nothing is new, or one was answered", () => {
    expect(arrived(new Set(["a", "b"]), ids("a", "b"))).toEqual([]);
    expect(arrived(new Set(["a", "b"]), ids("b"))).toEqual([]);
    expect(arrived(new Set(), [])).toEqual([]);
  });
});

describe("wantingTables", () => {
  it("is each table once, however many phones asked", () => {
    const tables = wantingTables([{ table_id: 4 }, { table_id: 4 }, { table_id: 7 }]);
    expect([...tables].sort()).toEqual([4, 7]);
    expect(wantingTables([]).size).toBe(0);
  });
});

describe("pillLabel", () => {
  it("agrees with the count", () => {
    expect(pillLabel(1)).toBe("table request");
    expect(pillLabel(2)).toBe("table requests");
    expect(pillLabel(0, 1)).toBe("table says paid");
    expect(pillLabel(0, 3)).toBe("tables say paid");
    expect(pillLabel(2, 1)).toBe("from tables");
    expect(pillLabel(0, 0, 1)).toBe("payment to sort out");
    expect(pillLabel(0, 0, 2)).toBe("payments to sort out");
    expect(pillLabel(1, 0, 1)).toBe("from tables");
    expect(pillLabel(0, 2, 1)).toBe("from tables");
  });
});

describe("waited", () => {
  const at = "2026-10-06T10:00:00Z";
  const after = (minutes: number, seconds = 0) => Date.parse(at) + minutes * 60_000 + seconds * 1000;
  const cases: [string, number, string][] = [
    ["under a minute", after(0, 59), "Just now"],
    ["a minute", after(1), "1 min"],
    ["most of an hour", after(59, 59), "59 min"],
    ["an hour", after(60), "1 h"],
    ["an hour and some", after(65), "1 h 5 min"],
    ["a clock set behind the server's", after(-3), "Just now"],
  ];
  it.each(cases)("%s", (_name, now, want) => {
    expect(waited(at, now)).toBe(want);
  });
  it("does not print a time it cannot read", () => {
    expect(waited("not a time", after(5))).toBe("Just now");
  });
});

describe("choicesOf and dishCount", () => {
  it("joins the choices, and is empty with none", () => {
    expect(choicesOf({ choices: ["Oat", "Extra shot"] })).toBe("Oat, Extra shot");
    expect(choicesOf({ choices: [] })).toBe("");
  });
  it("counts every dish, not every line", () => {
    const line = { item_id: 1, name: "x", choices: [], total: { minor: 0, currency: "INR" } };
    expect(
      dishCount({
        lines: [
          { ...line, qty: 2 },
          { ...line, qty: 1 },
        ],
      }),
    ).toBe(3);
  });
});

describe("owesOtherwise", () => {
  const inr = (minor: number) => ({ minor, currency: "INR" });
  it("is false while the table owes what the guest said they paid", () => {
    expect(owesOtherwise({ amount: inr(54000), due: inr(54000) })).toBe(false);
  });
  it("is true once a round was added or part was recorded", () => {
    expect(owesOtherwise({ amount: inr(54000), due: inr(84000) })).toBe(true);
    expect(owesOtherwise({ amount: inr(54000), due: inr(20000) })).toBe(true);
  });
});

describe("saidAgo", () => {
  const at = "2026-10-06T10:00:00Z";
  it("reads as a sentence at any age", () => {
    expect(saidAgo(at, Date.parse(at) + 20_000)).toBe("Said just now");
    expect(saidAgo(at, Date.parse(at) + 4 * 60_000)).toBe("Said 4 min ago");
    expect(saidAgo(at, Date.parse(at) + 65 * 60_000)).toBe("Said 1 h 5 min ago");
  });
});

describe("methodWord and gatewayName", () => {
  it("words a method for staff", () => {
    expect(methodWord("upi")).toBe("UPI");
    expect(methodWord("card")).toBe("card");
    expect(methodWord("other")).toBe("online");
    expect(methodWord("")).toBe("online");
  });
  it("names a gateway as staff know it, and leaves one it does not know as it is", () => {
    expect(gatewayName("razorpay")).toBe("Razorpay");
    expect(gatewayName("test")).toBe("test");
  });
});

describe("requestHeading", () => {
  it("names the table for a table's first order", () => {
    expect(requestHeading({ table_name: "5" })).toEqual({ title: "Table 5", note: null });
    expect(requestHeading({ table_name: "5", joins_open_ticket: false })).toEqual({ title: "Table 5", note: null });
  });
  it("says a new phone asks to join a table that has a bill open", () => {
    const heading = requestHeading({ table_name: "5", joins_open_ticket: true });
    expect(heading.title).toBe("New phone at Table 5");
    expect(heading.note).toContain("join the table's open bill");
  });
});

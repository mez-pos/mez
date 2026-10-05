import { describe, expect, it } from "vitest";
import type { Group, StaffTally } from "../../../shared/api-types";
import { linesText, removedLines, staffLines, tipTakers } from "./staff-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

function group(key: string, name: string, qty: number, total: number): Group {
  const z = inr(0);
  return { key, name, qty, gross: z, discounts: z, net: z, tax: z, total: inr(total) };
}

const tally = (staff_id: number, count: number, amount: number): StaffTally => ({ staff_id, count, amount: inr(amount) });

describe("staffLines", () => {
  const names = { "3": "Meera", "5": "Kabir", "7": "Asha" };
  const roles = { "3": "cashier", "5": "waiter", "7": "" };

  it("joins the bills each person opened with the tips they took", () => {
    const got = staffLines(
      { currency: "INR", by_staff: [group("5", "Kabir", 9, 540_000), group("3", "Meera", 3, 120_000)], tips_by: [tally(5, 4, 30_000)] },
      names,
      roles,
    );
    expect(got).toEqual([
      { id: "5", name: "Kabir", role: "waiter", guest: false, bills: 9, sales: inr(540_000), average: inr(60_000), tips: inr(30_000) },
      { id: "3", name: "Meera", role: "cashier", guest: false, bills: 3, sales: inr(120_000), average: inr(40_000), tips: inr(0) },
    ]);
  });

  it("lists someone who only took payments, with no bills of their own", () => {
    const got = staffLines({ currency: "INR", by_staff: [group("5", "Kabir", 2, 50_000)], tips_by: [tally(3, 1, 5_000), tally(7, 1, 9_000)] }, names, roles);
    expect(got.map((l) => [l.name, l.bills, l.average, l.tips.minor])).toEqual([
      ["Kabir", 2, inr(25_000), 0],
      ["Asha", 0, null, 9_000],
      ["Meera", 0, null, 5_000],
    ]);
    expect(got[1]?.role).toBe("");
  });

  it("orders by sales, then tips, then name", () => {
    const got = staffLines(
      {
        currency: "INR",
        by_staff: [group("3", "Meera", 1, 10_000), group("5", "Kabir", 1, 10_000), group("7", "Asha", 1, 10_000)],
        tips_by: [tally(5, 1, 500)],
      },
      names,
      roles,
    );
    expect(got.map((l) => l.name)).toEqual(["Kabir", "Asha", "Meera"]);
  });

  it("still lists a tip taker the report cannot name", () => {
    expect(staffLines({ currency: "INR", by_staff: [], tips_by: [tally(9, 1, 100)] }, {}, {})[0]?.name).toBe("Someone no longer on record");
  });

  it("names the bills no staff member opened as a guest's", () => {
    const got = staffLines({ currency: "INR", by_staff: [group("0", "", 2, 30_000)], tips_by: [] }, names, roles);
    expect(got.map((l) => [l.name, l.guest, l.role])).toEqual([["A guest", true, ""]]);
  });

  it("is empty for a quiet range", () => {
    expect(staffLines({ currency: "INR", by_staff: [], tips_by: [] }, {}, {})).toEqual([]);
  });
});

describe("removedLines", () => {
  it("names who took lines off, the most first", () => {
    expect(removedLines([tally(3, 1, 12_000), tally(5, 2, 30_000), tally(9, 1, 500)], { "3": "Meera", "5": "Kabir" })).toEqual([
      { id: "5", name: "Kabir", count: 2, amount: inr(30_000) },
      { id: "3", name: "Meera", count: 1, amount: inr(12_000) },
      { id: "9", name: "Someone no longer on record", count: 1, amount: inr(500) },
    ]);
  });
  it("counts who took tips, not someone whose earlier tip was voided", () => {
    expect(tipTakers([tally(5, 2, 3000), tally(3, -1, -1500), tally(7, 0, 0)])).toBe(1);
    expect(tipTakers([])).toBe(0);
  });
  it("counts lines in words", () => {
    expect(linesText(1)).toBe("1 line");
    expect(linesText(3)).toBe("3 lines");
  });
});

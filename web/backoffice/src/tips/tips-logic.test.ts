import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { ErrorCode, TipPoolResponse } from "../../../shared/api-types";
import { membersOf, ruleWords, startingRows, tipProblem, toMinutes } from "./tips-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

const pool: TipPoolResponse = {
  shift_id: 1,
  pooled: false,
  rule: "hours",
  total: inr(7000),
  members: [
    { staff_id: "stf_RAVI0001", name: "Ravi", role: "cashier", in_pool: true, minutes: 0, weight: 1, taken: inr(5000), share: inr(0) },
    { staff_id: "stf_PRIYA001", name: "Priya", role: "manager", in_pool: true, minutes: 270, weight: 2, taken: inr(2000), share: inr(0) },
  ],
};

describe("toMinutes", () => {
  it("turns hours and minutes into minutes", () => {
    expect(toMinutes("8", "30")).toBe(510);
    expect(toMinutes("", "45")).toBe(45);
    expect(toMinutes("4", "")).toBe(240);
    expect(toMinutes("", "")).toBe(0);
  });
  it("refuses what is not a time", () => {
    expect(toMinutes("8.5", "")).toBeNull();
    expect(toMinutes("-1", "")).toBeNull();
    expect(toMinutes("1", "60")).toBeNull();
    expect(toMinutes("37", "")).toBeNull();
    expect(toMinutes("eight", "")).toBeNull();
  });
});

describe("startingRows and membersOf", () => {
  it("starts with the suggested people in, then the rest of the staff out", () => {
    const rows = startingRows(pool, [
      { id: "stf_PRIYA001", name: "Priya", role: "owner" },
      { id: "stf_ASHA0001", name: "Asha", role: "cashier" },
    ]);
    expect(rows.map((r) => [r.staffId, r.inPool])).toEqual([
      ["stf_RAVI0001", true],
      ["stf_PRIYA001", true],
      ["stf_ASHA0001", false],
    ]);
    expect(rows[1]).toMatchObject({ hours: "4", minutes: "30" });
    expect(membersOf(rows)).toEqual([
      { staff_id: "stf_RAVI0001", minutes: 0 },
      { staff_id: "stf_PRIYA001", minutes: 270 },
    ]);
  });
  it("sends nothing while someone's hours are not a time", () => {
    const rows = startingRows(pool, []).map((r) => ({ ...r, minutes: "75" }));
    expect(membersOf(rows)).toBeNull();
  });
});

describe("words", () => {
  it("names the rules", () => {
    expect(ruleWords("even")).toBe("Split evenly");
    expect(ruleWords("hours")).toBe("Split by hours worked");
  });
  it("says why a pool was refused", () => {
    const refused = (code: ErrorCode, message = "") => new ApiError(code, message, 409);
    expect(tipProblem(refused("already_pooled"), "even")).toMatch(/already pooled/);
    expect(tipProblem(refused("invalid_tip_pool", "invalid tip pool: staff 3 has no hours"), "hours")).toBe("Enter hours for everyone in the pool.");
    expect(tipProblem(refused("invalid_tip_pool", "invalid tip pool: nobody is in it"), "hours")).toBe("Put at least one person in the pool.");
    expect(tipProblem(refused("forbidden"), "even")).toMatch(/permission/);
  });
});

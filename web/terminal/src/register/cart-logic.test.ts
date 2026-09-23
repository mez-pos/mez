import { describe, expect, it } from "vitest";
import type { LineResponse } from "../../../shared/api-types";
import { humanise } from "../../../shared/ui/timeline-entries";
import {
  compReasons,
  discountProblem,
  discountReasons,
  discountRequest,
  formatPercent,
  lineActions,
  voidReasons,
} from "./cart-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });
const override = { authorised_by: "stf_MGR00001", pin: "2468" };

const line = (extra: Partial<LineResponse> = {}): LineResponse => ({
  id: 2, item_id: 1, name: "Flat white", qty: 1, unit_price: inr(21000), status: "active",
  course_fired: false, sent: false, gross: inr(21000), discount: inr(0), tax: inr(1000), due: inr(21000),
  ...extra,
});

describe("lineActions", () => {
  const cases: [string, Partial<LineResponse>, Record<string, boolean>][] = [
    ["an unsent line allows everything", {}, { change: true, note: true, void: true, comp: true, discount: true }],
    ["a sent line cannot change", { sent: true }, { change: false, void: true, comp: true, discount: true }],
    ["a fired course cannot change", { course_fired: true }, { change: false, void: true }],
    ["a voided line only takes a note", { status: "voided" }, { change: false, note: true, void: false, comp: false, discount: false }],
    ["a comped line cannot be voided", { status: "comped" }, { void: false, comp: false }],
  ];
  it.each(cases)("%s", (_, extra, want) => {
    expect(lineActions(line(extra))).toMatchObject(want);
  });
});

describe("reasons", () => {
  it("are stable snake_case codes that read back as their label's meaning", () => {
    for (const r of [...voidReasons, ...compReasons, ...discountReasons]) {
      expect(r.code).toMatch(/^[a-z]+(_[a-z]+)*$/);
      expect(humanise(r.code)).toBeTruthy();
    }
    expect(new Set(voidReasons.map((r) => r.code)).size).toBe(voidReasons.length);
  });
});

describe("formatPercent", () => {
  it.each([
    [1000, "10%"],
    [1250, "12.5%"],
    [1205, "12.05%"],
    [5, "0.05%"],
    [10000, "100%"],
  ])("%i is %s", (bp, text) => {
    expect(formatPercent(bp)).toBe(text);
  });
});

describe("discountProblem", () => {
  const worth = inr(21000);
  const cases: [string, Parameters<typeof discountProblem>[0], string | null][] = [
    ["no percentage", { kind: "pct", basisPoints: 0 }, "Enter a percentage."],
    ["over 100%", { kind: "pct", basisPoints: 10001 }, "A discount can't be more than 100%."],
    ["100% is allowed", { kind: "pct", basisPoints: 10000 }, null],
    ["no amount", { kind: "amount", minor: 0 }, "Enter an amount."],
    ["more than it's worth", { kind: "amount", minor: 21001 }, "That's more than it's worth."],
    ["exactly its worth", { kind: "amount", minor: 21000 }, null],
  ];
  it.each(cases)("%s", (_, entry, want) => {
    expect(discountProblem(entry, worth)).toBe(want);
  });
});

describe("discountRequest", () => {
  it("builds a line percentage", () => {
    expect(discountRequest({ lineId: 2 }, { kind: "pct", basisPoints: 1000 }, "INR", "regular", override)).toEqual({
      ...override, scope: "line", line_id: 2, kind: "pct", basis_points: 1000, reason: "regular",
    });
  });

  it("builds an order amount, in the order's currency", () => {
    expect(discountRequest("order", { kind: "amount", minor: 5000 }, "INR", "complaint", override)).toEqual({
      ...override, scope: "order", kind: "amount", amount: inr(5000), reason: "complaint",
    });
  });
});

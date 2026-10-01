import { describe, expect, it } from "vitest";
import type { AuditEntry } from "../../../shared/api-types";
import { auditQuery, authorisedText, dayText, itemText, kinds, kindWords, lastDays, reasonWords, tradingDay } from "./audit-logic";

const entry = (over: Partial<AuditEntry>): AuditEntry => ({ id: 1, kind: "void", at: "2026-10-01T06:30:00Z", order_id: "ord_1", ...over });

describe("trading days", () => {
  it("counts the hours before 04:00 as the day before", () => {
    expect(tradingDay(new Date(2026, 9, 1, 12, 0))).toBe("2026-10-01");
    expect(tradingDay(new Date(2026, 9, 1, 4, 0))).toBe("2026-10-01");
    expect(tradingDay(new Date(2026, 9, 1, 3, 59))).toBe("2026-09-30");
    expect(tradingDay(new Date(2026, 0, 1, 1, 30))).toBe("2025-12-31");
  });
  it("gives the last n days, today included, across a month", () => {
    expect(lastDays(new Date(2026, 9, 3, 12, 0), 7)).toEqual({ from: "2026-09-27", to: "2026-10-03" });
    expect(lastDays(new Date(2026, 9, 3, 2, 0), 1)).toEqual({ from: "2026-10-02", to: "2026-10-02" });
  });
  it("words a day", () => {
    expect(dayText("2026-09-24")).toMatch(/24 Sep/);
  });
});

describe("the words", () => {
  it("has words for every kind", () => {
    expect(kinds).toEqual(["void", "comp", "discount", "refund", "payment_void", "reopen"]);
    expect(kindWords.payment_void.many).toBe("Payment voids");
  });
  it("words a reason code and leaves free text alone", () => {
    expect(reasonWords("wrong_item")).toBe("Wrong item");
    expect(reasonWords("regular")).toBe("Regular");
    expect(reasonWords("Card declined, tried twice")).toBe("Card declined, tried twice");
    expect(reasonWords(undefined)).toBe("");
  });
  it("says who let it through", () => {
    const asha = { id: "stf_A", name: "Asha" };
    const priya = { id: "stf_P", name: "Priya" };
    expect(authorisedText(entry({ by: asha, authorised_by: priya }))).toEqual({ text: "Priya", muted: false });
    expect(authorisedText(entry({ by: asha, authorised_by: asha }))).toEqual({ text: "Themselves", muted: true });
    expect(authorisedText(entry({ by: asha }))).toEqual({ text: "Not recorded", muted: true });
  });
  it("says what it was about", () => {
    expect(itemText(entry({ item: "Chai", qty: 2 }))).toBe("Chai × 2");
    expect(itemText(entry({ kind: "discount" }))).toBe("Whole bill");
    expect(itemText(entry({ kind: "reopen" }))).toBe("");
  });
});

describe("auditQuery", () => {
  it("sends the range, and the person, kind and cursor when given", () => {
    const f = { from: "2026-09-24", to: "2026-09-30", staff: "", kind: null };
    expect(auditQuery(f, 0)).toBe("/api/v1/audit?from=2026-09-24&to=2026-09-30&limit=50");
    expect(auditQuery({ ...f, staff: "stf_A", kind: "void" }, 120, 25)).toBe(
      "/api/v1/audit?from=2026-09-24&to=2026-09-30&limit=25&staff=stf_A&kind=void&before=120",
    );
  });
});

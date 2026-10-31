import { describe, expect, it } from "vitest";
import type { AuditEntry } from "../../../shared/api-types";
import { auditLink, auditQuery, authorisedText, filterFromSearch, itemText, kinds, kindWords, kindsText, reasonWords } from "./audit-logic";

const entry = (over: Partial<AuditEntry>): AuditEntry => ({ id: 1, kind: "void", at: "2026-10-01T06:30:00Z", order_id: "ord_1", ...over });

describe("the words", () => {
  it("has words for every kind", () => {
    expect(kinds).toEqual(["void", "comp", "discount", "refund", "payment_void", "reopen", "tip_approved"]);
    expect(kindWords.payment_void.many).toBe("Payment voids");
    // A tip a manager let through over the venue's ceiling (ADR-261).
    expect(kindWords.tip_approved).toEqual({ one: "Tip approved", many: "Tips approved" });
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

describe("opening on an address", () => {
  // Noon in Kolkata on 3 Oct: the week is the venue's, not the browser's (ADR-385).
  const now = new Date("2026-10-03T06:30:00Z");
  const clock = { timezone: "Asia/Kolkata", dayStart: 240 };
  const week = { from: "2026-09-27", to: "2026-10-03" };
  it("links to a filter and reads it back", () => {
    const link = auditLink({ from: "2026-09-01", to: "2026-09-30", staff: "stf_ASHA0001", kind: "void" });
    expect(link).toBe("/audit?from=2026-09-01&to=2026-09-30&staff=stf_ASHA0001&kind=void");
    expect(filterFromSearch(link.slice(link.indexOf("?")), now, clock)).toEqual({ from: "2026-09-01", to: "2026-09-30", staff: "stf_ASHA0001", kind: "void" });
  });
  it("leaves out what is not asked for", () => {
    expect(auditLink({ from: "2026-09-01", to: "2026-09-30" })).toBe("/audit?from=2026-09-01&to=2026-09-30");
    expect(auditLink({ from: "2026-09-01", to: "2026-09-30", staff: "", kind: null })).toBe("/audit?from=2026-09-01&to=2026-09-30");
  });
  const tests: { name: string; search: string; want: ReturnType<typeof filterFromSearch> }[] = [
    { name: "no address", search: "", want: { ...week, staff: "", kind: null } },
    { name: "a kind alone keeps the default days", search: "?kind=refund", want: { ...week, staff: "", kind: "refund" } },
    {
      name: "a kind we do not have",
      search: "?from=2026-09-01&to=2026-09-30&kind=theft",
      want: { from: "2026-09-01", to: "2026-09-30", staff: "", kind: null },
    },
    { name: "a day that is not one", search: "?from=yesterday&to=2026-09-30", want: { ...week, staff: "", kind: null } },
    { name: "one day missing", search: "?from=2026-09-01", want: { ...week, staff: "", kind: null } },
    { name: "backwards", search: "?from=2026-09-30&to=2026-09-01", want: { ...week, staff: "", kind: null } },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(filterFromSearch(tt.search, now, clock)).toEqual(tt.want);
    });
  }
});

describe("what a set of corrections were", () => {
  it("says each kind there was, one or many", () => {
    expect(
      kindsText([
        { kind: "void", count: 2 },
        { kind: "comp", count: 1 },
        { kind: "discount", count: 0 },
        { kind: "payment_void", count: 3 },
      ]),
    ).toBe("2 voids · 1 comp · 3 payment voids");
  });
  it("says nothing for none", () => {
    expect(kindsText([{ kind: "void", count: 0 }])).toBe("");
    expect(kindsText([])).toBe("");
  });
});

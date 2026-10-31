import { describe, expect, it } from "vitest";
import {
  approvedTips,
  carriedText,
  channelName,
  creditLines,
  differenceWords,
  methodName,
  rateText,
  reportProblem,
  staffName,
  serviceName,
  stampText,
  taxNote,
} from "./zreport-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });

describe("approvedTips", () => {
  it("is the tips a manager let through over the ceiling, when there were any (ADR-261)", () => {
    expect(approvedTips({ tips_approved: { count: 2, amount: inr(14000) } })).toEqual({ count: 2, amount: inr(14000) });
    expect(approvedTips({ tips_approved: { count: 0, amount: inr(0) } })).toBeNull();
  });
  it("is nothing on a report issued before they were counted", () => {
    expect(approvedTips({})).toBeNull();
  });
});

describe("rateText", () => {
  it("writes thousandths of a percent without trailing zeros", () => {
    expect(rateText(2500)).toBe("2.5%");
    expect(rateText(9000)).toBe("9%");
    expect(rateText(8875)).toBe("8.875%");
    expect(rateText(250)).toBe("0.25%");
    expect(rateText(0)).toBe("0%");
  });
});

describe("names", () => {
  it("names channels and methods for a person", () => {
    expect(channelName("qr_table")).toBe("QR at the table");
    expect(channelName("somewhere")).toBe("somewhere");
    expect(methodName("upi")).toBe("UPI");
  });
  it("names staff from the report's own list", () => {
    const names = [{ id: 2, name: "Priya" }];
    expect(staffName(names, 2)).toBe("Priya");
    expect(staffName(names, 9)).toBe("Staff 9");
  });
});

describe("differenceWords", () => {
  it("says short, over or balanced with a tone", () => {
    expect(differenceWords(inr(0))).toEqual({ text: "Balanced", tone: "success" });
    const short = differenceWords(inr(-950));
    expect(short.tone).toBe("danger");
    expect(short.text).toMatch(/^Short .*9\.50$/);
    expect(differenceWords(inr(500)).text).toMatch(/^Over .*5\.00$/);
  });
});

describe("reportProblem", () => {
  it("says why a report could not be read", () => {
    expect(reportProblem(403)).toMatch(/permission/);
    expect(reportProblem(404, 99)).toBe("There is no Z-report 99.");
    expect(reportProblem(undefined)).toMatch(/Can't reach/);
  });
});

describe("a slip's words", () => {
  it("stamps the trading day and the close", () => {
    expect(stampText("2026-10-05", "not a time", "Asia/Kolkata")).toBe("5 Oct 2026");
    // On the venue's clock, the twelve-hour way the status bar says it (ADR-385).
    expect(stampText("2026-10-05", "2026-10-05T18:02:00Z", "Asia/Kolkata")).toBe("5 Oct 2026 · 11:32 PM");
    expect(stampText("2026-10-05", "2026-10-05T18:02:00Z", "UTC")).toBe("5 Oct 2026 · 6:02 PM");
  });
  it("names how an order was served, and leaves another as it came", () => {
    expect(serviceName("dine_in")).toBe("Dine-in");
    expect(serviceName("takeaway")).toBe("Takeaway");
    expect(serviceName("delivery")).toBe("Delivery");
    expect(serviceName("drive_through")).toBe("drive_through");
  });
});

describe("the tax card's note", () => {
  it("says the rows are after credit notes, only when there were any", () => {
    expect(taxNote(0)).toBeNull();
    expect(taxNote(1)).toBe("After 1 credit note: the tax invoiced, less the tax credited.");
    expect(taxNote(3)).toBe("After 3 credit notes: the tax invoiced, less the tax credited.");
  });
});

describe("creditLines", () => {
  const sales = { total: inr(120000), net_of_credits: inr(101000) };
  it("is the credit notes taken off the total, and what is left, when there were any", () => {
    expect(creditLines({ sales, credit_notes: { count: 2, amount: inr(19000) } })).toEqual({
      label: "Less credit notes (2)",
      less: inr(-19000),
      net: inr(101000),
    });
  });
  it("is nothing on a shift with none", () => {
    expect(creditLines({ sales, credit_notes: { count: 0, amount: inr(0) } })).toBeNull();
  });
});

describe("carriedText", () => {
  it("says how many orders a close carried and what was still to pay (ADR-384)", () => {
    expect(carriedText({ count: 3, amount: inr(65000) })).toBe("3 · ₹650.00 to pay");
    expect(carriedText({ count: 1, amount: inr(0) })).toBe("1 · nothing to pay");
  });
  it("says none when nothing was open", () => {
    expect(carriedText({ count: 0, amount: inr(0) })).toBe("None");
  });
  it("is nothing on a report issued before they were counted", () => {
    expect(carriedText(undefined)).toBeNull();
  });
});

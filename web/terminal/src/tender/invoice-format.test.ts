import { describe, expect, it } from "vitest";
import { invoiceTitle, issuedAtText, rateText, registrationText } from "./invoice-format";

describe("rateText", () => {
  const cases: [number, string][] = [
    [5000, "5%"],
    [2500, "2.5%"],
    [18000, "18%"],
    [8875, "8.875%"],
    [250, "0.25%"],
    [0, "0%"],
  ];
  it.each(cases)("%i is %s", (rate, want) => {
    expect(rateText(rate)).toBe(want);
  });
});

describe("invoiceTitle", () => {
  it("names the document the way its tax regime does", () => {
    expect(invoiceTitle("gst_in")).toBe("Tax invoice");
    expect(invoiceTitle("vat_uk")).toBe("VAT receipt");
    expect(invoiceTitle("none")).toBe("Invoice");
    // The composition scheme issues a bill of supply, not a tax invoice (ADR-401).
    expect(invoiceTitle("gst_in_composition")).toBe("Bill of supply");
  });
});

describe("registrationText", () => {
  it("calls the number what the printed receipt does", () => {
    expect(registrationText("gst_in")).toBe("GSTIN");
    expect(registrationText("gst_in_composition")).toBe("GSTIN");
    expect(registrationText("vat_uk")).toBe("VAT No.");
  });
});

describe("issuedAtText", () => {
  it("renders in the venue's time zone", () => {
    expect(issuedAtText("2026-09-18T04:30:00Z", "Asia/Kolkata", "en-GB")).toBe("18/09/2026, 10:00");
  });
  it("falls back to the device's zone for a zone it does not know", () => {
    expect(issuedAtText("2026-09-18T04:30:00Z", "Mars/Olympus", "en-GB")).toMatch(/^18\/09\/2026, \d\d:\d\d$/);
  });
});

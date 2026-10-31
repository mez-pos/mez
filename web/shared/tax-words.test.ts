import { describe, expect, it } from "vitest";
import { compositionDeclaration, declarationOf, registrationLabel } from "./tax-words";

describe("registrationLabel", () => {
  it("names the registration number each profile uses", () => {
    expect(registrationLabel("gst_in")).toBe("GSTIN");
    expect(registrationLabel("gst_in_composition")).toBe("GSTIN");
    expect(registrationLabel("vat_uk")).toBe("VAT number");
    expect(registrationLabel("none")).toBeNull();
    expect(registrationLabel("sales_tax_us")).toBeNull();
    expect(registrationLabel("")).toBeNull();
  });
});

describe("declarationOf (ADR-401)", () => {
  it("is the rule's own words, under the composition scheme only", () => {
    expect(compositionDeclaration).toBe("Composition taxable person, not eligible to collect tax on supplies");
    expect(declarationOf("gst_in_composition")).toBe(compositionDeclaration);
    for (const profile of ["gst_in", "none", "vat_uk", "sales_tax_us", ""]) {
      expect(declarationOf(profile)).toBeNull();
    }
  });
});

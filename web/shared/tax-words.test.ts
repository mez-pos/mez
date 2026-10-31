import { describe, expect, it } from "vitest";
import { registrationLabel } from "./tax-words";

describe("registrationLabel", () => {
  it("names the registration number each profile uses", () => {
    expect(registrationLabel("gst_in")).toBe("GSTIN");
    expect(registrationLabel("vat_uk")).toBe("VAT number");
    expect(registrationLabel("none")).toBeNull();
    expect(registrationLabel("sales_tax_us")).toBeNull();
    expect(registrationLabel("")).toBeNull();
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { NumberingResponse } from "../../../shared/api-types";
import {
  MONTHS,
  numberingChanged,
  numberingFormFrom,
  numberingProblem,
  numberingProblemOf,
  parseStep,
  previewKey,
  stepRange,
  stepText,
  toNumberingInput,
  withSeries,
} from "./numbering-logic";

const saved: NumberingResponse = {
  invoice_series: "INV",
  credit_series: "CN",
  invoice_prefix: "INV",
  credit_prefix: "CN",
  template: "{prefix}/{fy:short}/{seq:06d}",
  financial_year_start_month: 4,
  cash_rounding_step: 50,
  currency: "INR",
  next_invoice_no: "INV/26-27/000001",
  next_credit_no: "CN/26-27/000001",
  number_limit: 16,
  fy_locked: false,
  max_rounding_step: 100,
};

describe("the months", () => {
  it("are twelve, January first", () => {
    expect(MONTHS).toHaveLength(12);
    expect(MONTHS[0]).toBe("January");
    expect(MONTHS[3]).toBe("April");
  });
});

describe("the rounding step", () => {
  it("is shown in major units", () => {
    expect(stepText(50, 100)).toBe("0.50");
    expect(stepText(1, 100)).toBe("0.01");
    expect(stepText(100, 100)).toBe("1.00");
    expect(stepText(5, 100)).toBe("0.05");
    expect(stepText(1, 1)).toBe("1"); // a currency with no minor unit
    expect(stepText(10, 1000)).toBe("0.010"); // three decimals
  });
  it("is read from what is typed, never through a float", () => {
    expect(parseStep("0.50", 100)).toBe(50);
    expect(parseStep("0.5", 100)).toBe(50);
    expect(parseStep("1", 100)).toBe(100);
    expect(parseStep(" 0.05 ", 100)).toBe(5);
    expect(parseStep("0.01", 100)).toBe(1);
    expect(parseStep("1", 1)).toBe(1);
    expect(parseStep("0.001", 1000)).toBe(1);
  });
  it("is refused when it rounds by more than a whole unit, by less than the smallest, or is no number", () => {
    for (const bad of ["", "0", "0.00", "1.01", "2", "0.505", "-1", "abc", "0,5", ".5", "1e2"]) {
      expect(parseStep(bad, 100), bad).toBeNull();
    }
    expect(parseStep("0.5", 1)).toBeNull(); // no decimals where there is no minor unit
    expect(parseStep("2", 1)).toBeNull();
  });
  it("reads back what it wrote", () => {
    for (const minor of [1, 5, 10, 25, 50, 99, 100]) expect(parseStep(stepText(minor, 100), 100)).toBe(minor);
  });
  it("names its range", () => {
    expect(stepRange(100)).toBe("from 0.01 to 1.00");
    expect(stepRange(1)).toBe("from 1 to 1");
  });
});

describe("the numbering form", () => {
  it("is what the server holds", () => {
    expect(numberingFormFrom(saved)).toEqual({
      invoiceSeries: "INV",
      creditSeries: "CN",
      invoicePrefix: "INV",
      creditPrefix: "CN",
      template: "{prefix}/{fy:short}/{seq:06d}",
      month: 4,
      step: "0.50",
    });
  });

  it("sends codes in capitals, trimmed, and the step in minor units", () => {
    const f = { ...numberingFormFrom(saved), invoiceSeries: " nf ", creditSeries: "cr", invoicePrefix: " ", step: "1" };
    expect(toNumberingInput(f, 100)).toEqual({
      invoice_series: "NF",
      credit_series: "CR",
      invoice_prefix: "",
      credit_prefix: "CN",
      template: "{prefix}/{fy:short}/{seq:06d}",
      financial_year_start_month: 4,
      cash_rounding_step: 100,
    });
  });

  it("is changed by a real difference, not by case or stray spaces", () => {
    const f = numberingFormFrom(saved);
    expect(numberingChanged(f, f)).toBe(false);
    expect(numberingChanged({ ...f, invoiceSeries: "inv " }, f)).toBe(false);
    expect(numberingChanged({ ...f, invoiceSeries: "NF" }, f)).toBe(true);
    expect(numberingChanged({ ...f, month: 1 }, f)).toBe(true);
    expect(numberingChanged({ ...f, step: "0.05" }, f)).toBe(true);
    expect(numberingChanged({ ...f, template: "{prefix}-{fy}-{seq}" }, f)).toBe(true);
    expect(numberingChanged({ ...f, creditPrefix: "CR" }, f)).toBe(true);
  });

  it("refuses a step it cannot read, with the range", () => {
    const f = numberingFormFrom(saved);
    expect(numberingProblem(f, 100)).toBeNull();
    expect(numberingProblem({ ...f, step: "5" }, 100)).toEqual({
      field: "cash_rounding_step",
      message: "Enter a step from 0.01 to 1.00 (the smallest rounds nothing).",
    });
  });

  it("puts the server's words under the field they are about", () => {
    expect(numberingProblemOf(new ApiError("invalid_setting", "No year.", 422, "template")).field).toBe("template");
    expect(numberingProblemOf(new ApiError("numbering_locked", "Locked.", 409, "financial_year_start_month")).field).toBe("financial_year_start_month");
    expect(numberingProblemOf(new ApiError("number_taken", "Taken.", 409, "invoice_prefix")).field).toBe("invoice_prefix");
    expect(numberingProblemOf(new ApiError("invalid_setting", "x", 422, "currency")).field).toBeNull();
  });
});

describe("typing a series code", () => {
  const f = numberingFormFrom(saved);
  it("raises it to capitals", () => {
    expect(withSeries(f, "invoice", "nf").invoiceSeries).toBe("NF");
  });
  it("carries a prefix that was following it", () => {
    const next = withSeries(f, "invoice", "nf");
    expect(next.invoicePrefix).toBe("NF");
    expect(next.creditPrefix).toBe("CN"); // the other series is untouched
    expect(withSeries(f, "credit", "cr")).toMatchObject({ creditSeries: "CR", creditPrefix: "CR", invoicePrefix: "INV" });
  });
  it("leaves a prefix the owner wrote", () => {
    const own = { ...f, invoicePrefix: "TAX" };
    expect(withSeries(own, "invoice", "nf")).toMatchObject({ invoiceSeries: "NF", invoicePrefix: "TAX" });
  });
  it("follows through an empty prefix too, as the server does", () => {
    expect(withSeries({ ...f, invoicePrefix: "" }, "invoice", "nf").invoicePrefix).toBe("NF");
  });
  it("keeps following while typing, a letter at a time", () => {
    let g = f;
    for (const typed of ["N", "NF", "NFC"]) g = withSeries(g, "invoice", typed);
    expect(g).toMatchObject({ invoiceSeries: "NFC", invoicePrefix: "NFC" });
  });
});

describe("the preview key", () => {
  it("is the same for the same draft, and differs when anything sent differs", () => {
    const f = numberingFormFrom(saved);
    expect(previewKey(f, 100)).toBe(previewKey({ ...f }, 100));
    expect(previewKey(f, 100)).toBe(previewKey({ ...f, invoiceSeries: "inv " }, 100));
    expect(previewKey(f, 100)).not.toBe(previewKey({ ...f, month: 1 }, 100));
    expect(previewKey(f, 100)).not.toBe(previewKey({ ...f, step: "1" }, 100));
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { TaxClassResponse, TaxSettingsResponse } from "../../../shared/api-types";
import {
  PROFILES,
  classFormFrom,
  classProblem,
  classProblemOf,
  classRateText,
  emptyClassForm,
  menuItemPath,
  namedItems,
  parsePercent,
  percentInput,
  percentText,
  showWithoutHSN,
  taxChanged,
  taxFormFrom,
  taxProblem,
  taxProblemOf,
  toClassInput,
  toTaxInput,
  type TaxForm,
} from "./tax-logic";

const gst: TaxSettingsResponse = {
  profile: "gst_in",
  registration_id: "29ABCDE1234F1Z5",
  pricing: "inclusive",
  state_code: "29",
  jurisdiction: [],
  needs_hsn: true,
  items_without_hsn: [],
  classes: [],
};
const sales: TaxSettingsResponse = {
  ...gst,
  profile: "sales_tax_us",
  registration_id: "",
  pricing: "exclusive",
  state_code: "",
  jurisdiction: [
    { name: "CA", rate: 7250 },
    { name: "SF", rate: 625 },
  ],
  needs_hsn: false,
};

describe("percentages", () => {
  it("read a rate as typed", () => {
    expect(parsePercent("5")).toBe(5000);
    expect(parsePercent("2.5")).toBe(2500);
    expect(parsePercent("7.25")).toBe(7250);
    expect(parsePercent(" 12 % ")).toBe(12000);
    expect(parsePercent("0")).toBe(0);
    expect(parsePercent("100")).toBe(100_000);
    expect(parsePercent("8.875")).toBe(8875);
    expect(parsePercent("0.5")).toBe(500);
  });
  it("refuse what is not a percentage from 0 to 100 with up to three decimals", () => {
    for (const bad of ["", " ", "abc", "-1", "100.001", "101", "1.2345", "1,5", "5%%", ".5", "1e2", "NaN"]) {
      expect(parsePercent(bad), bad).toBeNull();
    }
  });
  it("write a rate back as it reads, never through a float", () => {
    expect(percentInput(5000)).toBe("5");
    expect(percentInput(2500)).toBe("2.5");
    expect(percentInput(8875)).toBe("8.875");
    expect(percentInput(250)).toBe("0.25");
    expect(percentInput(0)).toBe("0");
    expect(percentText(2500)).toBe("2.5%");
    for (const rate of [0, 1, 250, 2500, 5000, 7250, 8875, 100_000]) expect(parsePercent(percentInput(rate))).toBe(rate);
  });
});

describe("the profiles", () => {
  it("are the five the server takes, the composition scheme after GST", () => {
    expect(PROFILES.map((p) => p.value)).toEqual(["none", "gst_in", "gst_in_composition", "vat_uk", "sales_tax_us"]);
  });
});

describe("the tax form", () => {
  it("is what the server holds, rates as percent", () => {
    expect(taxFormFrom(sales)).toEqual({
      profile: "sales_tax_us",
      registrationId: "",
      pricing: "exclusive",
      stateCode: "",
      jurisdiction: [
        { name: "CA", rate: "7.25" },
        { name: "SF", rate: "0.625" },
      ],
    });
    expect(taxFormFrom({ ...gst, profile: "unheard_of" }).profile).toBe("none");
  });

  it("sends only what its profile uses", () => {
    const f = taxFormFrom(gst);
    expect(toTaxInput(f)).toEqual({ profile: "gst_in", registration_id: "29ABCDE1234F1Z5", pricing: "inclusive", state_code: "29", jurisdiction: [] });
    // Moving to none leaves nothing behind.
    expect(toTaxInput({ ...f, profile: "none" })).toEqual({ profile: "none", registration_id: "", pricing: "", state_code: "", jurisdiction: [] });
    // US sales tax is always exclusive, whatever the form held.
    const us = { ...taxFormFrom(sales), pricing: "inclusive" as const, registrationId: "stale", stateCode: "29" };
    expect(toTaxInput(us)).toEqual({
      profile: "sales_tax_us",
      registration_id: "",
      pricing: "",
      state_code: "",
      jurisdiction: [
        { name: "CA", rate: 7250 },
        { name: "SF", rate: 625 },
      ],
    });
    expect(toTaxInput({ ...f, profile: "vat_uk", registrationId: " gb 123 " }).registration_id).toBe("gb 123");
    // The composition scheme keeps the GSTIN and nothing else (ADR-401).
    expect(toTaxInput({ ...f, profile: "gst_in_composition", pricing: "exclusive" })).toEqual({
      profile: "gst_in_composition",
      registration_id: "29ABCDE1234F1Z5",
      pricing: "",
      state_code: "",
      jurisdiction: [],
    });
  });

  it("is read back under the composition scheme", () => {
    expect(taxFormFrom({ ...gst, profile: "gst_in_composition", pricing: "", state_code: "" }).profile).toBe("gst_in_composition");
  });

  it("is changed only by what its profile uses", () => {
    const saved = taxFormFrom(gst);
    expect(taxChanged(saved, saved)).toBe(false);
    expect(taxChanged({ ...saved, registrationId: "29 abcde1234f1z5" }, saved)).toBe(false); // spaces and case
    expect(taxChanged({ ...saved, jurisdiction: [{ name: "x", rate: "1" }] }, saved)).toBe(false); // GST ignores it
    expect(taxChanged({ ...saved, registrationId: "29ABCDE1234F1Z6" }, saved)).toBe(true);
    expect(taxChanged({ ...saved, pricing: "exclusive" }, saved)).toBe(true);
    expect(taxChanged({ ...saved, stateCode: "27" }, saved)).toBe(true);
    expect(taxChanged({ ...saved, profile: "none" }, saved)).toBe(true);
    const none = taxFormFrom({ ...gst, profile: "none", registration_id: "", state_code: "", pricing: "" });
    expect(taxChanged({ ...none, registrationId: "anything" }, none)).toBe(false);
    const comp = { ...saved, profile: "gst_in_composition" as const };
    expect(taxChanged({ ...comp, pricing: "exclusive", stateCode: "27" }, comp)).toBe(false); // it adds and splits nothing
    expect(taxChanged({ ...comp, registrationId: "29ABCDE1234F1Z6" }, comp)).toBe(true);
    expect(taxChanged(comp, saved)).toBe(true);
    const us = taxFormFrom(sales);
    expect(taxChanged({ ...us, jurisdiction: [...us.jurisdiction, { name: "LA", rate: "2.25" }] }, us)).toBe(true);
    expect(taxChanged({ ...us, jurisdiction: [{ name: " CA ", rate: "7.250" }, us.jurisdiction[1]!] }, us)).toBe(false);
  });

  describe("what is refused before asking the server", () => {
    const base: TaxForm = taxFormFrom(gst);
    it("lets a good form through, for every profile", () => {
      expect(taxProblem(base)).toBeNull();
      expect(taxProblem({ ...base, profile: "none", registrationId: "" })).toBeNull();
      expect(taxProblem({ ...base, profile: "vat_uk", registrationId: "GB123456789" })).toBeNull();
      expect(taxProblem(taxFormFrom(sales))).toBeNull();
    });
    it("wants the registration number GST and VAT print", () => {
      expect(taxProblem({ ...base, registrationId: "  " })).toEqual({ field: "registration_id", message: "Enter the venue's GSTIN: invoices print it." });
      expect(taxProblem({ ...base, profile: "vat_uk", registrationId: "" })?.message).toMatch(/VAT number/);
      expect(taxProblem({ ...base, profile: "gst_in_composition", registrationId: " " })).toEqual({
        field: "registration_id",
        message: "Enter the venue's GSTIN: every bill shows it.",
      });
      expect(taxProblem({ ...base, profile: "gst_in_composition" })).toBeNull();
    });
    it("wants a rate under US sales tax, each with a name and a real percentage", () => {
      const us = taxFormFrom(sales);
      expect(taxProblem({ ...us, jurisdiction: [] })?.field).toBe("jurisdiction");
      expect(taxProblem({ ...us, jurisdiction: [{ name: "", rate: "7" }] })?.field).toBe("jurisdiction");
      expect(taxProblem({ ...us, jurisdiction: [{ name: "CA", rate: "seven" }] })?.field).toBe("jurisdiction");
      expect(taxProblem({ ...us, jurisdiction: [{ name: "CA", rate: "101" }] })?.field).toBe("jurisdiction");
    });
    it("does not ask a profile for what it ignores", () => {
      expect(taxProblem({ ...base, profile: "none", registrationId: "", jurisdiction: [{ name: "", rate: "x" }] })).toBeNull();
      expect(taxProblem({ ...taxFormFrom(sales), registrationId: "" })).toBeNull();
    });
  });

  it("puts the server's words under the field they are about", () => {
    expect(taxProblemOf(new ApiError("invalid_setting", "GSTIN is not valid.", 422, "registration_id"))).toEqual({
      field: "registration_id",
      message: "GSTIN is not valid.",
    });
    expect(taxProblemOf(new ApiError("tickets_open", "Tickets are still open.", 409)).field).toBeNull();
    expect(taxProblemOf(new ApiError("invalid_setting", "x", 422, "name")).field).toBeNull();
  });
});

describe("the items a profile still owes", () => {
  it("open in the menu editor", () => {
    expect(menuItemPath(7)).toBe("/menu?item=7");
  });
  it("are named a few at a time", () => {
    const items = Array.from({ length: 11 }, (_, i) => ({ id: i + 1, name: `Item ${i + 1}` }));
    expect(namedItems(items)).toEqual({ shown: items.slice(0, 8), more: 3 });
    expect(namedItems(items.slice(0, 2))).toEqual({ shown: items.slice(0, 2), more: 0 });
    expect(namedItems([])).toEqual({ shown: [], more: 0 });
    expect(namedItems(items, 3).more).toBe(8);
  });
});

describe("when the items without an HSN are listed", () => {
  const owed = { profile: "gst_in", needs_hsn: true, items_without_hsn: [{ id: 1, name: "Cold brew" }] };
  it("is while the form is the profile that owes them", () => {
    expect(showWithoutHSN(owed, "gst_in")).toBe(true);
  });
  it("is not once the form is another profile", () => {
    expect(showWithoutHSN(owed, "sales_tax_us")).toBe(false);
    expect(showWithoutHSN(owed, "none")).toBe(false);
  });
  it("is not when nothing is owed, or the profile prints no HSN", () => {
    expect(showWithoutHSN({ ...owed, items_without_hsn: [] }, "gst_in")).toBe(false);
    expect(showWithoutHSN({ profile: "none", needs_hsn: false, items_without_hsn: [] }, "none")).toBe(false);
  });
});

describe("the tax class form", () => {
  const food: TaxClassResponse = { id: 1, name: "Food", rate: 5000, exempt: false, active: true };
  const water: TaxClassResponse = { id: 2, name: "Water", rate: 0, exempt: true, active: true };

  it("is what the server holds", () => {
    expect(classFormFrom(food)).toEqual({ name: "Food", rate: "5", exempt: false });
    expect(classFormFrom(water)).toEqual({ name: "Water", rate: "", exempt: true });
    expect(emptyClassForm()).toEqual({ name: "", rate: "", exempt: false });
  });
  it("sends a rate, or none at all when exempt", () => {
    expect(toClassInput({ name: " Food ", rate: "2.5", exempt: false })).toEqual({ name: "Food", rate: 2500, exempt: false });
    expect(toClassInput({ name: "Water", rate: "12", exempt: true })).toEqual({ name: "Water", rate: 0, exempt: true });
  });
  it("is refused for no name, a long one, or a rate that is not one", () => {
    expect(classProblem({ name: " ", rate: "5", exempt: false })?.field).toBe("name");
    expect(classProblem({ name: "a".repeat(41), rate: "5", exempt: false })?.field).toBe("name");
    expect(classProblem({ name: "Food", rate: "", exempt: false })?.field).toBe("rate");
    expect(classProblem({ name: "Food", rate: "abc", exempt: false })?.field).toBe("rate");
    expect(classProblem({ name: "Food", rate: "0", exempt: false })).toBeNull();
    expect(classProblem({ name: "Water", rate: "", exempt: true })).toBeNull(); // no rate asked of an exempt class
    expect(classProblem({ name: "a".repeat(40), rate: "5", exempt: false })).toBeNull();
  });
  it("reads a class as its rate or Exempt", () => {
    expect(classRateText(food)).toBe("5%");
    expect(classRateText(water)).toBe("Exempt");
    expect(classRateText({ rate: 2500, exempt: false })).toBe("2.5%");
  });
  it("puts the server's words under name or rate", () => {
    expect(classProblemOf(new ApiError("tax_class_name_taken", "A tax class with that name already exists.", 409, "name")).field).toBe("name");
    expect(classProblemOf(new ApiError("invalid_setting", "Enter a rate.", 422, "rate")).field).toBe("rate");
    expect(classProblemOf(new ApiError("tax_class_in_use", "Items still use it.", 409)).field).toBeNull();
  });
});

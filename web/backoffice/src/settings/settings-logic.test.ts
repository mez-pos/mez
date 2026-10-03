import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import {
  DEFAULT_SECTION,
  MAX_LEGAL_NAME,
  MAX_NAME,
  SETTINGS_SECTIONS,
  builtSectionFor,
  problemOf,
  timezoneNames,
  toVenueInput,
  venueChanged,
  venueFormFrom,
  venueProblem,
} from "./settings-logic";

const venue = { name: "Cafe", legal_name: "Cafe Pvt Ltd", currency: "INR", timezone: "Asia/Kolkata" };

describe("the sections", () => {
  it("have distinct ids and paths under /settings", () => {
    expect(new Set(SETTINGS_SECTIONS.map((s) => s.id)).size).toBe(SETTINGS_SECTIONS.length);
    expect(new Set(SETTINGS_SECTIONS.map((s) => s.path)).size).toBe(SETTINGS_SECTIONS.length);
    for (const s of SETTINGS_SECTIONS) expect(s.path).toBe(`/settings/${s.id}`);
  });
  it("start at the venue, which is built", () => {
    expect(DEFAULT_SECTION.id).toBe("venue");
    expect(DEFAULT_SECTION.built).toBe(true);
  });
  it("name a built section by its path, and no other", () => {
    expect(builtSectionFor("/settings/venue")?.id).toBe("venue");
    expect(builtSectionFor("/settings/tax")?.id).toBe("tax");
    expect(builtSectionFor("/settings/shifts")).toBeNull(); // not built yet
    expect(builtSectionFor("/settings/nothing")).toBeNull();
    expect(builtSectionFor("/settings")).toBeNull();
  });
});

describe("the venue form", () => {
  it("is what the server holds", () => {
    expect(venueFormFrom(venue)).toEqual({ name: "Cafe", legalName: "Cafe Pvt Ltd", timezone: "Asia/Kolkata" });
  });
  it("sends trimmed text in the server's field names", () => {
    expect(toVenueInput({ name: "  Blue Door ", legalName: " Blue Door LLP ", timezone: " Europe/London " })).toEqual({
      name: "Blue Door",
      legal_name: "Blue Door LLP",
      timezone: "Europe/London",
    });
  });
  it("is changed only by a real difference, not by stray spaces", () => {
    const saved = venueFormFrom(venue);
    expect(venueChanged(saved, saved)).toBe(false);
    expect(venueChanged({ ...saved, name: "  Cafe  " }, saved)).toBe(false);
    expect(venueChanged({ ...saved, name: "Cafe 2" }, saved)).toBe(true);
    expect(venueChanged({ ...saved, legalName: "" }, saved)).toBe(true);
    expect(venueChanged({ ...saved, timezone: "Europe/London" }, saved)).toBe(true);
  });
});

describe("what is refused before asking the server", () => {
  const ok = venueFormFrom(venue);
  it("lets a good form through, with no legal name", () => {
    expect(venueProblem(ok)).toBeNull();
    expect(venueProblem({ ...ok, legalName: "" })).toBeNull();
  });
  it("refuses no name, and a name that is only spaces", () => {
    expect(venueProblem({ ...ok, name: "" })).toEqual({ field: "name", message: "Give the venue a name." });
    expect(venueProblem({ ...ok, name: "   " })?.field).toBe("name");
  });
  it("counts characters, not bytes", () => {
    expect(venueProblem({ ...ok, name: "a".repeat(MAX_NAME) })).toBeNull();
    expect(venueProblem({ ...ok, name: "a".repeat(MAX_NAME + 1) })?.field).toBe("name");
    // 60 Devanagari letters are 180 bytes and 60 characters.
    expect(venueProblem({ ...ok, name: "स".repeat(MAX_NAME) })).toBeNull();
    // An emoji is two UTF-16 units and one character.
    expect(venueProblem({ ...ok, name: "\u{1F600}".repeat(MAX_NAME) })).toBeNull();
    expect(venueProblem({ ...ok, legalName: "a".repeat(MAX_LEGAL_NAME + 1) })?.field).toBe("legal_name");
  });
  it("refuses no timezone", () => {
    expect(venueProblem({ ...ok, timezone: " " })).toEqual({ field: "timezone", message: "Choose a timezone, like Asia/Kolkata." });
  });
  it("reports the first problem, name before the rest", () => {
    expect(venueProblem({ name: "", legalName: "a".repeat(MAX_LEGAL_NAME + 1), timezone: "" })?.field).toBe("name");
  });
});

describe("a failed request", () => {
  it("puts the server's words under the field they are about", () => {
    expect(problemOf(new ApiError("invalid_setting", "That is not a timezone name.", 422, "timezone"))).toEqual({
      field: "timezone",
      message: "That is not a timezone name.",
    });
    expect(problemOf(new ApiError("invalid_setting", "Give the venue a name.", 422, "name")).field).toBe("name");
    expect(problemOf(new ApiError("invalid_setting", "Too long.", 422, "legal_name")).field).toBe("legal_name");
  });
  it("puts a refusal about no field of this form in the footer", () => {
    expect(problemOf(new ApiError("venue_not_found", "No such venue.", 404))).toEqual({ field: null, message: "No such venue." });
    expect(problemOf(new ApiError("invalid_setting", "x", 422, "currency")).field).toBeNull();
  });
  it("says what a missing permission means", () => {
    expect(problemOf(new ApiError("forbidden", "x", 403, "settings.edit")).message).toMatch(/change the venue's settings/);
  });
  it("says when the server cannot be reached, and keeps the words plain for anything else", () => {
    expect(problemOf(new ApiError("network", "fetch failed")).message).toMatch(/Can't reach the mez server/);
    expect(problemOf(new ApiError("timeout", "slow")).message).toMatch(/Can't reach the mez server/);
    expect(problemOf(new Error("boom"))).toEqual({ field: null, message: "Something went wrong. Try again." });
  });
});

describe("the timezone suggestions", () => {
  it("are the browser's list when it has one", () => {
    expect(timezoneNames({ supportedValuesOf: () => ["Asia/Kolkata", "UTC"] })).toEqual(["Asia/Kolkata", "UTC"]);
  });
  it("fall back to a short list where it has none, or it throws", () => {
    for (const intl of [{}, { supportedValuesOf: () => [] }, { supportedValuesOf: () => { throw new RangeError("no"); } }]) {
      const names = timezoneNames(intl);
      expect(names).toContain("Asia/Kolkata");
      expect(names.length).toBeGreaterThan(5);
    }
  });
});

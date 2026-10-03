import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { PrinterResponse } from "../../../shared/api-types";
import { agoText, connectionText, emptyForm, formFrom, problemOf, stationsText, toSettings } from "./printers-logic";

const printer = (extra: Partial<PrinterResponse> = {}): PrinterResponse => ({
  id: 1,
  name: "Counter",
  transport: "tcp",
  address: "10.0.0.5",
  paper_mm: 80,
  active: true,
  is_receipt: false,
  stations: [],
  ...extra,
});

describe("connectionText", () => {
  const cases: [Partial<PrinterResponse>, string][] = [
    [{ address: "10.0.0.5" }, "10.0.0.5:9100"],
    [{ address: "10.0.0.5:9101" }, "10.0.0.5:9101"],
    [{ address: "printer.local" }, "printer.local:9100"],
    [{ address: "fe80::1" }, "[fe80::1]:9100"],
    [{ address: "[fe80::1]:9101" }, "[fe80::1]:9101"],
    [{ transport: "windows", address: "EPSON TM-T82" }, "USB: EPSON TM-T82"],
  ];
  it.each(cases)("%o is %s", (extra, want) => {
    expect(connectionText(printer(extra))).toBe(want);
  });
});

describe("stationsText", () => {
  const stations = [
    { code: "kitchen", name: "Kitchen" },
    { code: "bar", name: "Bar" },
  ];
  it("names the stations, in the printer's order", () => {
    expect(stationsText(printer({ stations: ["bar", "kitchen"] }), stations)).toBe("Bar, Kitchen");
    expect(stationsText(printer(), stations)).toBe("No stations");
  });
});

describe("agoText", () => {
  const now = new Date("2026-09-25T10:00:00Z");
  it("says how long ago", () => {
    expect(agoText("2026-09-25T09:59:30Z", now)).toBe("just now");
    expect(agoText("2026-09-25T09:48:00Z", now)).toBe("12 min ago");
    expect(agoText("2026-09-25T07:00:00Z", now)).toBe("3 h ago");
  });
});

describe("the form", () => {
  it("round-trips a printer, trimmed", () => {
    const f = { ...formFrom(printer({ paper_mm: 58 })), name: "  Front ", address: " 10.0.0.6 " };
    expect(toSettings(f)).toEqual({ name: "Front", transport: "tcp", address: "10.0.0.6", paper_mm: 58 });
    expect(toSettings(emptyForm())).toEqual({ name: "", transport: "tcp", address: "", paper_mm: 80 });
  });
});

describe("problemOf", () => {
  it("puts a refusal under its field", () => {
    const err = new ApiError("invalid_printer", "Choose 80mm or 58mm.", 422, "paper_mm");
    expect(problemOf(err)).toEqual({ field: "paper_mm", message: "Choose 80mm or 58mm." });
  });
  it("keeps other refusals over the form", () => {
    expect(problemOf(new ApiError("printer_retired", "That printer is retired.", 409)).field).toBeNull();
    expect(problemOf(new ApiError("network", "x")).message).toMatch(/server/);
    expect(problemOf(new ApiError("forbidden", "x", 403)).message).toMatch(/settings/);
    expect(problemOf(new Error("x")).field).toBeNull();
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { PrinterResponse } from "../../../shared/api-types";
import {
  addedPrinterId,
  agoText,
  connectionText,
  defaultPrints,
  emptyForm,
  formFrom,
  printersSummary,
  printsOptions,
  printsText,
  problemOf,
  toSettings,
} from "./printers-logic";

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

describe("printersSummary", () => {
  const cases: [boolean[], string][] = [
    [[], "None yet"],
    [[true], "1 printer"],
    [[true, true], "2 printers"],
    [[true, false, false], "3 printers, 2 retired"],
  ];
  it.each(cases)("%o is %s", (actives, want) => {
    expect(printersSummary(actives.map((active) => ({ active })))).toBe(want);
  });
});

describe("agoText", () => {
  const now = new Date("2026-09-25T10:00:00Z");
  it("says how long ago", () => {
    expect(agoText("2026-09-25T09:59:30Z", now, "Asia/Kolkata")).toBe("just now");
    expect(agoText("2026-09-25T09:48:00Z", now, "Asia/Kolkata")).toBe("12 min ago");
    expect(agoText("2026-09-25T07:00:00Z", now, "Asia/Kolkata")).toBe("3 h ago");
    // Older than a day: the venue's date, not the browser's locale's (BO-4).
    expect(agoText("2026-09-22T20:00:00Z", now, "Asia/Kolkata")).toBe("23 Sep 2026");
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

describe("printsText", () => {
  const stations = [
    { code: "kitchen", name: "Kitchen" },
    { code: "bar", name: "Bar" },
  ];
  const cases: [Partial<PrinterResponse>, string][] = [
    [{}, "Nothing yet"],
    [{ is_receipt: true }, "Receipts"],
    [{ stations: ["bar"] }, "Bar tickets"],
    [{ is_receipt: true, stations: ["bar", "kitchen"] }, "Receipts, Bar tickets, Kitchen tickets"],
    [{ stations: ["grill"] }, "grill tickets"],
  ];
  it.each(cases)("%o prints %s", (extra, want) => {
    expect(printsText(printer(extra), stations)).toBe(want);
  });
});

describe("what a new printer prints (BO-15)", () => {
  const stations = [
    { code: "kitchen", name: "Kitchen" },
    { code: "bar", name: "Bar", printer_id: 2 },
  ];

  it("offers receipts, each station's tickets, and deciding later", () => {
    expect(printsOptions(stations).map((o) => o.label)).toEqual(["Receipts", "Kitchen tickets", "Bar tickets", "Decide later"]);
  });

  it("starts on what nothing prints yet: receipts first, then a station with no printer, else later", () => {
    expect(defaultPrints({ printers: [], stations })).toBe("receipts");
    expect(defaultPrints({ printers: [printer({ id: 1, is_receipt: true }), printer({ id: 2 })], stations })).toBe("station:kitchen");
    expect(
      defaultPrints({ printers: [printer({ id: 1, is_receipt: true }), printer({ id: 3 })], stations: [{ code: "bar", name: "Bar", printer_id: 3 }] }),
    ).toBe("later");
  });

  it("counts a retired printer as printing nothing", () => {
    expect(defaultPrints({ printers: [printer({ id: 1, is_receipt: true, active: false })], stations: [] })).toBe("receipts");
    expect(defaultPrints({ printers: [printer({ id: 1, is_receipt: true }), printer({ id: 2, active: false })], stations })).toBe("station:kitchen");
  });

  it("finds the printer an add made", () => {
    expect(addedPrinterId({ printers: [printer({ id: 1 })], stations: [] }, { printers: [printer({ id: 1 }), printer({ id: 7 })], stations: [] })).toBe(7);
    expect(addedPrinterId(undefined, { printers: [printer({ id: 4 })], stations: [] })).toBe(4);
    expect(addedPrinterId({ printers: [printer({ id: 1 })], stations: [] }, { printers: [printer({ id: 1 })], stations: [] })).toBeNull();
  });
});

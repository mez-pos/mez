import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { GuestReceiptResponse } from "../../shared/api-types";
import {
  BILL_REMEMBER_MS,
  billIdOf,
  billStorageKey,
  issuedText,
  offersReceipt,
  paidRows,
  parseKeptBill,
  problemWords,
  receiptProblem,
  receiptRows,
  registrationName,
} from "./receipt-logic";

const inr = (minor: number) => ({ minor, currency: "INR" });
const now = 1_800_000_000_000;

const receipt: GuestReceiptResponse = {
  bill: "ord_7K3M9QX2",
  invoice_no: "INV/26-27/000041",
  issued_at: "2026-10-06T07:30:00Z",
  time_zone: "Asia/Kolkata",
  seller: { name: "Demo Cafe", legal_name: "Demo Foods LLP", registration: "29ABCDE1234F1Z5" },
  lines: [
    { name: "Flat white", qty: 2, choices: ["Oat"], amount: inr(44000) },
    { name: "Brownie", qty: 1, choices: [], amount: inr(12000) },
  ],
  items: inr(56000),
  discounts: inr(0),
  taxes: [
    { name: "CGST", rate: 2500, amount: inr(1400) },
    { name: "SGST", rate: 2500, amount: inr(1400) },
  ],
  tax_included: false,
  total: inr(58800),
  payments: [
    { method: "cash", amount: inr(20000) },
    { method: "upi", amount: inr(38800) },
  ],
  credited: inr(0),
  refunded: inr(0),
};

describe("the bill the phone remembers", () => {
  const stored = (change: object = {}) => JSON.stringify({ id: "ord_7K3M9QX2", at: now - 1000, ...change });
  it("is kept apart for each table", () => {
    expect(billStorageKey("abc")).not.toBe(billStorageKey("abd"));
  });
  it("is read back", () => {
    expect(parseKeptBill(stored(), now)).toEqual({ id: "ord_7K3M9QX2", at: now - 1000 });
  });
  it("is forgotten when the venue would no longer give its receipt", () => {
    expect(parseKeptBill(stored({ at: now - BILL_REMEMBER_MS - 1 }), now)).toBeNull();
    expect(parseKeptBill(stored({ at: now + BILL_REMEMBER_MS + 1 }), now)).toBeNull();
    expect(parseKeptBill(stored({ at: now - BILL_REMEMBER_MS }), now)).not.toBeNull();
  });
  it.each([
    ["nothing", null],
    ["not JSON", "bill"],
    ["not an object", "7"],
    ["an id that is not a bill's", stored({ id: "../../api/v1/orders" })],
    ["a charge's id", stored({ id: "chg_7K3M9QX2" })],
    ["no time", stored({ at: "now" })],
  ])("is nothing for %s", (_name, text) => {
    expect(parseKeptBill(text, now)).toBeNull();
  });
  it("takes an id only if it is a bill's", () => {
    expect(billIdOf("ord_7K3M9QX2")).toBe("ord_7K3M9QX2");
    expect(billIdOf(undefined)).toBeNull();
    expect(billIdOf("")).toBeNull();
    expect(billIdOf("ord_7k3m")).toBeNull();
  });
});

describe("offersReceipt", () => {
  const kept = { id: "ord_7K3M9QX2", at: now };
  it("offers it once the bill is paid, or gone", () => {
    expect(offersReceipt("paid", kept)).toBe(true);
    expect(offersReceipt("none", kept)).toBe(true);
  });
  it("offers none while something is owed, or with no bill remembered", () => {
    expect(offersReceipt("due", kept)).toBe(false);
    expect(offersReceipt("part_paid", kept)).toBe(false);
    expect(offersReceipt("paid", null)).toBe(false);
  });
});

describe("the receipt in words", () => {
  it("calls the seller's number what the invoice's profile calls it (GST-15)", () => {
    expect(registrationName({ tax_profile: "gst_in" })).toBe("GSTIN");
    expect(registrationName({ tax_profile: "vat_uk" })).toBe("VAT number");
    expect(registrationName({})).toBe("Tax registration");
    expect(registrationName({ tax_profile: "sales_tax_us" })).toBe("Tax registration");
  });

  it("says when it was issued in the venue's time, whatever the phone's", () => {
    expect(issuedText(receipt)).toBe("Tue, 6 Oct 2026 · 1:00 PM");
    expect(issuedText({ issued_at: "not a time", time_zone: "Asia/Kolkata" })).toBe("");
  });
  it("lists what the dishes come to and each tax, as the bill does", () => {
    expect(receiptRows(receipt)).toEqual([
      { label: "Items (3)", amount: inr(56000) },
      { label: "CGST (2.5%)", amount: inr(1400) },
      { label: "SGST (2.5%)", amount: inr(1400) },
    ]);
  });
  it("says tax already in the prices as included, and shows a discount", () => {
    const rows = receiptRows({ ...receipt, tax_included: true, discounts: inr(5000) });
    expect(rows[1]).toEqual({ label: "Discount", amount: inr(5000), minus: true });
    expect(rows[2]).toEqual({ label: "Includes CGST (2.5%)", amount: inr(1400), within: true });
  });
  it("says how it was paid, by method", () => {
    expect(paidRows(receipt)).toEqual([
      { label: "Paid by cash", amount: inr(20000) },
      { label: "Paid by UPI", amount: inr(38800) },
    ]);
  });
  it("says what has come back since, only when something has", () => {
    const rows = paidRows({ ...receipt, credited: inr(12000), refunded: inr(12000) });
    expect(rows.slice(2)).toEqual([
      { label: "Taken off since, by credit note", amount: inr(12000), minus: true },
      { label: "Refunded", amount: inr(12000), minus: true },
    ]);
  });
});

describe("a receipt that could not be read", () => {
  it("is none yet when the venue has none to give", () => {
    expect(receiptProblem(new ApiError("receipt_not_found", "x", 404))).toBe("none_yet");
    expect(problemWords("none_yet")).toContain("once it is paid in full");
  });
  it("is unreachable when the venue cannot be reached", () => {
    expect(receiptProblem(new ApiError("network", "offline"))).toBe("unreachable");
    expect(receiptProblem(new ApiError("internal", "x", 503))).toBe("unreachable");
  });
  it("is failed for anything else, and never shows the server's insides", () => {
    expect(receiptProblem(new ApiError("internal", "pq: seq 4", 500))).toBe("failed");
    expect(receiptProblem(new Error("boom"))).toBe("failed");
    expect(problemWords("failed")).not.toMatch(/seq|boom/);
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { PaymentSettingsResponse } from "../../../shared/api-types";
import {
  payeeShown,
  paymentsChanged,
  paymentsFormFrom,
  paymentsProblem,
  paymentsProblemOf,
  paymentsStanding,
  toPaymentsInput,
  type PaymentsForm,
} from "./payments-logic";

const stored: PaymentSettingsResponse = { upi_address: "cafe@okbank", upi_payee: "", venue_name: "Demo Cafe", currency: "INR", max_payee: 60 };
const form = (change: Partial<PaymentsForm> = {}): PaymentsForm => ({ address: "cafe@okbank", payee: "", ...change });

describe("the form and what is sent", () => {
  it("reads the server's settings and sends them back trimmed", () => {
    expect(paymentsFormFrom(stored)).toEqual(form());
    expect(toPaymentsInput(form({ address: "  cafe@okbank ", payee: " Filter Coffee " }))).toEqual({ upi_address: "cafe@okbank", upi_payee: "Filter Coffee" });
  });
  it("is unchanged by spaces alone", () => {
    expect(paymentsChanged(form({ address: " cafe@okbank " }), form())).toBe(false);
    expect(paymentsChanged(form({ payee: "Cafe" }), form())).toBe(true);
    expect(paymentsChanged(form({ address: "" }), form())).toBe(true);
  });
});

describe("paymentsProblem", () => {
  it.each(["cafe@okbank", "filter.coffee-house_1@ybl", "9876543210@upi", "", "   "])("takes %j", (address) => {
    expect(paymentsProblem(form({ address }), 60)).toBeNull();
  });
  it.each(["cafe", "@okbank", "cafe@", "cafe@1bank", "cafe @okbank", "cafe@ok&am=1", "upi://pay?pa=cafe@okbank", "9876543210"])("refuses %j", (address) => {
    expect(paymentsProblem(form({ address }), 60)?.field).toBe("upi_address");
  });
  it("holds the payee to the server's length, by characters", () => {
    expect(paymentsProblem(form({ payee: "é".repeat(60) }), 60)).toBeNull();
    expect(paymentsProblem(form({ payee: "é".repeat(61) }), 60)).toEqual({ field: "upi_payee", message: "Keep the name to 60 characters." });
  });
});

describe("paymentsProblemOf", () => {
  it("puts the server's refusal under its field", () => {
    expect(paymentsProblemOf(new ApiError("invalid_setting", "Enter a UPI address like cafe@okbank.", 422, "upi_address"))).toEqual({
      field: "upi_address",
      message: "Enter a UPI address like cafe@okbank.",
    });
  });
  it("puts anything else in the footer", () => {
    expect(paymentsProblemOf(new ApiError("network", "offline")).field).toBeNull();
    expect(paymentsProblemOf(new ApiError("invalid_setting", "x", 422, "some_other_key")).field).toBeNull();
  });
});

describe("payeeShown and paymentsStanding", () => {
  it("is the name typed, or the venue's", () => {
    expect(payeeShown(form(), "Demo Cafe")).toBe("Demo Cafe");
    expect(payeeShown(form({ payee: " Filter Coffee " }), "Demo Cafe")).toBe("Filter Coffee");
  });
  it("says what a guest's bill does, from what is saved and not what is typed", () => {
    expect(paymentsStanding(form(), "INR")).toBe('A guest\'s bill offers "Pay by UPI" to cafe@okbank.');
    expect(paymentsStanding(form({ address: "" }), "INR")).toContain("offers no UPI");
    expect(paymentsStanding(form(), "USD")).toContain("rupees only");
  });
});

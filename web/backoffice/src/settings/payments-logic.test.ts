import { describe, expect, it } from "vitest";
import { ApiError } from "../../../shared/api";
import type { OnlinePaymentsResponse, PaymentSettingsResponse } from "../../../shared/api-types";
import {
  PAY_MODES,
  gatewayName,
  modeWords,
  onlineWords,
  payeeShown,
  paymentsChanged,
  paymentsFormFrom,
  paymentsProblem,
  paymentsProblemOf,
  paymentsStanding,
  toPaymentsInput,
  type PaymentsForm,
} from "./payments-logic";

const stored: PaymentSettingsResponse = {
  upi_address: "cafe@okbank",
  upi_payee: "",
  venue_name: "Demo Cafe",
  currency: "INR",
  max_payee: 60,
  mode: "pay_at_table",
  can_charge: false,
};
const form = (change: Partial<PaymentsForm> = {}): PaymentsForm => ({ address: "cafe@okbank", payee: "", mode: "pay_at_table", ...change });

describe("the form and what is sent", () => {
  it("reads the server's settings and sends them back trimmed", () => {
    expect(paymentsFormFrom(stored)).toEqual(form());
    expect(toPaymentsInput(form({ address: "  cafe@okbank ", payee: " Filter Coffee " }))).toEqual({
      upi_address: "cafe@okbank",
      upi_payee: "Filter Coffee",
      mode: "pay_at_table",
    });
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

describe("when a guest pays", () => {
  it("reads the mode, and takes a server that says none as paid at the table", () => {
    expect(paymentsFormFrom({ ...stored, mode: "prepay" }).mode).toBe("prepay");
    const { mode: _none, ...older } = stored;
    expect(paymentsFormFrom(older).mode).toBe("pay_at_table");
  });
  it("sends the mode with the address, so saving one never loses the other", () => {
    expect(toPaymentsInput(form({ mode: "prepay" }))).toEqual({ upi_address: "cafe@okbank", upi_payee: "", mode: "prepay" });
  });
  it("is a change to save", () => {
    expect(paymentsChanged(form({ mode: "prepay" }), form())).toBe(true);
    expect(paymentsChanged(form({ mode: "prepay" }), form({ mode: "prepay" }))).toBe(false);
  });
  it("offers both, paying on leaving first", () => {
    expect(PAY_MODES.map((m) => m.value)).toEqual(["pay_at_table", "prepay"]);
  });
  it("says what each does to a guest's order", () => {
    expect(modeWords("pay_at_table", true)).toEqual(modeWords("pay_at_table", false));
    expect(modeWords("pay_at_table", true).text).toContain("waits for a member of staff");
    const first = modeWords("prepay", true);
    expect(first.inert).toBe(false);
    expect(first.text).toContain("Nothing reaches the kitchen until it is paid for");
    expect(first.text).toContain("staff do not accept it");
  });
  it("says paying first does nothing where nobody can pay online, and where that is put right", () => {
    const inert = modeWords("prepay", false);
    expect(inert.inert).toBe(true);
    expect(inert.text).toContain("no effect");
    expect(inert.text).toContain("no payment gateway");
    expect(inert.text).toContain("not on this screen");
  });
  it("puts the server's refusal of a mode under the choice", () => {
    expect(paymentsProblemOf(new ApiError("invalid_setting", "Choose when guests pay.", 422, "mode")).field).toBe("mode");
  });
});

describe("paying online, as the owner reads it", () => {
  const when = (at: Date) => `at ${at.toISOString()}`;
  const set = (change: Partial<OnlinePaymentsResponse> = {}): OnlinePaymentsResponse => ({
    gateway: "stripe",
    webhooks: true,
    return_url: "https://cafe.example/order/",
    last_paid_at: "2026-10-06T04:32:00Z",
    ...change,
  });

  it("names a gateway as its owner does, and one it has no name for as configured", () => {
    expect(gatewayName("razorpay")).toBe("Razorpay");
    expect(gatewayName("stripe")).toBe("Stripe");
    expect(gatewayName("cashfree")).toBe("cashfree");
  });
  it("says there is none, and where one is set", () => {
    const none = onlineWords(undefined, when);
    expect(none.tone).toBe("none");
    expect(none.title).toBe("Guests cannot pay online");
    expect(none.lines.join(" ")).toContain("not on this screen");
  });
  it("says which, when it last took a payment, and where guests come back to", () => {
    const w = onlineWords(set(), when);
    expect(w.tone).toBe("good");
    expect(w.title).toBe("Guests can pay online through Stripe");
    expect(w.lines).toEqual([
      "The last payment through it arrived at 2026-10-06T04:32:00.000Z.",
      "Stripe tells this venue as soon as a payment is made.",
      "After paying, a guest is brought back to https://cafe.example/order/.",
    ]);
  });
  it("warns of a gateway nobody has seen take a payment", () => {
    const { last_paid_at: _never, ...never } = set({ gateway: "razorpay" });
    const w = onlineWords(never, when);
    expect(w.tone).toBe("warning");
    expect(w.title).toContain("Razorpay");
    expect(w.lines[0]).toContain("No payment has arrived through it yet");
    expect(w.lines[0]).toContain("pay a small bill yourself");
    // A time that is not one is not shown as one.
    expect(onlineWords(set({ last_paid_at: "soon" }), when).tone).toBe("warning");
  });
  it("says when payments are asked about and not told, and when guests are not brought back", () => {
    const w = onlineWords(set({ webhooks: false, return_url: "" }), when);
    expect(w.lines[1]).toContain("asks Stripe about each payment every few seconds");
    expect(w.lines[2]).toContain("address is not set");
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { ChargeStatus } from "../../shared/api-types";
import { CHARGE_REMEMBER_MS, chargeOpen, chargeWords, checkoutUrlOf, parseStarted, startFailure } from "./charge-logic";

describe("parseStarted", () => {
  const now = 1_800_000_000_000;
  const stored = (v: unknown) => JSON.stringify(v);

  it("reads back the charge the phone started", () => {
    expect(parseStarted(stored({ id: "chg_7K3M9QX2", at: now - 60_000 }), now)).toEqual({ id: "chg_7K3M9QX2", at: now - 60_000 });
  });
  it("forgets one older than a gateway's page lasts", () => {
    expect(parseStarted(stored({ id: "chg_7K3M9QX2", at: now - CHARGE_REMEMBER_MS - 1 }), now)).toBeNull();
    expect(parseStarted(stored({ id: "chg_7K3M9QX2", at: now + CHARGE_REMEMBER_MS + 1 }), now)).toBeNull();
    expect(parseStarted(stored({ id: "chg_7K3M9QX2", at: now - CHARGE_REMEMBER_MS }), now)).not.toBeNull();
  });
  it.each([
    ["nothing", null],
    ["not JSON", "chg_7K3M9QX2"],
    ["a list", "[]"],
    ["an id that is not a charge's", stored({ id: "ord_7K3M9QX2", at: now })],
    ["an id that reaches for another path", stored({ id: "chg_../../menu", at: now })],
    ["no time", stored({ id: "chg_7K3M9QX2" })],
    ["a time that is not a number", stored({ id: "chg_7K3M9QX2", at: "now" })],
  ])("is nothing for %s", (_name, text) => {
    expect(parseStarted(text, now)).toBeNull();
  });
});

describe("checkoutUrlOf", () => {
  it("follows an https page", () => {
    expect(checkoutUrlOf("https://rzp.io/i/nxrHnLJ")).toBe("https://rzp.io/i/nxrHnLJ");
  });
  it.each([undefined, "", "http://rzp.io/i/x", "javascript:alert(1)", "upi://pay?pa=x@y", "//rzp.io/i/x", "https://user:pw@rzp.io/i/x", "not an address"])(
    "follows nothing else: %s",
    (url) => {
      expect(checkoutUrlOf(url)).toBeNull();
    },
  );
});

describe("chargeWords", () => {
  it("has words for every status the server can answer", () => {
    const all: ChargeStatus[] = ["open", "paid", "failed", "unapplied"];
    for (const status of all) {
      const w = chargeWords(status);
      expect(w.title).not.toBe("");
      expect(w.line).not.toBe("");
    }
  });
  it("never says paid of a charge that is only open", () => {
    expect(chargeWords("open").title).toBe("Waiting for your payment");
    expect(chargeWords("open").tone).toBe("waiting");
  });
  it("tells a guest nothing was taken when it failed, and to find staff when it was", () => {
    expect(chargeWords("failed").line).toContain("Nothing was taken");
    expect(chargeWords("unapplied").title).toContain("was taken");
    expect(chargeWords("unapplied").line).toContain("member of staff");
  });
  it("is open only while open", () => {
    expect(chargeOpen("open")).toBe(true);
    expect(chargeOpen("paid")).toBe(false);
    expect(chargeOpen(undefined)).toBe(false);
  });
});

describe("startFailure", () => {
  it("uses the server's words for a gateway that is not working, and names staff", () => {
    const said = "Paying online can't be reached right now. Try again, or pay a member of staff.";
    expect(startFailure(new ApiError("gateway_unavailable", said, 503))).toBe(said);
  });
  it("says so when nothing is owed", () => {
    expect(startFailure(new ApiError("nothing_to_pay", "x", 409))).toBe("There is nothing left to pay on this bill.");
  });
  it("names staff or the counter for anything else, and shows nothing of the server's insides", () => {
    expect(startFailure(new ApiError("network", "offline"))).toContain("pay a member of staff");
    for (const err of [new ApiError("internal", "pq: seq 4", 500), new Error("boom"), "boom"]) {
      const text = startFailure(err);
      expect(text).toContain("Pay a member of staff");
      expect(text).not.toMatch(/seq|boom/);
    }
  });
});

import { describe, expect, it } from "vitest";
import { ApiError } from "../../shared/api";
import type { ChargeStatus, GuestRoundStatus } from "../../shared/api-types";
import {
  CHARGE_REMEMBER_MS,
  canPayRound,
  paymentStuck,
  stuckWords,
  chargeOpen,
  chargeStorageKey,
  chargeWords,
  checkoutUrlOf,
  parseStarted,
  parseStartedFor,
  roundChargeStorageKey,
  roundChargeWords,
  roundStartFailure,
  startFailure,
} from "./charge-logic";

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
    ["an order's id that is not one", stored({ id: "chg_7K3M9QX2", at: now, round: "rnd_../bill" })],
    ["an order's id that is not words", stored({ id: "chg_7K3M9QX2", at: now, round: 7 })],
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

describe("an order's charge", () => {
  const now = 1_800_000_000_000;
  const text = JSON.stringify({ id: "chg_7K3M9QX2", at: now - 1000, round: "rnd_YFMKJXT5" });

  it("is kept apart from the bill's", () => {
    expect(roundChargeStorageKey("abc")).not.toBe(chargeStorageKey("abc"));
  });
  it("is read back for its own order and no other", () => {
    expect(parseStartedFor(text, now, "rnd_YFMKJXT5")).toEqual({ id: "chg_7K3M9QX2", at: now - 1000, round: "rnd_YFMKJXT5" });
    expect(parseStartedFor(text, now, "rnd_ANOTHER1")).toBeNull();
    expect(parseStartedFor(JSON.stringify({ id: "chg_7K3M9QX2", at: now }), now, "rnd_YFMKJXT5")).toBeNull();
    expect(parseStartedFor(null, now, "rnd_YFMKJXT5")).toBeNull();
  });

  it("is offered only while the order is unpaid and no payment may yet land", () => {
    const rounds: GuestRoundStatus[] = ["waiting", "sent", "declined"];
    for (const r of rounds) expect(canPayRound(r, undefined)).toBe(false);
    expect(canPayRound("unpaid", undefined)).toBe(true);
    expect(canPayRound("unpaid", "failed")).toBe(true);
    // Open: it may be paid any moment. Paid: the order is about to say so.
    // Unapplied: the money was taken, and a second payment is a second refund.
    expect(canPayRound("unpaid", "open")).toBe(false);
    expect(canPayRound("unpaid", "paid")).toBe(false);
    expect(canPayRound("unpaid", "unapplied")).toBe(false);
  });

  it("is stuck only when the money was taken and the order is still held", () => {
    expect(paymentStuck("unpaid", "unapplied")).toBe(true);
    expect(paymentStuck("unpaid", "open")).toBe(false);
    expect(paymentStuck("unpaid", undefined)).toBe(false);
    // Sent, the food went: the order says so, and staff have the rest.
    expect(paymentStuck("sent", "unapplied")).toBe(false);
    expect(stuckWords.line).toContain("not been sent");
    expect(stuckWords.line).toContain("member of staff");
  });

  it("has words for every status, and never says paid of one that is open", () => {
    const all: ChargeStatus[] = ["open", "paid", "failed", "unapplied"];
    for (const status of all) {
      expect(roundChargeWords(status).title).not.toBe("");
      expect(roundChargeWords(status).line).not.toBe("");
    }
    expect(roundChargeWords("open").tone).toBe("waiting");
    expect(roundChargeWords("paid").line).toContain("kitchen");
    expect(roundChargeWords("failed").line).toContain("nothing was sent to the kitchen");
    expect(roundChargeWords("unapplied").line).toContain("Do not pay again");
  });

  it("says why paying could not be started, in the server's words where it wrote them for a guest", () => {
    expect(roundStartFailure(new ApiError("nothing_to_pay", "x", 409))).toBe("This order is already paid for.");
    expect(roundStartFailure(new ApiError("request_not_found", "x", 404))).toContain("Place it again");
    const lapsed = "That order is from an earlier day. Place it again.";
    expect(roundStartFailure(new ApiError("request_lapsed", lapsed, 409))).toBe(lapsed);
    const shut = "Ordering is closed right now. Ask a member of staff.";
    expect(roundStartFailure(new ApiError("no_open_shift", shut, 409))).toBe(shut);
    expect(roundStartFailure(new ApiError("network", "offline"))).toContain("Check your connection");
    for (const err of [new ApiError("internal", "pq: seq 4", 500), new Error("boom"), "boom"]) {
      const said = roundStartFailure(err);
      expect(said).toContain("member of staff");
      expect(said).not.toMatch(/seq|boom/);
    }
  });
});

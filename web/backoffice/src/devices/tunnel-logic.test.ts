import { describe, expect, it } from "vitest";
import type { TunnelState } from "../../../shared/api-types";
import { cloudLine, daysLeft, detailText, refreshMs, retryText, sinceText, tunnelLine, whyNotCarried } from "./tunnel-logic";

const now = new Date(2026, 9, 5, 14, 30, 0);
const at = (h: number, m: number, day = 5) => new Date(2026, 9, day, h, m, 0).toISOString();
const inMs = (ms: number) => new Date(now.getTime() + ms).toISOString();

const states: readonly TunnelState[] = ["off", "connecting", "connected", "waiting", "refused", "not_entitled", "replaced", "stopped"];

describe("tunnelLine", () => {
  it.each(states)("has a sentence for %s", (state) => {
    expect(tunnelLine({ state }, now, "Asia/Kolkata").length).toBeGreaterThan(20);
  });

  it("says what guests can do when connected, and since when", () => {
    expect(tunnelLine({ state: "connected", since: at(14, 2) }, now, "Asia/Kolkata")).toBe("Guests can order from their phones. Connected since 2:02 PM.");
    expect(tunnelLine({ state: "connected" }, now, "Asia/Kolkata")).toBe("Guests can order from their phones.");
  });

  it.each(["connecting", "waiting", "refused", "not_entitled"] as const)("says guests cannot order while %s", (state) => {
    expect(tunnelLine({ state }, now, "Asia/Kolkata")).toMatch(/cannot order from their phones/);
  });

  it("says the counter is not affected when the relay cannot be reached", () => {
    expect(tunnelLine({ state: "waiting" }, now, "Asia/Kolkata")).toMatch(/The counter is not affected\./);
  });

  it("says what to do for the two states waiting will not mend", () => {
    expect(tunnelLine({ state: "refused" }, now, "Asia/Kolkata")).toMatch(/Check the relay key/);
    // A venue paired to mez Cloud whose key is refused pairs again (ADR-359).
    const paired = { owner: "Priya", hostname: "blue-tokai.mezpos.com", paired_at: "2026-10-15T04:00:00Z" };
    expect(tunnelLine({ state: "refused", paired_to: paired }, now, "Asia/Kolkata")).toMatch(/Pair this computer again with a new code/);
    expect(tunnelLine({ state: "replaced" }, now, "Asia/Kolkata")).toMatch(/stop the other, or give this one a key of its own/);
  });

  it("says why the relay is not carrying the venue, and that it comes back by itself", () => {
    const line = tunnelLine({ state: "not_entitled", cloud: { back_office: false, reason: "payment_failed" } }, now, "Asia/Kolkata");
    expect(line).toMatch(/^The relay is not carrying this venue/);
    expect(line).toMatch(/The last payment for mez Cloud did not go through\./);
    expect(line).toMatch(/asks again every minute and comes back by itself/);
    expect(line).toMatch(/The counter is not affected\.$/);
    // With nothing said about why, it says who can.
    expect(tunnelLine({ state: "not_entitled" }, now, "Asia/Kolkata")).toMatch(/Whoever runs the relay can say why\./);
  });

  it("does not call a venue with no relay broken", () => {
    const line = tunnelLine({ state: "off" }, now, "Asia/Kolkata");
    expect(line).toMatch(/^Guests order at the counter\./);
    expect(line).not.toMatch(/cannot|fail|error/i);
  });
});

describe("sinceText", () => {
  it("is a time for today and a date with it for any other day", () => {
    expect(sinceText(at(9, 5), now, "Asia/Kolkata")).toBe("9:05 AM");
    expect(sinceText(at(23, 40, 3), now, "Asia/Kolkata")).toBe("3 Oct 2026, 11:40 PM");
  });
});

describe("retryText", () => {
  it.each([
    [500, "Trying again in 1 s"],
    [20_000, "Trying again in 20 s"],
    [59_001, "Trying again in 60 s"],
    [60_000, "Trying again in 1 min"],
    [4 * 60_000 + 1, "Trying again in 5 min"],
  ])("in %d ms reads %s", (ms, want) => {
    expect(retryText({ state: "waiting", retry_at: inMs(ms) }, now)).toBe(want);
  });

  it("says now once the time has come", () => {
    expect(retryText({ state: "waiting", retry_at: inMs(0) }, now)).toBe("Trying again now");
    expect(retryText({ state: "refused", retry_at: inMs(-5_000) }, now)).toBe("Trying again now");
  });

  it("says nothing when no attempt is due", () => {
    expect(retryText({ state: "waiting" }, now)).toBeNull();
    expect(retryText({ state: "connected", retry_at: inMs(20_000) }, now)).toBeNull();
    expect(retryText({ state: "off", retry_at: inMs(20_000) }, now)).toBeNull();
  });
});

describe("detailText", () => {
  it("passes on why the last attempt failed", () => {
    expect(detailText({ state: "waiting", detail: " dialling the relay: no such host " })).toBe("dialling the relay: no such host");
  });

  it("says nothing when there is nothing, or nothing is wrong", () => {
    expect(detailText({ state: "waiting" })).toBeNull();
    expect(detailText({ state: "waiting", detail: "  " })).toBeNull();
    expect(detailText({ state: "connected", detail: "the failure before" })).toBeNull();
    expect(detailText({ state: "off", detail: "x" })).toBeNull();
  });
});

describe("refreshMs", () => {
  it("reads often while the state is changing and seldom once it has settled", () => {
    expect(refreshMs("waiting")).toBe(5_000);
    expect(refreshMs("connecting")).toBe(5_000);
    expect(refreshMs("connected")).toBe(30_000);
    expect(refreshMs("refused")).toBe(30_000);
    expect(refreshMs(undefined)).toBe(30_000);
    expect(refreshMs("off")).toBeGreaterThan(60_000);
  });
});

describe("whyNotCarried", () => {
  it.each([
    ["payment_failed", /payment for mez Cloud did not go through/],
    ["payment_stopped", /payment for mez Cloud did not go through/],
    ["trial_ended", /free trial has ended/],
    ["cancelled", /subscription was cancelled/],
    ["closed", /account this venue was under is closed/],
    ["expired", /date it was carried until has passed/],
    ["never", /has not turned it on for this venue/],
    ["not_listed", /has not turned it on for this venue/],
  ])("%s", (reason, want) => {
    expect(whyNotCarried(reason)).toMatch(want);
  });

  it("says who to ask for a code this build does not know, or none", () => {
    expect(whyNotCarried("something_new")).toBe("Whoever runs the relay can say why.");
    expect(whyNotCarried(undefined)).toBe("Whoever runs the relay can say why.");
  });
});

describe("daysLeft", () => {
  it.each([
    [5 * 24 * 3600_000, "5 days left"],
    [4 * 24 * 3600_000 + 1, "5 days left"],
    [24 * 3600_000, "1 day left"],
    [24 * 3600_000 - 1, "less than a day left"],
    [60_000, "less than a day left"],
  ])("in %d ms reads %s", (ms, want) => {
    expect(daysLeft(inMs(ms), now)).toBe(want);
  });

  it("is nothing with no date, a date that has passed, or one that is not a date", () => {
    expect(daysLeft(undefined, now)).toBeNull();
    expect(daysLeft(inMs(0), now)).toBeNull();
    expect(daysLeft(inMs(-1000), now)).toBeNull();
    expect(daysLeft("not a date", now)).toBeNull();
  });
});

describe("cloudLine", () => {
  const paid = new Date(2026, 10, 12, 12, 0, 0).toISOString();
  const until = new Date(2026, 10, 19, 12, 0, 0).toISOString();

  it("is nothing when the relay said nothing, or nothing worth a line", () => {
    expect(cloudLine(undefined, now, "Asia/Kolkata")).toBeNull();
    expect(cloudLine({ back_office: true }, now, "Asia/Kolkata")).toBeNull();
    expect(cloudLine({ back_office: false, reason: "never" }, now, "Asia/Kolkata")).toBeNull();
  });

  it("says what is paid for, and to when", () => {
    expect(cloudLine({ state: "active", paid_until: paid, until, back_office: true }, now, "Asia/Kolkata")).toBe("Cloud: paid until 12 Nov 2026.");
    expect(cloudLine({ state: "active", back_office: true }, now, "Asia/Kolkata")).toBe("Cloud: paid.");
    expect(cloudLine({ state: "trial", paid_until: paid, until, back_office: true }, now, "Asia/Kolkata")).toBe("Cloud: free trial until 12 Nov 2026.");
  });

  it("counts the days left once a payment has failed, from when the relay stops and not from what was paid", () => {
    const grace = { state: "grace", paid_until: inMs(-2 * 24 * 3600_000), until: inMs(5 * 24 * 3600_000), back_office: true };
    expect(cloudLine({ ...grace, reason: "payment_failed" }, now, "Asia/Kolkata")).toBe("Cloud: payment failed, 5 days left.");
    expect(cloudLine({ ...grace, reason: "trial_ended" }, now, "Asia/Kolkata")).toBe("Cloud: the free trial has ended, 5 days left.");
    expect(cloudLine({ state: "grace", back_office: true, reason: "payment_failed" }, now, "Asia/Kolkata")).toBe("Cloud: payment failed.");
  });

  it("says a cancelled venue is carried to its date, and a stopped one what it was paid to", () => {
    expect(cloudLine({ state: "cancelled", paid_until: paid, until: paid, back_office: true }, now, "Asia/Kolkata")).toBe(
      "Cloud: cancelled. Carried until 12 Nov 2026.",
    );
    expect(cloudLine({ state: "suspended", paid_until: paid, back_office: false, reason: "payment_failed" }, now, "Asia/Kolkata")).toBe(
      "Cloud: stopped. It was paid until 12 Nov 2026.",
    );
    expect(cloudLine({ state: "closed", back_office: false }, now, "Asia/Kolkata")).toBe("Cloud: stopped.");
  });

  it("says only the date for a relay that keeps a file", () => {
    expect(cloudLine({ until, back_office: true }, now, "Asia/Kolkata")).toBe("The relay carries this venue until 19 Nov 2026.");
  });
});

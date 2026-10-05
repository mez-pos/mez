import { describe, expect, it } from "vitest";
import type { TunnelState } from "../../../shared/api-types";
import { detailText, refreshMs, retryText, sinceText, tunnelLine } from "./tunnel-logic";

const now = new Date(2026, 9, 5, 14, 30, 0);
const at = (h: number, m: number, day = 5) => new Date(2026, 9, day, h, m, 0).toISOString();
const inMs = (ms: number) => new Date(now.getTime() + ms).toISOString();

const states: readonly TunnelState[] = ["off", "connecting", "connected", "waiting", "refused", "replaced", "stopped"];

describe("tunnelLine", () => {
  it.each(states)("has a sentence for %s", (state) => {
    expect(tunnelLine({ state }, now).length).toBeGreaterThan(20);
  });

  it("says what guests can do when connected, and since when", () => {
    expect(tunnelLine({ state: "connected", since: at(14, 2) }, now)).toBe("Guests can order from their phones. Connected since 14:02.");
    expect(tunnelLine({ state: "connected" }, now)).toBe("Guests can order from their phones.");
  });

  it.each(["connecting", "waiting", "refused"] as const)("says guests cannot order while %s", (state) => {
    expect(tunnelLine({ state }, now)).toMatch(/cannot order from their phones/);
  });

  it("says the counter is not affected when the relay cannot be reached", () => {
    expect(tunnelLine({ state: "waiting" }, now)).toMatch(/The counter is not affected\./);
  });

  it("says what to do for the two states waiting will not mend", () => {
    expect(tunnelLine({ state: "refused" }, now)).toMatch(/Check the relay key/);
    expect(tunnelLine({ state: "replaced" }, now)).toMatch(/stop the other, or give this one a key of its own/);
  });

  it("does not call a venue with no relay broken", () => {
    const line = tunnelLine({ state: "off" }, now);
    expect(line).toMatch(/^Guests order at the counter\./);
    expect(line).not.toMatch(/cannot|fail|error/i);
  });
});

describe("sinceText", () => {
  it("is a time for today and a date with it for any other day", () => {
    expect(sinceText(at(9, 5), now)).toBe("09:05");
    expect(sinceText(at(23, 40, 3), now)).toBe("3 Oct 2026, 23:40");
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

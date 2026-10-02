import { describe, expect, it } from "vitest";
import { ApiError } from "../api";
import { WrongDeviceKind } from "./auth-store";
import { pairingError, signInError } from "./messages";

const host = "192.168.1.20:7777";

describe("pairingError", () => {
  const tests = [
    { name: "a spent code", err: new ApiError("invalid_pairing_code", "x", 401), want: /didn't work.*10 minutes.*once/ },
    { name: "rate limited, with the wait", err: new ApiError("rate_limited", "x", 429, null, 42_000), want: /Wait 0:42/ },
    { name: "rate limited, wait unknown", err: new ApiError("rate_limited", "x", 429), want: /Wait a minute/ },
    { name: "no server", err: new ApiError("network", "x"), want: /Can't reach the mez server at 192\.168\.1\.20:7777/ },
    { name: "a code for another app", err: new WrongDeviceKind("terminal", "back_office"), want: /pairs a terminal, and this is the back office/ },
    { name: "anything else", err: new Error("boom"), want: /Something went wrong/ },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(pairingError(tt.err, host)).toMatch(tt.want));
  }
});

describe("signInError", () => {
  const now = 1_000_000;
  it("a wrong PIN does not lock the pad", () => {
    expect(signInError(new ApiError("invalid_pin", "x", 401), host, now)).toEqual({ message: "That PIN isn't right. Try again.", lockedUntil: null });
  });
  it("a lockout locks the pad for the server's wait", () => {
    expect(signInError(new ApiError("locked_out", "x", 429, null, 240_000), host, now).lockedUntil).toBe(now + 240_000);
  });
  it("a lockout without Retry-After waits five minutes", () => {
    expect(signInError(new ApiError("locked_out", "x", 429), host, now).lockedUntil).toBe(now + 300_000);
  });
  it("no server says the PIN was not checked", () => {
    expect(signInError(new ApiError("timeout", "x"), host, now).message).toMatch(/PIN wasn't checked/);
  });
});

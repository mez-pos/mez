import { describe, expect, it } from "vitest";
import type { AuthStatus } from "./auth-store";
import { routeFor } from "./route";

const device = { id: "dev_1", name: "Counter 1", kind: "terminal" as const };
const states: Record<string, AuthStatus> = {
  checking: { kind: "checking" },
  unpaired: { kind: "unpaired", notice: null },
  locked: { kind: "locked", device, notice: null },
  signed_in: { kind: "signed_in", device, staff: { id: "s", name: "Asha" }, expiresAt: "x" },
};

describe("routeFor", () => {
  const tests = [
    { state: "checking", path: "/", want: null },
    { state: "checking", path: "/pair", want: null },
    { state: "unpaired", path: "/", want: "/pair" },
    { state: "unpaired", path: "/lock", want: "/pair" },
    { state: "unpaired", path: "/pair", want: null },
    { state: "locked", path: "/", want: "/lock" },
    { state: "locked", path: "/pair", want: "/lock" },
    { state: "locked", path: "/lock", want: null },
    { state: "signed_in", path: "/pair", want: "/" },
    { state: "signed_in", path: "/lock", want: "/" },
    { state: "signed_in", path: "/", want: null },
    { state: "signed_in", path: "/tickets/42", want: null },
  ];
  for (const tt of tests) {
    it(`${tt.state} at ${tt.path}`, () => expect(routeFor(states[tt.state] as AuthStatus, tt.path)).toBe(tt.want));
  }
});

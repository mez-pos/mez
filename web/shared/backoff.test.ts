import { describe, expect, it } from "vitest";
import { backoffDelay } from "./backoff";

const policy = { baseMs: 500, capMs: 10_000 };

describe("backoffDelay", () => {
  const tests = [
    { name: "first retry, lowest draw", attempt: 1, random: 0, want: 250 },
    { name: "first retry, highest draw", attempt: 1, random: 0.999, want: 499 },
    { name: "doubles each attempt", attempt: 3, random: 0, want: 1000 },
    { name: "capped", attempt: 9, random: 0, want: 5000 },
    { name: "capped, highest draw", attempt: 50, random: 0.999, want: 9995 },
    { name: "after a week of retries, still finite", attempt: 100_000, random: 0.5, want: 7500 },
    { name: "attempt below one is treated as one", attempt: 0, random: 0, want: 250 },
  ];
  for (const tt of tests) {
    it(tt.name, () => {
      expect(backoffDelay(tt.attempt, policy, () => tt.random)).toBe(tt.want);
    });
  }

  it("never reaches zero, so a countdown never flickers", () => {
    for (let attempt = 1; attempt < 40; attempt++) {
      expect(backoffDelay(attempt, policy, () => 0)).toBeGreaterThan(0);
    }
  });
});

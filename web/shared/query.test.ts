import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { shouldRetry } from "./query";

describe("shouldRetry", () => {
  const tests = [
    { name: "network failure is retried", count: 0, error: new ApiError("network", "x"), want: true },
    { name: "timeout is retried", count: 1, error: new ApiError("timeout", "x"), want: true },
    { name: "but only twice", count: 2, error: new ApiError("network", "x"), want: false },
    { name: "a refusal is an answer", count: 0, error: new ApiError("forbidden", "x", 403), want: false },
    { name: "a server error is an answer", count: 0, error: new ApiError("internal", "x", 500), want: false },
    { name: "an unknown error is not retried", count: 0, error: new Error("x"), want: false },
  ];
  for (const tt of tests) {
    it(tt.name, () => expect(shouldRetry(tt.count, tt.error)).toBe(tt.want));
  }
});

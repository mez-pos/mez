import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { replaceQuery, shouldRetry } from "./query";

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

describe("replaceQuery", () => {
  it("drops a read in flight so it cannot put the old data back over the answer", async () => {
    const queryClient = new QueryClient();
    const key = ["kitchen", "grill"];
    let answer: (board: string) => void = () => undefined;
    const slow = queryClient.fetchQuery({ queryKey: key, queryFn: () => new Promise<string>((resolve) => (answer = resolve)), retry: false });
    await new Promise((resolve) => setTimeout(resolve, 0));

    await replaceQuery(queryClient, key, "after the bump");
    answer("before the bump");
    await slow.catch(() => undefined);
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(queryClient.getQueryData(key)).toBe("after the bump");
  });

  it("sets the data when nothing is in flight", async () => {
    const queryClient = new QueryClient();
    await replaceQuery(queryClient, ["floor", "live"], { tables: [] });
    expect(queryClient.getQueryData(["floor", "live"])).toEqual({ tables: [] });
  });
});
